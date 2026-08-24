package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

// Database connection types.
const (
	ConnTypeMySQL      = "mysql"
	ConnTypePostgreSQL = "postgresql"
	ConnTypeDM8        = "dm8"
	ConnTypeRedis      = "redis"
)

// Migration types (MVP supports FULL only).
const (
	MigrationTypeFull = "FULL"
)

// Migration task statuses.
const (
	TaskStatusPending   = "PENDING"
	TaskStatusPreparing = "PREPARING"
	TaskStatusRunning   = "RUNNING"
	TaskStatusSuccess   = "SUCCESS"
	TaskStatusFailed    = "FAILED"
	TaskStatusCancelled = "CANCELLED"
)

// Target database policy when a migration starts.
const (
	TargetDBPolicyError     = "error"     // refuse if target database exists
	TargetDBPolicyCreate    = "create"    // create if missing, refuse if exists
	TargetDBPolicyOverwrite = "overwrite" // create if missing, overwrite if exists
)

// Connection is a managed database connection. Passwords are stored encrypted.
type Connection struct {
	ID                uint64    `gorm:"primaryKey" json:"id"`
	Name              string    `gorm:"size:100;not null" json:"name"`
	Type              string    `gorm:"size:30;not null" json:"type"`
	Host              string    `gorm:"size:255;not null" json:"host"`
	Port              int       `gorm:"not null" json:"port"`
	Username          string    `gorm:"size:255" json:"username"`
	PasswordEncrypted string    `gorm:"column:password_encrypted;type:text" json:"-"`
	DatabaseName      string    `gorm:"column:database_name;size:255" json:"database"`
	SSLMode           string    `gorm:"column:ssl_mode;size:50" json:"ssl_mode"`
	Description       string    `gorm:"type:text" json:"description"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// TableName for Connection.
func (Connection) TableName() string { return "connections" }

// MigrationTask represents one database migration.
type MigrationTask struct {
	ID                 uint64          `gorm:"primaryKey" json:"id"`
	Name               string          `gorm:"size:200;not null" json:"name"`
	SourceConnectionID uint64          `gorm:"column:source_connection_id;not null" json:"source_connection_id"`
	TargetConnectionID uint64          `gorm:"column:target_connection_id;not null" json:"target_connection_id"`
	SourceDatabase     string          `gorm:"column:source_database;size:255" json:"source_database"`
	TargetDatabase     string          `gorm:"column:target_database;size:255" json:"target_database"`
	Databases          DatabaseMapping `gorm:"type:jsonb" json:"databases"`
	MigrationType      string          `gorm:"column:migration_type;size:50;not null" json:"migration_type"`
	TargetDBPolicy     string          `gorm:"column:target_db_policy;size:30;not null;default:error" json:"target_db_policy"`
	Engine             string          `gorm:"size:50" json:"engine"`
	Status             string          `gorm:"size:30;not null" json:"status"`
	Progress           int             `gorm:"default:0" json:"progress"`
	TablesTotal        int64           `gorm:"column:tables_total;default:0" json:"tables_total"`
	TablesCompleted    int64           `gorm:"column:tables_completed;default:0" json:"tables_completed"`
	RowsTotal          int64           `gorm:"column:rows_total;default:0" json:"rows_total"`
	RowsCompleted      int64           `gorm:"column:rows_completed;default:0" json:"rows_completed"`
	BytesTotal         int64           `gorm:"column:bytes_total;default:0" json:"bytes_total"`
	BytesTransferred   int64           `gorm:"column:bytes_transferred;default:0" json:"bytes_transferred"`
	Speed              int64           `gorm:"default:0" json:"speed"`
	ErrorMessage       string          `gorm:"column:error_message;type:text" json:"error_message"`
	StartedAt          *time.Time      `gorm:"column:started_at" json:"started_at"`
	FinishedAt         *time.Time      `gorm:"column:finished_at" json:"finished_at"`
	QueuedAt           *time.Time      `gorm:"column:queued_at" json:"queued_at"`
	CreatedBy          string          `gorm:"column:created_by;size:100" json:"created_by"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`

	SourceConnection *Connection `gorm:"foreignKey:SourceConnectionID" json:"source_connection"`
	TargetConnection *Connection `gorm:"foreignKey:TargetConnectionID" json:"target_connection"`
}

// DatabasePair maps one source database to one target database.
type DatabasePair struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

// DatabaseMapping is the list of source->target database pairs for a task.
type DatabaseMapping []DatabasePair

// Value implements driver.Valuer for jsonb storage.
func (m DatabaseMapping) Value() (driver.Value, error) {
	if m == nil {
		m = DatabaseMapping{}
	}
	b, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

// Scan implements sql.Scanner for jsonb storage.
func (m *DatabaseMapping) Scan(value any) error {
	if value == nil {
		*m = DatabaseMapping{}
		return nil
	}
	var b []byte
	switch v := value.(type) {
	case []byte:
		b = v
	case string:
		b = []byte(v)
	default:
		return fmt.Errorf("cannot scan %T into DatabaseMapping", value)
	}
	if len(b) == 0 {
		*m = DatabaseMapping{}
		return nil
	}
	return json.Unmarshal(b, m)
}

// DatabasePairs returns the configured mapping, falling back to the legacy
// single-database fields when the mapping column is empty.
func (t *MigrationTask) DatabasePairs() []DatabasePair {
	if len(t.Databases) > 0 {
		return t.Databases
	}
	if t.SourceDatabase != "" {
		return []DatabasePair{{Source: t.SourceDatabase, Target: t.TargetDatabase}}
	}
	return nil
}

// TableName for MigrationTask.
func (MigrationTask) TableName() string { return "migration_tasks" }

// MigrationLog is one log line belonging to a migration task.
type MigrationLog struct {
	ID        uint64    `gorm:"primaryKey" json:"id"`
	TaskID    uint64    `gorm:"column:task_id;not null;index" json:"task_id"`
	Level     string    `gorm:"size:20;not null" json:"level"`
	Message   string    `gorm:"type:text;not null" json:"message"`
	CreatedAt time.Time `json:"created_at"`
}

// TableName for MigrationLog.
func (MigrationLog) TableName() string { return "migration_logs" }
