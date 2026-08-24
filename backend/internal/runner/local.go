package runner

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/dbmove/dbmove/backend/internal/model"
)

// LocalRunner executes the worker binary as a subprocess (developer mode).
type LocalRunner struct {
	WorkerBin     string
	APIURL        string
	InternalToken string
	Watcher       *Watcher
}

func (r *LocalRunner) Start(ctx context.Context, task *model.MigrationTask, secrets Secrets) error {
	cmd := exec.CommandContext(ctx, r.WorkerBin)
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("TASK_ID=%d", task.ID),
		"DBMOVE_API="+r.APIURL,
		"DBMOVE_INTERNAL_TOKEN="+r.InternalToken,
		"DBMOVE_SOURCE_PASSWORD="+secrets.SourcePassword,
		"DBMOVE_TARGET_PASSWORD="+secrets.TargetPassword,
	)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start worker process: %w", err)
	}
	go func() {
		err := cmd.Wait()
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			_ = r.Watcher.MarkFailedIfStillActive(context.Background(), task.ID, fmt.Sprintf("worker process exited: %v", err))
			return
		}
		task2, gerr := r.Watcher.Repo.GetTask(context.Background(), task.ID)
		if gerr == nil && !IsTerminal(task2.Status) {
			now := time.Now()
			_ = r.Watcher.Repo.UpdateTaskFields(context.Background(), task.ID, map[string]any{
				"status":      model.TaskStatusSuccess,
				"finished_at": now,
			})
		}
	}()
	return nil
}

func (r *LocalRunner) Cancel(ctx context.Context, taskID uint64) error {
	// The process is killed by the OS when its parent exits; for local mode
	// cancellation is best-effort via the API status transition only.
	return nil
}

var _ Runner = (*LocalRunner)(nil)
