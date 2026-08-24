package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dbmove/dbmove/backend/internal/model"
	"github.com/dbmove/dbmove/backend/internal/repository"
	"gorm.io/gorm"
)

// MigrationInput is the payload for creating a migration task.
type MigrationInput struct {
	Name             string               `json:"name"`
	SourceConnection uint64               `json:"source_connection_id"`
	TargetConnection uint64               `json:"target_connection_id"`
	SourceDatabase   string               `json:"source_database"`
	TargetDatabase   string               `json:"target_database"`
	Databases        []model.DatabasePair `json:"databases"`
	MigrationType    string               `json:"migration_type"`
	TargetDBPolicy   string               `json:"target_db_policy"`
	CreatedBy        string               `json:"created_by"`
}

// MigrationService manages migration tasks.
type MigrationService struct {
	repo *repository.Repository
}

func NewMigrationService(repo *repository.Repository) *MigrationService {
	return &MigrationService{repo: repo}
}

var (
	ErrTaskNotFound       = errors.New("migration task not found")
	ErrInvalidState       = errors.New("invalid task state for this operation")
	ErrAlreadyRunning     = errors.New("migration task is already running")
	ErrUnsupportedMigrate = errors.New("unsupported migration (MVP supports mysql->mysql and postgresql->postgresql)")
)

// Create validates and persists a new migration task in PENDING state.
func (s *MigrationService) Create(ctx context.Context, in MigrationInput) (*model.MigrationTask, error) {
	if in.MigrationType == "" {
		in.MigrationType = model.MigrationTypeFull
	}
	if in.TargetDBPolicy == "" {
		in.TargetDBPolicy = model.TargetDBPolicyError
	}
	if err := s.validateCreate(ctx, in); err != nil {
		return nil, err
	}
	source, err := s.repo.GetConnection(ctx, in.SourceConnection)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrConnectionNotFound
		}
		return nil, err
	}
	pairs, err := resolveDatabasePairs(in)
	if err != nil {
		return nil, err
	}
	task := &model.MigrationTask{
		Name:               in.Name,
		SourceConnectionID: in.SourceConnection,
		TargetConnectionID: in.TargetConnection,
		SourceDatabase:     pairs[0].Source,
		TargetDatabase:     pairs[0].Target,
		Databases:          model.DatabaseMapping(pairs),
		MigrationType:      in.MigrationType,
		TargetDBPolicy:     in.TargetDBPolicy,
		Engine:             engineForType(source.Type),
		Status:             model.TaskStatusPending,
		CreatedBy:          in.CreatedBy,
	}
	if err := s.repo.CreateTask(ctx, task); err != nil {
		return nil, fmt.Errorf("create migration task: %w", err)
	}
	return task, nil
}

func (s *MigrationService) validateCreate(ctx context.Context, in MigrationInput) error {
	if in.Name == "" {
		return errors.New("name is required")
	}
	if in.SourceConnection == 0 || in.TargetConnection == 0 {
		return errors.New("source and target connections are required")
	}
	if in.SourceConnection == in.TargetConnection {
		return errors.New("source and target connections must be different")
	}
	if in.MigrationType != model.MigrationTypeFull {
		return fmt.Errorf("unsupported migration type %q (MVP supports FULL)", in.MigrationType)
	}
	switch in.TargetDBPolicy {
	case model.TargetDBPolicyError:
	case model.TargetDBPolicyCreate, model.TargetDBPolicyOverwrite:
	default:
		return fmt.Errorf("invalid target_db_policy %q", in.TargetDBPolicy)
	}
	source, err := s.repo.GetConnection(ctx, in.SourceConnection)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrConnectionNotFound
		}
		return err
	}
	target, err := s.repo.GetConnection(ctx, in.TargetConnection)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrConnectionNotFound
		}
		return err
	}
	if source.Type != target.Type {
		return ErrUnsupportedMigrate
	}
	switch source.Type {
	case model.ConnTypeMySQL, model.ConnTypePostgreSQL:
	default:
		return ErrUnsupportedMigrate
	}
	if _, err := resolveDatabasePairs(in); err != nil {
		return err
	}
	return nil
}

// resolveDatabasePairs normalizes the databases mapping. The new multi-db
// field takes precedence; the legacy single-db pair is used as a fallback.
func resolveDatabasePairs(in MigrationInput) ([]model.DatabasePair, error) {
	if len(in.Databases) > 0 {
		seenSource := map[string]bool{}
		seenTarget := map[string]bool{}
		for i := range in.Databases {
			p := in.Databases[i]
			if p.Source == "" {
				return nil, fmt.Errorf("databases[%d].source is required", i)
			}
			if p.Target == "" {
				return nil, fmt.Errorf("databases[%d].target is required", i)
			}
			if seenSource[p.Source] {
				return nil, fmt.Errorf("duplicate source database %q", p.Source)
			}
			if seenTarget[p.Target] {
				return nil, fmt.Errorf("duplicate target database %q", p.Target)
			}
			seenSource[p.Source] = true
			seenTarget[p.Target] = true
		}
		return in.Databases, nil
	}
	if in.SourceDatabase == "" {
		return nil, errors.New("source_database is required (or provide a databases mapping)")
	}
	if in.TargetDatabase == "" {
		return nil, errors.New("target_database is required")
	}
	return []model.DatabasePair{{Source: in.SourceDatabase, Target: in.TargetDatabase}}, nil
}

func engineForType(connType string) string {
	switch connType {
	case model.ConnTypeMySQL:
		return "mysql-dump"
	case model.ConnTypePostgreSQL:
		return "postgresql-dump"
	}
	return "unknown"
}

// Start queues a task for execution.
func (s *MigrationService) Start(ctx context.Context, id uint64) (*model.MigrationTask, error) {
	task, err := s.repo.GetTask(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTaskNotFound
		}
		return nil, err
	}
	switch task.Status {
	case model.TaskStatusPending, model.TaskStatusCancelled:
		now := time.Now()
		if err := s.repo.UpdateTaskFields(ctx, id, map[string]any{
			"status":        model.TaskStatusPending,
			"queued_at":     now,
			"error_message": "",
			"finished_at":   nil,
			"started_at":    nil,
			"progress":      0,
		}); err != nil {
			return nil, err
		}
	case model.TaskStatusFailed:
		return nil, errors.New("failed tasks must be retried via the retry endpoint")
	default:
		return nil, ErrAlreadyRunning
	}
	return s.repo.GetTask(ctx, id)
}

// Cancel cancels a queued or running task.
func (s *MigrationService) Cancel(ctx context.Context, id uint64, cancelRunner func(context.Context, uint64) error) (*model.MigrationTask, error) {
	task, err := s.repo.GetTask(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTaskNotFound
		}
		return nil, err
	}
	switch task.Status {
	case model.TaskStatusPending, model.TaskStatusPreparing, model.TaskStatusRunning:
	default:
		return nil, ErrInvalidState
	}
	now := time.Now()
	if err := s.repo.UpdateTaskFields(ctx, id, map[string]any{
		"status":      model.TaskStatusCancelled,
		"finished_at": now,
	}); err != nil {
		return nil, err
	}
	if cancelRunner != nil {
		_ = cancelRunner(ctx, id)
	}
	_ = s.repo.AddLog(ctx, &model.MigrationLog{TaskID: id, Level: "WARN", Message: "migration cancelled by user", CreatedAt: now})
	return s.repo.GetTask(ctx, id)
}

// Retry resets a failed task and queues it again.
func (s *MigrationService) Retry(ctx context.Context, id uint64) (*model.MigrationTask, error) {
	task, err := s.repo.GetTask(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTaskNotFound
		}
		return nil, err
	}
	if task.Status != model.TaskStatusFailed {
		return nil, ErrInvalidState
	}
	now := time.Now()
	if err := s.repo.UpdateTaskFields(ctx, id, map[string]any{
		"status":            model.TaskStatusPending,
		"queued_at":         now,
		"error_message":     "",
		"progress":          0,
		"finished_at":       nil,
		"started_at":        nil,
		"tables_total":      0,
		"tables_completed":  0,
		"rows_total":        0,
		"rows_completed":    0,
		"bytes_total":       0,
		"bytes_transferred": 0,
		"speed":             0,
	}); err != nil {
		return nil, err
	}
	_ = s.repo.AddLog(ctx, &model.MigrationLog{TaskID: id, Level: "INFO", Message: "migration queued for retry", CreatedAt: now})
	return s.repo.GetTask(ctx, id)
}

// Get returns one task with connections.
func (s *MigrationService) Get(ctx context.Context, id uint64) (*model.MigrationTask, error) {
	task, err := s.repo.GetTask(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTaskNotFound
		}
		return nil, err
	}
	return task, nil
}

// List returns a page of tasks.
func (s *MigrationService) List(ctx context.Context, page, pageSize int, status, createdBy string) ([]model.MigrationTask, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	return s.repo.ListTasks(ctx, page, pageSize, status, createdBy)
}

// Logs returns the persisted logs for a task.
func (s *MigrationService) Logs(ctx context.Context, id uint64, limit int) ([]model.MigrationLog, error) {
	if limit <= 0 || limit > 5000 {
		limit = 1000
	}
	return s.repo.ListLogs(ctx, id, limit)
}

// Stats returns dashboard statistics.
func (s *MigrationService) Stats(ctx context.Context) (map[string]any, error) {
	conns, err := s.repo.CountConnectionsByType(ctx)
	if err != nil {
		return nil, err
	}
	statuses, err := s.repo.CountTasksByStatus(ctx)
	if err != nil {
		return nil, err
	}
	recent, err := s.repo.RecentTasks(ctx, 5)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"connections": map[string]int64{
			"total":      conns[model.ConnTypeMySQL] + conns[model.ConnTypePostgreSQL] + conns[model.ConnTypeDM8] + conns[model.ConnTypeRedis],
			"mysql":      conns[model.ConnTypeMySQL],
			"postgresql": conns[model.ConnTypePostgreSQL],
			"dm8":        conns[model.ConnTypeDM8],
			"redis":      conns[model.ConnTypeRedis],
		},
		"migrations": map[string]int64{
			"total":     statuses[model.TaskStatusPending] + statuses[model.TaskStatusPreparing] + statuses[model.TaskStatusRunning] + statuses[model.TaskStatusSuccess] + statuses[model.TaskStatusFailed] + statuses[model.TaskStatusCancelled],
			"pending":   statuses[model.TaskStatusPending],
			"preparing": statuses[model.TaskStatusPreparing],
			"running":   statuses[model.TaskStatusRunning],
			"success":   statuses[model.TaskStatusSuccess],
			"failed":    statuses[model.TaskStatusFailed],
			"cancelled": statuses[model.TaskStatusCancelled],
		},
		"recent_migrations": recent,
	}, nil
}
