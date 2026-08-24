package engine

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/dbmove/dbmove/worker/internal/executor"
	"github.com/dbmove/dbmove/worker/internal/reporter"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// PostgreSQLEngine migrates PostgreSQL databases with pg_dump/pg_restore.
type PostgreSQLEngine struct{}

func (e *PostgreSQLEngine) Name() string { return "postgresql-dump" }

var (
	pgDumpStartRE  = regexp.MustCompile(`(?i)dumping contents of table\s+"?([^"\s]+)"?`)
	pgRestoreStart = regexp.MustCompile(`(?i)creating table\s+"?([^"\s]+)"?`)
)

func pgDSN(ci ConnectionInfo, password, database string) string {
	sslMode := ci.SSLMode
	if sslMode == "" {
		sslMode = "prefer"
	}
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s connect_timeout=10",
		ci.Host, ci.Port, ci.Username, password, database, sslMode)
}

func (e *PostgreSQLEngine) Preflight(ctx context.Context, task *TaskConfig, env *Env, rep *reporter.Reporter) error {
	for _, bin := range []string{"pg_dump", "pg_restore"} {
		if err := executor.LookPath(bin); err != nil {
			return fmt.Errorf("migration engine requires %s in PATH", bin)
		}
	}
	pairs := task.DatabasePairs()
	if len(pairs) == 0 {
		return fmt.Errorf("no databases configured for migration")
	}

	rep.Log("INFO", "preflight: checking source connection %s:%d", task.Source.Host, task.Source.Port)
	src, err := sql.Open("pgx", pgDSN(task.Source, env.SourcePassword, pairs[0].Source))
	if err != nil {
		return fmt.Errorf("source connection failed: %w", err)
	}
	defer src.Close()
	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := src.PingContext(pingCtx); err != nil {
		return fmt.Errorf("source connection failed: %w", err)
	}
	rep.Log("INFO", "preflight: source connected")

	rep.Log("INFO", "preflight: checking target connection %s:%d", task.Target.Host, task.Target.Port)
	if _, err := e.targetDatabaseExists(ctx, task, env, pairs[0].Target); err != nil {
		return fmt.Errorf("target connection failed: %w", err)
	}
	rep.Log("INFO", "preflight: target connected")

	srcVersion, err := pgServerVersion(ctx, src)
	if err != nil {
		return fmt.Errorf("query source version failed: %w", err)
	}
	rep.Log("INFO", "preflight: source PostgreSQL version %s", srcVersion)
	main, err := sql.Open("pgx", pgDSN(task.Target, env.TargetPassword, "postgres"))
	if err != nil {
		return fmt.Errorf("target connection failed: %w", err)
	}
	defer main.Close()
	tgtVersion, err := pgServerVersion(ctx, main)
	if err != nil {
		return fmt.Errorf("query target version failed: %w", err)
	}
	rep.Log("INFO", "preflight: target PostgreSQL version %s", tgtVersion)
	clientMajor := pgClientMajor()
	rep.Log("INFO", "preflight: pg_dump client major version %d", clientMajor)
	if warning, fatal := pgVersionCompat(clientMajor, parseMajor(srcVersion), parseMajor(tgtVersion)); fatal != "" {
		return fmt.Errorf("compatibility check failed: %s", fatal)
	} else if warning != "" {
		rep.Log("WARN", "preflight: %s", warning)
	}

	for _, pair := range pairs {
		tableCount, err := e.tableCount(ctx, task, env, pair.Source)
		if err != nil {
			return fmt.Errorf("database %q not accessible: %w", pair.Source, err)
		}
		rep.Log("INFO", "preflight: source database %s has %d tables", pair.Source, tableCount)

		exists, err := e.targetDatabaseExists(ctx, task, env, pair.Target)
		if err != nil {
			return fmt.Errorf("target database %q check failed: %w", pair.Target, err)
		}
		switch {
		case !exists && task.TargetDBPolicy == "error":
			return fmt.Errorf("target database %q does not exist (choose 'create' policy to create it)", pair.Target)
		case !exists:
			rep.Log("INFO", "preflight: target database %q does not exist and will be created", pair.Target)
		case exists && task.TargetDBPolicy == "create":
			return fmt.Errorf("target database %q already exists (choose 'overwrite' policy to replace it)", pair.Target)
		case exists && task.TargetDBPolicy == "overwrite":
			rep.Log("WARN", "preflight: target database %q exists and will be overwritten", pair.Target)
		default:
			rep.Log("INFO", "preflight: target database %q exists (policy: error)", pair.Target)
		}
	}
	rep.Log("INFO", "preflight: %d database(s) ready to migrate", len(pairs))
	return nil
}

func (e *PostgreSQLEngine) Migrate(ctx context.Context, task *TaskConfig, env *Env, rep *reporter.Reporter) error {
	dataDir := env.DataDir
	if dataDir == "" {
		dataDir = "/data"
	}
	_ = os.MkdirAll(dataDir, 0o755)
	pairs := task.DatabasePairs()
	if len(pairs) == 0 {
		return fmt.Errorf("no databases configured for migration")
	}

	agg := &pgProgress{rep: rep}
	start := time.Now()

	for i, pair := range pairs {
		rep.Log("INFO", "migrating database %d/%d: %s -> %s", i+1, len(pairs), pair.Source, pair.Target)
		if err := e.ensureTargetDB(ctx, task, env, rep, pair.Target); err != nil {
			return err
		}

		tableCount, err := e.tableCount(ctx, task, env, pair.Source)
		if err != nil {
			return err
		}
		pairTables := int64(tableCount)
		agg.tablesTotal += pairTables
		agg.report()
		if pairTables == 0 {
			rep.Log("WARN", "source database %s has no tables; skipped", pair.Source)
			continue
		}

		dumpFile := filepath.Join(dataDir, fmt.Sprintf("migration-%d-%d.dump", task.ID, i))
		defer os.Remove(dumpFile)
		rep.Log("INFO", "starting dump of database %q", pair.Source)
		pairDone := int64(0)
		envList := append(os.Environ(), "PGPASSWORD="+env.SourcePassword)
		args := []string{
			"-Fc", "--verbose",
			"--host", task.Source.Host,
			"--port", strconv.Itoa(task.Source.Port),
			"--username", task.Source.Username,
			"--dbname", pair.Source,
			"--file", dumpFile,
		}
		res, err := executor.RunFile(ctx, "pg_dump", args, envList, "", "", func(line string) {
			if m := pgDumpStartRE.FindStringSubmatch(line); m != nil {
				pairDone++
				rep.Log("INFO", "dumping table %s.%s", pair.Source, m[1])
				agg.reportWith(pairDone)
			}
		})
		if err != nil {
			return fmt.Errorf("dump %q failed: %w%s", pair.Source, err, tailSuffix(res.Tail))
		}
		info, _ := os.Stat(dumpFile)
		pairBytes := int64(0)
		if info != nil {
			pairBytes = info.Size()
		}
		agg.bytesTotal += pairBytes
		agg.bytesTransferred += pairBytes
		agg.tablesCompleted += pairTables
		rep.Progress(reporter.ProgressUpdate{
			Progress:         agg.percent(),
			TablesTotal:      agg.tablesTotal,
			TablesCompleted:  agg.tablesCompleted,
			BytesTotal:       agg.bytesTotal,
			BytesTransferred: agg.bytesTransferred,
		})
		rep.Log("INFO", "dump of %q completed: %d tables, %.1f MB", pair.Source, pairTables, float64(pairBytes)/1048576)

		rep.Log("INFO", "restoring %q into %s:%d/%s", pair.Source, task.Target.Host, task.Target.Port, pair.Target)
		restoreArgs := []string{
			"-Fc", "--verbose", "--no-owner", "--no-privileges",
		}
		if task.TargetDBPolicy == "overwrite" {
			restoreArgs = append(restoreArgs, "--clean", "--if-exists")
		}
		restoreArgs = append(restoreArgs,
			"--host", task.Target.Host,
			"--port", strconv.Itoa(task.Target.Port),
			"--username", task.Target.Username,
			"--dbname", pair.Target,
			dumpFile,
		)
		envListT := append(os.Environ(), "PGPASSWORD="+env.TargetPassword)
		restored := int64(0)
		res2, err := executor.RunFile(ctx, "pg_restore", restoreArgs, envListT, "", "", func(line string) {
			if m := pgRestoreStart.FindStringSubmatch(line); m != nil {
				restored++
				agg.reportWith(restored)
			}
			if strings.Contains(line, "error") || strings.Contains(line, "ERROR") {
				rep.Log("ERROR", "%s", line)
			}
		})
		if err != nil {
			return fmt.Errorf("restore %q failed: %w%s", pair.Target, err, tailSuffix(res2.Tail))
		}
		rep.Log("INFO", "database %s -> %s completed", pair.Source, pair.Target)
	}

	elapsed := time.Since(start).Seconds()
	speed := int64(0)
	if elapsed > 0 {
		speed = int64(float64(agg.bytesTransferred) / elapsed)
	}
	rep.Progress(reporter.ProgressUpdate{
		Progress:         100,
		TablesTotal:      agg.tablesTotal,
		TablesCompleted:  agg.tablesTotal,
		BytesTotal:       agg.bytesTotal,
		BytesTransferred: agg.bytesTransferred,
		Speed:            speed,
	})
	rep.Log("INFO", "restore completed: %d database(s), %d tables", len(pairs), agg.tablesTotal)
	return nil
}

func (e *PostgreSQLEngine) targetDatabaseExists(ctx context.Context, task *TaskConfig, env *Env, database string) (bool, error) {
	db, err := sql.Open("pgx", pgDSN(task.Target, env.TargetPassword, database))
	if err != nil {
		return false, err
	}
	defer db.Close()
	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err == nil {
		return true, nil
	}
	main, err := sql.Open("pgx", pgDSN(task.Target, env.TargetPassword, "postgres"))
	if err != nil {
		return false, err
	}
	defer main.Close()
	if err := main.PingContext(pingCtx); err != nil {
		return false, err
	}
	var exists bool
	if err := main.QueryRowContext(ctx,
		"SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = $1)",
		database).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

func (e *PostgreSQLEngine) ensureTargetDB(ctx context.Context, task *TaskConfig, env *Env, rep *reporter.Reporter, database string) error {
	exists, err := e.targetDatabaseExists(ctx, task, env, database)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	if task.TargetDBPolicy == "error" {
		return fmt.Errorf("target database %q does not exist", database)
	}
	main, err := sql.Open("pgx", pgDSN(task.Target, env.TargetPassword, "postgres"))
	if err != nil {
		return err
	}
	defer main.Close()
	quoted := `"` + strings.ReplaceAll(database, `"`, `""`) + `"`
	if _, err := main.ExecContext(ctx, "CREATE DATABASE "+quoted); err != nil {
		return fmt.Errorf("create target database failed: %w", err)
	}
	rep.Log("INFO", "target database %q created", database)
	return nil
}

func (e *PostgreSQLEngine) tableCount(ctx context.Context, task *TaskConfig, env *Env, database string) (int, error) {
	src, err := sql.Open("pgx", pgDSN(task.Source, env.SourcePassword, database))
	if err != nil {
		return 0, err
	}
	defer src.Close()
	var n int
	if err := src.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM information_schema.tables
		 WHERE table_schema NOT IN ('pg_catalog', 'information_schema')`).Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

func pgServerVersion(ctx context.Context, db *sql.DB) (string, error) {
	var v string
	if err := db.QueryRowContext(ctx, "SELECT current_setting('server_version')").Scan(&v); err != nil {
		return "", err
	}
	return v, nil
}

func pgClientMajor() int {
	out, err := exec.Command("pg_dump", "--version").Output()
	if err != nil {
		return 0
	}
	return parseMajor(string(out))
}

// pgProgress aggregates progress across multiple database pairs.
type pgProgress struct {
	rep              progressSink
	tablesTotal      int64
	tablesCompleted  int64
	bytesTotal       int64
	bytesTransferred int64
}

func (a *pgProgress) percent() int {
	if a.tablesTotal == 0 {
		return 0
	}
	return int(a.tablesCompleted * 100 / a.tablesTotal)
}

func (a *pgProgress) report() {
	a.reportWith(0)
}

// reportWith reports overall progress, optionally adding tables completed
// within the current dump/restore phase.
func (a *pgProgress) reportWith(pairDone int64) {
	done := a.tablesCompleted + pairDone
	p := 0
	if a.tablesTotal > 0 {
		p = int(done * 100 / a.tablesTotal)
	}
	if p > 99 {
		p = 99
	}
	a.rep.Progress(reporter.ProgressUpdate{
		Progress:         p,
		TablesTotal:      a.tablesTotal,
		TablesCompleted:  done,
		BytesTotal:       a.bytesTotal,
		BytesTransferred: a.bytesTransferred,
	})
}

var _ Engine = (*PostgreSQLEngine)(nil)

func init() { Register(&PostgreSQLEngine{}) }
