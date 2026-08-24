package runner

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/dbmove/dbmove/backend/internal/model"
	"github.com/dbmove/dbmove/backend/internal/sse"
)

// DockerRunner executes workers as ephemeral Docker containers (used for
// local development where Kubernetes may not be available).
type DockerRunner struct {
	WorkerImage   string
	APIURL        string
	DataDir       string
	Network       string
	InternalToken string
	Watcher       *Watcher
}

func containerName(taskID uint64) string {
	return fmt.Sprintf("dbmove-migration-%d", taskID)
}

// Start launches a worker container for the task.
func (r *DockerRunner) Start(ctx context.Context, task *model.MigrationTask, secrets Secrets) error {
	name := containerName(task.ID)
	envFile := filepath.Join(r.DataDir, fmt.Sprintf("env-%d", task.ID))
	envContent := fmt.Sprintf("TASK_ID=%d\nDBMOVE_API=%s\nDBMOVE_SOURCE_PASSWORD=%s\nDBMOVE_TARGET_PASSWORD=%s\nDBMOVE_INTERNAL_TOKEN=%s\n",
		task.ID, r.APIURL, secrets.SourcePassword, secrets.TargetPassword, r.InternalToken)
	if err := os.WriteFile(envFile, []byte(envContent), 0o600); err != nil {
		return fmt.Errorf("write env file: %w", err)
	}
	defer os.Remove(envFile)

	args := []string{
		"run", "--rm", "-d",
		"--name", name,
		"--label", "app=dbmove",
		"--label", fmt.Sprintf("task-id=%d", task.ID),
		"--env-file", envFile,
	}
	if r.Network != "" {
		args = append(args, "--network", r.Network)
	}
	args = append(args, "--volume", r.DataDir+":/data", r.WorkerImage)
	cmd := exec.CommandContext(ctx, "docker", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("start worker container: %v: %s", err, strings.TrimSpace(string(out)))
	}
	go r.watch(task.ID)
	return nil
}

// Cancel removes the worker container.
func (r *DockerRunner) Cancel(ctx context.Context, taskID uint64) error {
	cmd := exec.CommandContext(ctx, "docker", "rm", "-f", containerName(taskID))
	out, _ := cmd.CombinedOutput()
	_ = out
	return nil
}

// Cleanup removes leftover resources for a task.
func (r *DockerRunner) Cleanup(ctx context.Context, taskID uint64) error {
	_ = r.Cancel(ctx, taskID)
	os.Remove(filepath.Join(r.DataDir, fmt.Sprintf("env-%d", taskID)))
	return nil
}

func (r *DockerRunner) watch(taskID uint64) {
	ctx, cancel := context.WithTimeout(context.Background(), 24*time.Hour)
	defer cancel()
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			status, exitCode, err := r.inspect(taskID)
			if err != nil {
				task, gerr := r.Watcher.Repo.GetTask(ctx, taskID)
				if gerr != nil {
					return
				}
				if IsTerminal(task.Status) {
					return
				}
				_ = r.Watcher.MarkFailedIfStillActive(ctx, taskID, "worker container was removed before reporting a result")
				return
			}
			if status == "exited" {
				reason := fmt.Sprintf("worker container exited with code %d", exitCode)
				if exitCode == 0 {
					// worker should have reported SUCCESS; if not, treat as success
					task, gerr := r.Watcher.Repo.GetTask(ctx, taskID)
					if gerr == nil && !IsTerminal(task.Status) {
						_ = r.markSuccess(taskID)
					}
					return
				}
				_ = r.Watcher.MarkFailedIfStillActive(ctx, taskID, reason)
				return
			}
		}
	}
}

func (r *DockerRunner) inspect(taskID uint64) (string, int, error) {
	name := containerName(taskID)
	cmd := exec.Command("docker", "inspect", "-f", "{{.State.Status}}", name)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", 0, fmt.Errorf("inspect: %w", err)
	}
	status := strings.TrimSpace(string(out))
	exitCmd := exec.Command("docker", "inspect", "-f", "{{.State.ExitCode}}", name)
	exitOut, exitErr := exitCmd.CombinedOutput()
	if exitErr != nil {
		return status, 0, nil
	}
	var code int
	_, _ = fmt.Sscanf(strings.TrimSpace(string(exitOut)), "%d", &code)
	return status, code, nil
}

func (r *DockerRunner) markSuccess(taskID uint64) error {
	now := time.Now()
	if err := r.Watcher.Repo.UpdateTaskFields(context.Background(), taskID, map[string]any{
		"status":      model.TaskStatusSuccess,
		"finished_at": now,
	}); err != nil {
		return err
	}
	msg := "migration completed"
	_ = r.Watcher.Repo.AddLog(context.Background(), &model.MigrationLog{TaskID: taskID, Level: "INFO", Message: msg, CreatedAt: now})
	if r.Watcher.Hub != nil {
		r.Watcher.Hub.Publish(taskID, sse.Event{Type: "status", Data: map[string]any{"status": model.TaskStatusSuccess}})
	}
	return nil
}

// Compile-time check that DockerRunner implements Runner.
var _ Runner = (*DockerRunner)(nil)
