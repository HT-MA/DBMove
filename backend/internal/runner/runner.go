package runner

import (
	"context"
	"fmt"
	"time"

	"github.com/dbmove/dbmove/backend/internal/model"
	"github.com/dbmove/dbmove/backend/internal/repository"
	"github.com/dbmove/dbmove/backend/internal/sse"
	"gorm.io/gorm"
)

// Secrets carries the decrypted passwords for a migration task.
type Secrets struct {
	SourcePassword string
	TargetPassword string
}

// Runner starts and cancels migration workers. Implementations are
// Kubernetes Jobs, Docker containers, or local processes.
type Runner interface {
	Start(ctx context.Context, task *model.MigrationTask, secrets Secrets) error
	Cancel(ctx context.Context, taskID uint64) error
}

// Watcher reconciles task state when a worker dies without reporting a
// terminal status.
type Watcher struct {
	Repo *repository.Repository
	Hub  *sse.Hub
}

// MarkFailedIfStillActive marks the task FAILED only when it is not in a
// terminal state, and appends a log line.
func (w *Watcher) MarkFailedIfStillActive(ctx context.Context, taskID uint64, reason string) error {
	task, err := w.Repo.GetTask(ctx, taskID)
	if err != nil {
		return err
	}
	switch task.Status {
	case model.TaskStatusPreparing, model.TaskStatusRunning, model.TaskStatusPending:
	default:
		return nil // already terminal
	}
	now := time.Now()
	if err := w.Repo.UpdateTaskFields(ctx, taskID, map[string]any{
		"status":        model.TaskStatusFailed,
		"error_message": reason,
		"finished_at":   now,
	}); err != nil {
		return err
	}
	msg := fmt.Sprintf("migration failed: %s", reason)
	_ = w.Repo.AddLog(ctx, &model.MigrationLog{TaskID: taskID, Level: "ERROR", Message: msg, CreatedAt: now})
	if w.Hub != nil {
		w.Hub.Publish(taskID, sse.Event{Type: "log", Data: map[string]any{"level": "ERROR", "message": msg, "created_at": now}})
		w.Hub.Publish(taskID, sse.Event{Type: "status", Data: map[string]any{"status": model.TaskStatusFailed, "error_message": reason}})
	}
	return nil
}

// MarkCancelledIfStillActive marks the task CANCELLED when not terminal.
func (w *Watcher) MarkCancelledIfStillActive(ctx context.Context, taskID uint64) error {
	task, err := w.Repo.GetTask(ctx, taskID)
	if err != nil {
		return err
	}
	if task.Status != model.TaskStatusPreparing && task.Status != model.TaskStatusRunning && task.Status != model.TaskStatusPending {
		return nil
	}
	now := time.Now()
	if err := w.Repo.UpdateTaskFields(ctx, taskID, map[string]any{
		"status":      model.TaskStatusCancelled,
		"finished_at": now,
	}); err != nil {
		return err
	}
	msg := "migration cancelled"
	_ = w.Repo.AddLog(ctx, &model.MigrationLog{TaskID: taskID, Level: "WARN", Message: msg, CreatedAt: now})
	if w.Hub != nil {
		w.Hub.Publish(taskID, sse.Event{Type: "log", Data: map[string]any{"level": "WARN", "message": msg, "created_at": now}})
		w.Hub.Publish(taskID, sse.Event{Type: "status", Data: map[string]any{"status": model.TaskStatusCancelled}})
	}
	return nil
}

// IsTerminal returns true when a status is a final state.
func IsTerminal(status string) bool {
	switch status {
	case model.TaskStatusSuccess, model.TaskStatusFailed, model.TaskStatusCancelled:
		return true
	}
	return false
}

// Cleanup is an optional interface for runners that need to remove resources
// after a terminal state.
type Cleanup interface {
	Cleanup(ctx context.Context, taskID uint64) error
}

// DB helpers shared by runners.
func DB(r *repository.Repository) *gorm.DB { return r.DB() }
