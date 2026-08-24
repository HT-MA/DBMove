package dispatcher

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/dbmove/dbmove/backend/internal/crypto"
	"github.com/dbmove/dbmove/backend/internal/model"
	"github.com/dbmove/dbmove/backend/internal/repository"
	"github.com/dbmove/dbmove/backend/internal/runner"
	"gorm.io/gorm"
)

// Dispatcher picks queued PENDING tasks and hands them to the runner while
// respecting the max-concurrency limit.
type Dispatcher struct {
	repo   *repository.Repository
	runner runner.Runner
	cipher *crypto.Cipher
	max    int
}

func New(repo *repository.Repository, r runner.Runner, c *crypto.Cipher, max int) *Dispatcher {
	return &Dispatcher{repo: repo, runner: r, cipher: c, max: max}
}

// Run starts the dispatch loop until ctx is cancelled.
func (d *Dispatcher) Run(ctx context.Context) {
	log.Printf("dispatcher started (max concurrent migrations: %d)", d.max)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.dispatchOnce(ctx)
		}
	}
}

func (d *Dispatcher) dispatchOnce(ctx context.Context) {
	active, err := d.repo.CountActiveTasks(ctx)
	if err != nil {
		log.Printf("dispatcher: count active tasks: %v", err)
		return
	}
	for active < int64(d.max) {
		task, err := d.repo.NextPendingTask(ctx)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return // no more queued tasks
			}
			log.Printf("dispatcher: next pending task: %v", err)
			return
		}
		secrets, err := d.secretsFor(ctx, task)
		if err != nil {
			now := time.Now()
			msg := fmt.Sprintf("failed to prepare migration secrets: %v", err)
			_ = d.repo.UpdateTaskFields(ctx, task.ID, map[string]any{
				"status":        model.TaskStatusFailed,
				"error_message": msg,
				"finished_at":   now,
			})
			_ = d.repo.AddLog(ctx, &model.MigrationLog{TaskID: task.ID, Level: "ERROR", Message: msg, CreatedAt: now})
			continue
		}

		now := time.Now()
		if err := d.repo.UpdateTaskFields(ctx, task.ID, map[string]any{
			"status":     model.TaskStatusPreparing,
			"started_at": now,
		}); err != nil {
			log.Printf("dispatcher: claim task %d: %v", task.ID, err)
			return
		}
		// The task may have been cancelled between the claim and now; never
		// start a worker for a task that is no longer PREPARING.
		claimed, err := d.repo.GetTask(ctx, task.ID)
		if err != nil {
			log.Printf("dispatcher: reload task %d: %v", task.ID, err)
			return
		}
		if claimed.Status != model.TaskStatusPreparing {
			log.Printf("dispatcher: task %d no longer PREPARING (%s), skipping start", task.ID, claimed.Status)
			continue
		}
		if err := d.runner.Start(ctx, task, secrets); err != nil {
			msg := fmt.Sprintf("failed to start migration worker: %v", err)
			_ = d.repo.UpdateTaskFields(ctx, task.ID, map[string]any{
				"status":        model.TaskStatusFailed,
				"error_message": msg,
				"finished_at":   time.Now(),
			})
			_ = d.repo.AddLog(ctx, &model.MigrationLog{TaskID: task.ID, Level: "ERROR", Message: msg, CreatedAt: time.Now()})
			continue
		}
		active++
	}
}

func (d *Dispatcher) secretsFor(ctx context.Context, task *model.MigrationTask) (runner.Secrets, error) {
	src, err := d.repo.GetConnection(ctx, task.SourceConnectionID)
	if err != nil {
		return runner.Secrets{}, fmt.Errorf("source connection: %w", err)
	}
	tgt, err := d.repo.GetConnection(ctx, task.TargetConnectionID)
	if err != nil {
		return runner.Secrets{}, fmt.Errorf("target connection: %w", err)
	}
	srcPass, err := d.cipher.Decrypt(src.PasswordEncrypted)
	if err != nil {
		return runner.Secrets{}, fmt.Errorf("decrypt source password: %w", err)
	}
	tgtPass, err := d.cipher.Decrypt(tgt.PasswordEncrypted)
	if err != nil {
		return runner.Secrets{}, fmt.Errorf("decrypt target password: %w", err)
	}
	return runner.Secrets{SourcePassword: srcPass, TargetPassword: tgtPass}, nil
}
