package engine

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/dbmove/dbmove/worker/internal/executor"
	"github.com/dbmove/dbmove/worker/internal/reporter"
	_ "github.com/go-sql-driver/mysql"
)

// MySQLEngine migrates MySQL databases with mysqldump/mysql.
type MySQLEngine struct{}

func (e *MySQLEngine) Name() string { return "mysql-dump" }

var (
	mysqlDumpStartRE = regexp.MustCompile(`(?i)dumping (?:data|table)(?: for table)?\s+["'` + "`" + `[]?([^"'` + "`" + `\]]+)`)
	mysqlDumpEndRE   = regexp.MustCompile(`(?i)finished dumping(?: for table)?\s+["'` + "`" + `[]?([^"'` + "`" + `\]]+)`)
)

func mysqlDSN(ci ConnectionInfo, password string) string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?timeout=10s&parseTime=true&multiStatements=true",
		ci.Username, password, ci.Host, ci.Port, ci.Database)
}

func mysqlServerDSN(ci ConnectionInfo, password string) string {
	ci.Database = ""
	return mysqlDSN(ci, password)
}

func (e *MySQLEngine) Preflight(ctx context.Context, task *TaskConfig, env *Env, rep *reporter.Reporter) error {
	if err := executor.LookPath("mysqldump"); err != nil {
		return fmt.Errorf("migration engine requires mysqldump in PATH")
	}
	if err := executor.LookPath("mysql"); err != nil {
		return fmt.Errorf("migration engine requires mysql client in PATH")
	}
	pairs := task.DatabasePairs()
	if len(pairs) == 0 {
		return fmt.Errorf("no databases configured for migration")
	}

	rep.Log("INFO", "preflight: checking source connection %s:%d", task.Source.Host, task.Source.Port)
	src, err := sql.Open("mysql", mysqlServerDSN(task.Source, env.SourcePassword))
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
	tgt, err := sql.Open("mysql", mysqlServerDSN(task.Target, env.TargetPassword))
	if err != nil {
		return fmt.Errorf("target connection failed: %w", err)
	}
	defer tgt.Close()
	if err := tgt.PingContext(pingCtx); err != nil {
		return fmt.Errorf("target connection failed: %w", err)
	}
	rep.Log("INFO", "preflight: target connected")

	srcVersion, err := mysqlServerVersion(ctx, src)
	if err != nil {
		return fmt.Errorf("query source version failed: %w", err)
	}
	rep.Log("INFO", "preflight: source MySQL version %s", srcVersion)
	tgtVersion, err := mysqlServerVersion(ctx, tgt)
	if err != nil {
		return fmt.Errorf("query target version failed: %w", err)
	}
	rep.Log("INFO", "preflight: target MySQL version %s", tgtVersion)
	if warning, fatal := mysqlVersionCompat(srcVersion, tgtVersion); fatal != "" {
		return fmt.Errorf("compatibility check failed: %s", fatal)
	} else if warning != "" {
		rep.Log("WARN", "preflight: %s", warning)
	}

	for _, pair := range pairs {
		var tableCount int
		if err := src.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = ?",
			pair.Source).Scan(&tableCount); err != nil {
			return fmt.Errorf("database %q not accessible: %w", pair.Source, err)
		}
		rep.Log("INFO", "preflight: source database %s has %d tables", pair.Source, tableCount)

		var exists int
		if err := tgt.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM information_schema.schemata WHERE schema_name = ?",
			pair.Target).Scan(&exists); err != nil {
			return fmt.Errorf("check target database failed: %w", err)
		}
		switch {
		case exists == 0 && task.TargetDBPolicy == "error":
			return fmt.Errorf("target database %q does not exist (choose 'create' policy to create it)", pair.Target)
		case exists == 0:
			rep.Log("INFO", "preflight: target database %q does not exist and will be created", pair.Target)
		case exists > 0 && task.TargetDBPolicy == "create":
			return fmt.Errorf("target database %q already exists (choose 'overwrite' policy to replace it)", pair.Target)
		case exists > 0 && task.TargetDBPolicy == "overwrite":
			rep.Log("WARN", "preflight: target database %q exists and will be overwritten", pair.Target)
		default:
			rep.Log("INFO", "preflight: target database %q exists (policy: error)", pair.Target)
		}
	}
	rep.Log("INFO", "preflight: %d database(s) ready to migrate", len(pairs))
	return nil
}

func (e *MySQLEngine) Migrate(ctx context.Context, task *TaskConfig, env *Env, rep *reporter.Reporter) error {
	dataDir := env.DataDir
	if dataDir == "" {
		dataDir = "/data"
	}
	_ = os.MkdirAll(dataDir, 0o755)
	pairs := task.DatabasePairs()
	if len(pairs) == 0 {
		return fmt.Errorf("no databases configured for migration")
	}

	agg := &mysqlProgress{rep: rep}
	start := time.Now()

	for i, pair := range pairs {
		rep.Log("INFO", "migrating database %d/%d: %s -> %s", i+1, len(pairs), pair.Source, pair.Target)
		if err := e.ensureTargetDB(ctx, task, env, rep, pair); err != nil {
			return err
		}

		tables, rows, err := e.tableStats(ctx, task, env, pair.Source)
		if err != nil {
			return err
		}
		pairTables := int64(len(tables))
		agg.tablesTotal += pairTables
		agg.rowsTotal += rows
		agg.report()
		if pairTables == 0 {
			rep.Log("WARN", "source database %s has no tables; skipped", pair.Source)
			continue
		}

		dumpFile := filepath.Join(dataDir, fmt.Sprintf("migration-%d-%d.sql", task.ID, i))
		defer os.Remove(dumpFile)
		rep.Log("INFO", "starting dump of database %q", pair.Source)
		pairDone := int64(0)
		envList := append(os.Environ(), "MYSQL_PWD="+env.SourcePassword)
		args := []string{
			"--single-transaction", "--routines", "--triggers", "--verbose",
			"--add-drop-table",
			"--host", task.Source.Host,
			"--port", strconv.Itoa(task.Source.Port),
			"--user", task.Source.Username,
			pair.Source,
		}
		res, err := executor.RunFile(ctx, "mysqldump", args, envList, "", dumpFile, func(line string) {
			if m := mysqlDumpStartRE.FindStringSubmatch(line); m != nil {
				name := strings.Trim(m[1], " `'\"[]")
				if name != "" {
					rep.Log("INFO", "dumping table %s.%s", pair.Source, name)
				}
			}
			if mysqlDumpEndRE.MatchString(line) {
				pairDone++
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
		agg.rowsCompleted += rows
		rep.Progress(reporter.ProgressUpdate{
			Progress:         agg.percent(),
			TablesTotal:      agg.tablesTotal,
			TablesCompleted:  agg.tablesCompleted,
			RowsTotal:        agg.rowsTotal,
			RowsCompleted:    agg.rowsCompleted,
			BytesTotal:       agg.bytesTotal,
			BytesTransferred: agg.bytesTransferred,
		})
		rep.Log("INFO", "dump of %q completed: %d tables, %.1f MB", pair.Source, pairTables, float64(pairBytes)/1048576)

		rep.Log("INFO", "restoring %q into %s:%d/%s", pair.Source, task.Target.Host, task.Target.Port, pair.Target)
		restoreArgs := []string{
			"--default-character-set=utf8mb4",
			"--host", task.Target.Host,
			"--port", strconv.Itoa(task.Target.Port),
			"--user", task.Target.Username,
			pair.Target,
		}
		envListT := append(os.Environ(), "MYSQL_PWD="+env.TargetPassword)
		res2, err := executor.RunFile(ctx, "mysql", restoreArgs, envListT, dumpFile, "", func(line string) {
			if strings.Contains(line, "ERROR") {
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
		RowsTotal:        agg.rowsTotal,
		RowsCompleted:    agg.rowsTotal,
		BytesTotal:       agg.bytesTotal,
		BytesTransferred: agg.bytesTransferred,
		Speed:            speed,
	})
	rep.Log("INFO", "restore completed: %d database(s), %d tables", len(pairs), agg.tablesTotal)
	return nil
}

func (e *MySQLEngine) ensureTargetDB(ctx context.Context, task *TaskConfig, env *Env, rep *reporter.Reporter, pair DatabasePair) error {
	tgt, err := sql.Open("mysql", mysqlServerDSN(task.Target, env.TargetPassword))
	if err != nil {
		return err
	}
	defer tgt.Close()
	var exists int
	if err := tgt.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM information_schema.schemata WHERE schema_name = ?",
		pair.Target).Scan(&exists); err != nil {
		return err
	}
	if exists > 0 {
		return nil
	}
	if task.TargetDBPolicy == "error" {
		return fmt.Errorf("target database %q does not exist", pair.Target)
	}
	quoted := "`" + strings.ReplaceAll(pair.Target, "`", "``") + "`"
	if _, err := tgt.ExecContext(ctx, "CREATE DATABASE "+quoted+" CHARACTER SET utf8mb4"); err != nil {
		return fmt.Errorf("create target database failed: %w", err)
	}
	rep.Log("INFO", "target database %q created", pair.Target)
	return nil
}

func (e *MySQLEngine) tableStats(ctx context.Context, task *TaskConfig, env *Env, database string) ([]string, int64, error) {
	src, err := sql.Open("mysql", mysqlDSN(task.Source, env.SourcePassword))
	if err != nil {
		return nil, 0, err
	}
	defer src.Close()
	rows, err := src.QueryContext(ctx,
		`SELECT table_name, COALESCE(table_rows, 0) FROM information_schema.tables
		 WHERE table_schema = ? ORDER BY table_name`, database)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var tables []string
	var totalRows int64
	for rows.Next() {
		var name string
		var r int64
		if err := rows.Scan(&name, &r); err != nil {
			return nil, 0, err
		}
		tables = append(tables, name)
		totalRows += r
	}
	return tables, totalRows, rows.Err()
}

func mysqlServerVersion(ctx context.Context, db *sql.DB) (string, error) {
	var v string
	if err := db.QueryRowContext(ctx, "SELECT VERSION()").Scan(&v); err != nil {
		return "", err
	}
	return v, nil
}

// mysqlProgress aggregates progress across multiple database pairs.
type mysqlProgress struct {
	rep              progressSink
	tablesTotal      int64
	tablesCompleted  int64
	rowsTotal        int64
	rowsCompleted    int64
	bytesTotal       int64
	bytesTransferred int64
}

func (a *mysqlProgress) percent() int {
	if a.tablesTotal == 0 {
		return 0
	}
	return int(a.tablesCompleted * 100 / a.tablesTotal)
}

func (a *mysqlProgress) report() {
	a.reportWith(0)
}

// reportWith reports overall progress, optionally adding tables completed
// within the current dump phase.
func (a *mysqlProgress) reportWith(pairDone int64) {
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
		RowsTotal:        a.rowsTotal,
		RowsCompleted:    a.rowsCompleted,
		BytesTotal:       a.bytesTotal,
		BytesTransferred: a.bytesTransferred,
	})
}

var _ Engine = (*MySQLEngine)(nil)

func init() { Register(&MySQLEngine{}) }
