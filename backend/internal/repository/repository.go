package repository

import (
	"context"

	"github.com/dbmove/dbmove/backend/internal/model"
	"gorm.io/gorm"
)

// Repository provides typed data access for the platform database.
type Repository struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) DB() *gorm.DB { return r.db }

// ---- connections ----

func (r *Repository) CreateConnection(ctx context.Context, c *model.Connection) error {
	return r.db.WithContext(ctx).Create(c).Error
}

func (r *Repository) UpdateConnection(ctx context.Context, c *model.Connection) error {
	return r.db.WithContext(ctx).Save(c).Error
}

func (r *Repository) DeleteConnection(ctx context.Context, id uint64) error {
	return r.db.WithContext(ctx).Delete(&model.Connection{}, id).Error
}

func (r *Repository) GetConnection(ctx context.Context, id uint64) (*model.Connection, error) {
	var c model.Connection
	if err := r.db.WithContext(ctx).First(&c, id).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *Repository) ListConnections(ctx context.Context) ([]model.Connection, error) {
	var items []model.Connection
	if err := r.db.WithContext(ctx).Order("created_at DESC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *Repository) CountConnectionsByType(ctx context.Context) (map[string]int64, error) {
	type row struct {
		Type  string
		Count int64
	}
	var rows []row
	if err := r.db.WithContext(ctx).Model(&model.Connection{}).
		Select("type, count(*) as count").Group("type").Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := map[string]int64{}
	for _, r := range rows {
		out[r.Type] = r.Count
	}
	return out, nil
}

func (r *Repository) ConnectionInUse(ctx context.Context, id uint64) (bool, error) {
	var n int64
	if err := r.db.WithContext(ctx).Model(&model.MigrationTask{}).
		Where("source_connection_id = ? OR target_connection_id = ?", id, id).Count(&n).Error; err != nil {
		return false, err
	}
	return n > 0, nil
}

// ---- migration tasks ----

func (r *Repository) CreateTask(ctx context.Context, t *model.MigrationTask) error {
	return r.db.WithContext(ctx).Create(t).Error
}

func (r *Repository) UpdateTask(ctx context.Context, t *model.MigrationTask) error {
	return r.db.WithContext(ctx).Save(t).Error
}

func (r *Repository) UpdateTaskFields(ctx context.Context, id uint64, fields map[string]any) error {
	return r.db.WithContext(ctx).Model(&model.MigrationTask{}).Where("id = ?", id).Updates(fields).Error
}

func (r *Repository) GetTask(ctx context.Context, id uint64) (*model.MigrationTask, error) {
	var t model.MigrationTask
	if err := r.db.WithContext(ctx).Preload("SourceConnection").Preload("TargetConnection").First(&t, id).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *Repository) ListTasks(ctx context.Context, page, pageSize int, status, createdBy string) ([]model.MigrationTask, int64, error) {
	q := r.db.WithContext(ctx).Model(&model.MigrationTask{})
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if createdBy != "" {
		q = q.Where("created_by = ?", createdBy)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []model.MigrationTask
	if err := q.Preload("SourceConnection").Preload("TargetConnection").
		Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *Repository) CountTasksByStatus(ctx context.Context) (map[string]int64, error) {
	type row struct {
		Status string
		Count  int64
	}
	var rows []row
	if err := r.db.WithContext(ctx).Model(&model.MigrationTask{}).
		Select("status, count(*) as count").Group("status").Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := map[string]int64{}
	for _, r := range rows {
		out[r.Status] = r.Count
	}
	return out, nil
}

func (r *Repository) RecentTasks(ctx context.Context, limit int) ([]model.MigrationTask, error) {
	var items []model.MigrationTask
	if err := r.db.WithContext(ctx).Preload("SourceConnection").Preload("TargetConnection").
		Order("created_at DESC").Limit(limit).Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *Repository) CountActiveTasks(ctx context.Context) (int64, error) {
	var n int64
	if err := r.db.WithContext(ctx).Model(&model.MigrationTask{}).
		Where("status IN ?", []string{model.TaskStatusPreparing, model.TaskStatusRunning}).Count(&n).Error; err != nil {
		return 0, err
	}
	return n, nil
}

func (r *Repository) NextPendingTask(ctx context.Context) (*model.MigrationTask, error) {
	var t model.MigrationTask
	if err := r.db.WithContext(ctx).
		Where("status = ? AND queued_at IS NOT NULL", model.TaskStatusPending).
		Order("queued_at ASC, id ASC").First(&t).Error; err != nil {
		return nil, err
	}
	return &t, nil
}

// ---- logs ----

func (r *Repository) AddLog(ctx context.Context, l *model.MigrationLog) error {
	return r.db.WithContext(ctx).Create(l).Error
}

func (r *Repository) AddLogs(ctx context.Context, logs []model.MigrationLog) error {
	if len(logs) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Create(&logs).Error
}

func (r *Repository) ListLogs(ctx context.Context, taskID uint64, limit int) ([]model.MigrationLog, error) {
	var items []model.MigrationLog
	if err := r.db.WithContext(ctx).Where("task_id = ?", taskID).
		Order("id ASC").Limit(limit).Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}
