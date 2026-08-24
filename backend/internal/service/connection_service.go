package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dbmove/dbmove/backend/internal/crypto"
	"github.com/dbmove/dbmove/backend/internal/model"
	"github.com/dbmove/dbmove/backend/internal/repository"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	"gorm.io/gorm"
)

// ConnectionInput is the accepted payload for creating/updating a connection.
type ConnectionInput struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Host        string `json:"host"`
	Port        int    `json:"port"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	Database    string `json:"database"`
	SSLMode     string `json:"ssl_mode"`
	Description string `json:"description"`
}

// ConnectionDTO is the API representation (never contains the password).
type ConnectionDTO struct {
	ID          uint64    `json:"id"`
	Name        string    `json:"name"`
	Type        string    `json:"type"`
	Host        string    `json:"host"`
	Port        int       `json:"port"`
	Username    string    `json:"username"`
	Database    string    `json:"database"`
	SSLMode     string    `json:"ssl_mode"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// TestResult is returned by the test connection endpoint.
type TestResult struct {
	Success   bool   `json:"success"`
	Version   string `json:"version"`
	LatencyMs int64  `json:"latency_ms"`
}

// ConnectionService implements connection management.
type ConnectionService struct {
	repo   *repository.Repository
	cipher *crypto.Cipher
}

func NewConnectionService(repo *repository.Repository, cipher *crypto.Cipher) *ConnectionService {
	return &ConnectionService{repo: repo, cipher: cipher}
}

// ErrConnectionNotFound is returned when a connection does not exist.
var ErrConnectionNotFound = errors.New("connection not found")

// ErrConnectionInUse is returned when a connection is referenced by tasks.
var ErrConnectionInUse = errors.New("connection is in use by migration tasks")

func ToConnectionDTO(c *model.Connection) ConnectionDTO {
	return ConnectionDTO{
		ID:          c.ID,
		Name:        c.Name,
		Type:        c.Type,
		Host:        c.Host,
		Port:        c.Port,
		Username:    c.Username,
		Database:    c.DatabaseName,
		SSLMode:     c.SSLMode,
		Description: c.Description,
		CreatedAt:   c.CreatedAt,
		UpdatedAt:   c.UpdatedAt,
	}
}

// Create validates input, encrypts the password and persists the connection.
func (s *ConnectionService) Create(ctx context.Context, in ConnectionInput) (*ConnectionDTO, error) {
	if err := ValidateConnectionInput(in); err != nil {
		return nil, err
	}
	encrypted, err := s.cipher.Encrypt(in.Password)
	if err != nil {
		return nil, fmt.Errorf("encrypt password: %w", err)
	}
	conn := &model.Connection{
		Name:              strings.TrimSpace(in.Name),
		Type:              in.Type,
		Host:              strings.TrimSpace(in.Host),
		Port:              in.Port,
		Username:          in.Username,
		PasswordEncrypted: encrypted,
		DatabaseName:      in.Database,
		SSLMode:           in.SSLMode,
		Description:       in.Description,
	}
	if err := s.repo.CreateConnection(ctx, conn); err != nil {
		return nil, fmt.Errorf("create connection: %w", err)
	}
	dto := ToConnectionDTO(conn)
	return &dto, nil
}

// Update applies partial updates; an empty password keeps the stored secret.
func (s *ConnectionService) Update(ctx context.Context, id uint64, in ConnectionInput) (*ConnectionDTO, error) {
	conn, err := s.repo.GetConnection(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrConnectionNotFound
		}
		return nil, err
	}
	if strings.TrimSpace(in.Name) != "" {
		conn.Name = strings.TrimSpace(in.Name)
	}
	if in.Type != "" {
		conn.Type = in.Type
	}
	if in.Host != "" {
		conn.Host = strings.TrimSpace(in.Host)
	}
	if in.Port > 0 {
		conn.Port = in.Port
	}
	if in.Username != "" {
		conn.Username = in.Username
	}
	if in.Database != "" {
		conn.DatabaseName = in.Database
	}
	if in.SSLMode != "" {
		conn.SSLMode = in.SSLMode
	}
	if in.Description != "" {
		conn.Description = in.Description
	}
	if in.Password != "" {
		enc, err := s.cipher.Encrypt(in.Password)
		if err != nil {
			return nil, fmt.Errorf("encrypt password: %w", err)
		}
		conn.PasswordEncrypted = enc
	}
	if err := s.repo.UpdateConnection(ctx, conn); err != nil {
		return nil, fmt.Errorf("update connection: %w", err)
	}
	dto := ToConnectionDTO(conn)
	return &dto, nil
}

// Delete removes a connection unless tasks reference it.
func (s *ConnectionService) Delete(ctx context.Context, id uint64) error {
	_, err := s.repo.GetConnection(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrConnectionNotFound
		}
		return err
	}
	inUse, err := s.repo.ConnectionInUse(ctx, id)
	if err != nil {
		return err
	}
	if inUse {
		return ErrConnectionInUse
	}
	return s.repo.DeleteConnection(ctx, id)
}

// List returns all connections as DTOs.
func (s *ConnectionService) List(ctx context.Context) ([]ConnectionDTO, error) {
	items, err := s.repo.ListConnections(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]ConnectionDTO, 0, len(items))
	for i := range items {
		out = append(out, ToConnectionDTO(&items[i]))
	}
	return out, nil
}

// Get returns one connection as a DTO.
func (s *ConnectionService) Get(ctx context.Context, id uint64) (*ConnectionDTO, error) {
	conn, err := s.repo.GetConnection(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrConnectionNotFound
		}
		return nil, err
	}
	dto := ToConnectionDTO(conn)
	return &dto, nil
}

// TestByID tests a stored connection.
func (s *ConnectionService) TestByID(ctx context.Context, id uint64) (*TestResult, error) {
	conn, err := s.repo.GetConnection(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrConnectionNotFound
		}
		return nil, err
	}
	password, err := s.cipher.Decrypt(conn.PasswordEncrypted)
	if err != nil {
		return nil, fmt.Errorf("decrypt password: %w", err)
	}
	return TestConnection(ctx, conn, password)
}

// TestByValues tests an unsaved connection configuration.
func (s *ConnectionService) TestByValues(ctx context.Context, in ConnectionInput) (*TestResult, error) {
	if err := ValidateConnectionInput(in); err != nil {
		return nil, err
	}
	conn := &model.Connection{
		Type:         in.Type,
		Host:         in.Host,
		Port:         in.Port,
		Username:     in.Username,
		DatabaseName: in.Database,
		SSLMode:      in.SSLMode,
	}
	return TestConnection(ctx, conn, in.Password)
}

// Databases lists database names visible through a connection.
func (s *ConnectionService) Databases(ctx context.Context, id uint64) ([]string, error) {
	conn, err := s.repo.GetConnection(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrConnectionNotFound
		}
		return nil, err
	}
	password, err := s.cipher.Decrypt(conn.PasswordEncrypted)
	if err != nil {
		return nil, fmt.Errorf("decrypt password: %w", err)
	}
	return ListDatabases(ctx, conn, password)
}

// ValidateConnectionInput checks required fields.
func ValidateConnectionInput(in ConnectionInput) error {
	if strings.TrimSpace(in.Name) == "" {
		return errors.New("name is required")
	}
	switch in.Type {
	case model.ConnTypeMySQL, model.ConnTypePostgreSQL:
	default:
		return fmt.Errorf("unsupported database type %q (MVP supports mysql and postgresql)", in.Type)
	}
	if strings.TrimSpace(in.Host) == "" {
		return errors.New("host is required")
	}
	if in.Port <= 0 {
		return errors.New("port must be a positive integer")
	}
	return nil
}

// TestConnection opens a connection to the configured database.
func TestConnection(ctx context.Context, conn *model.Connection, password string) (*TestResult, error) {
	start := time.Now()
	db, err := openDB(conn, password, conn.DatabaseName)
	if err != nil {
		return nil, fmt.Errorf("failed to connect database: %w", err)
	}
	defer db.Close()

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		return nil, fmt.Errorf("failed to connect database: %w", err)
	}

	version, err := serverVersion(pingCtx, db, conn.Type)
	if err != nil {
		return nil, fmt.Errorf("query version: %w", err)
	}
	return &TestResult{
		Success:   true,
		Version:   version,
		LatencyMs: time.Since(start).Milliseconds(),
	}, nil
}

// ListDatabases returns the database names visible through a connection.
func ListDatabases(ctx context.Context, conn *model.Connection, password string) ([]string, error) {
	db, err := openDB(conn, password, "")
	if err != nil {
		return nil, fmt.Errorf("failed to connect database: %w", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var query string
	switch conn.Type {
	case model.ConnTypeMySQL:
		query = "SHOW DATABASES"
	case model.ConnTypePostgreSQL:
		query = "SELECT datname FROM pg_database WHERE datistemplate = false ORDER BY datname"
	default:
		return nil, fmt.Errorf("unsupported database type %q", conn.Type)
	}
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list databases: %w", err)
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		if conn.Type == model.ConnTypeMySQL && mysqlSystemDB(name) {
			continue
		}
		out = append(out, name)
	}
	return out, rows.Err()
}

// mysqlSystemDB reports whether a MySQL schema name is a system database that
// should never be selected as a migration source or target.
func mysqlSystemDB(name string) bool {
	switch name {
	case "information_schema", "performance_schema", "mysql", "sys":
		return true
	}
	return false
}

func openDB(conn *model.Connection, password, database string) (*sql.DB, error) {
	dsn, err := buildDSN(conn, password, database)
	if err != nil {
		return nil, err
	}
	driver := conn.Type
	if conn.Type == model.ConnTypePostgreSQL {
		driver = "pgx"
	}
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(5)
	db.SetConnMaxLifetime(2 * time.Minute)
	return db, nil
}

func buildDSN(conn *model.Connection, password, database string) (string, error) {
	switch conn.Type {
	case model.ConnTypeMySQL:
		cfg := mysqlConfig(conn, password, database)
		return cfg.FormatDSN(), nil
	case model.ConnTypePostgreSQL:
		return pgDSN(conn, password, database), nil
	default:
		return "", fmt.Errorf("unsupported database type %q", conn.Type)
	}
}

func serverVersion(ctx context.Context, db *sql.DB, dbType string) (string, error) {
	var v string
	switch dbType {
	case model.ConnTypeMySQL:
		if err := db.QueryRowContext(ctx, "SELECT VERSION()").Scan(&v); err != nil {
			return "", err
		}
		return v, nil
	case model.ConnTypePostgreSQL:
		if err := db.QueryRowContext(ctx, "SELECT version()").Scan(&v); err != nil {
			return "", err
		}
		parts := strings.SplitN(v, ",", 2)
		return strings.TrimSpace(parts[0]), nil
	}
	return "", fmt.Errorf("unsupported database type %q", dbType)
}
