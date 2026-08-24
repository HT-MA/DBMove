-- Reference schema for the DBMove platform database (PostgreSQL).
-- The backend also runs GORM AutoMigrate on startup.

CREATE TABLE IF NOT EXISTS connections (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    type VARCHAR(30) NOT NULL,
    host VARCHAR(255) NOT NULL,
    port INTEGER NOT NULL,
    username VARCHAR(255),
    password_encrypted TEXT,
    database_name VARCHAR(255),
    ssl_mode VARCHAR(50),
    description TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS migration_tasks (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(200) NOT NULL,
    source_connection_id BIGINT NOT NULL,
    target_connection_id BIGINT NOT NULL,
    source_database VARCHAR(255),
    target_database VARCHAR(255),
    databases JSONB,
    migration_type VARCHAR(50) NOT NULL,
    target_db_policy VARCHAR(30) NOT NULL DEFAULT 'error',
    engine VARCHAR(50),
    status VARCHAR(30) NOT NULL,
    progress INTEGER DEFAULT 0,
    tables_total BIGINT DEFAULT 0,
    tables_completed BIGINT DEFAULT 0,
    rows_total BIGINT DEFAULT 0,
    rows_completed BIGINT DEFAULT 0,
    bytes_total BIGINT DEFAULT 0,
    bytes_transferred BIGINT DEFAULT 0,
    speed BIGINT DEFAULT 0,
    error_message TEXT,
    started_at TIMESTAMP,
    finished_at TIMESTAMP,
    queued_at TIMESTAMP,
    created_by VARCHAR(100),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS migration_logs (
    id BIGSERIAL PRIMARY KEY,
    task_id BIGINT NOT NULL,
    level VARCHAR(20) NOT NULL,
    message TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_migration_logs_task ON migration_logs (task_id);
