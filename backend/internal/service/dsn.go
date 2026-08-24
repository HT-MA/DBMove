package service

import (
	"fmt"

	"github.com/dbmove/dbmove/backend/internal/model"
	"github.com/go-sql-driver/mysql"
)

func mysqlConfig(conn *model.Connection, password, database string) *mysql.Config {
	cfg := mysql.NewConfig()
	cfg.User = conn.Username
	cfg.Passwd = password
	cfg.Net = "tcp"
	cfg.Addr = fmt.Sprintf("%s:%d", conn.Host, conn.Port)
	cfg.DBName = database
	cfg.Timeout = 10e9 // 10s
	cfg.ReadTimeout = 30e9
	cfg.ParseTime = true
	cfg.Params = map[string]string{"charset": "utf8mb4"}
	switch conn.SSLMode {
	case "disable", "disabled":
		cfg.TLSConfig = "false"
	case "required", "require", "verify-ca", "verify-full":
		cfg.TLSConfig = "true"
	case "preferred", "prefer", "":
		cfg.TLSConfig = "preferred"
	default:
		cfg.TLSConfig = "preferred"
	}
	return cfg
}

func pgDSN(conn *model.Connection, password, database string) string {
	sslMode := conn.SSLMode
	if sslMode == "" {
		sslMode = "prefer"
	}
	// An empty database name makes pgx default to a database named after the
	// user, which usually does not exist. Fall back to the maintenance DB so
	// connection tests and database listing work without a stored database.
	if database == "" {
		database = "postgres"
	}
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s connect_timeout=10",
		conn.Host, conn.Port, conn.Username, password, database, sslMode)
}
