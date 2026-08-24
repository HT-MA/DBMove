package engine

import (
	"context"
	"fmt"

	"github.com/dbmove/dbmove/worker/internal/reporter"
)

// ConnectionInfo describes one endpoint of a migration.
type ConnectionInfo struct {
	Type     string `json:"type"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Database string `json:"database"`
	SSLMode  string `json:"ssl_mode"`
}

// DatabasePair maps one source database to one target database.
type DatabasePair struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

// TaskConfig is the migration configuration the worker receives from the
// backend internal API. Passwords are injected separately via env vars.
type TaskConfig struct {
	ID             uint64         `json:"id"`
	Name           string         `json:"name"`
	MigrationType  string         `json:"migration_type"`
	Engine         string         `json:"engine"`
	TargetDBPolicy string         `json:"target_db_policy"`
	Databases      []DatabasePair `json:"databases"`
	Source         ConnectionInfo `json:"source"`
	Target         ConnectionInfo `json:"target"`
}

// DatabasePairs returns the configured source->target mapping, falling back
// to the legacy single-database fields.
func (t *TaskConfig) DatabasePairs() []DatabasePair {
	if len(t.Databases) > 0 {
		return t.Databases
	}
	if t.Source.Database != "" {
		return []DatabasePair{{Source: t.Source.Database, Target: t.Target.Database}}
	}
	return nil
}

// Env carries secrets and runtime paths.
type Env struct {
	SourcePassword string
	TargetPassword string
	DataDir        string
}

// Engine runs the migration for one database family.
type Engine interface {
	Name() string
	Preflight(ctx context.Context, task *TaskConfig, env *Env, rep *reporter.Reporter) error
	Migrate(ctx context.Context, task *TaskConfig, env *Env, rep *reporter.Reporter) error
}

// progressSink is the subset of the reporter used by progress aggregators.
type progressSink interface {
	Progress(reporter.ProgressUpdate)
}

// Registry maps engine names to implementations.
var registry = map[string]Engine{}

func Register(e Engine) {
	registry[e.Name()] = e
}

// Get returns the engine for a name.
func Get(name string) (Engine, error) {
	e, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("engine %q not found", name)
	}
	return e, nil
}
