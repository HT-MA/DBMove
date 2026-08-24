export type ConnType = 'mysql' | 'postgresql' | 'dm8' | 'redis';
export type TaskStatus = 'PENDING' | 'PREPARING' | 'RUNNING' | 'SUCCESS' | 'FAILED' | 'CANCELLED';
export type TargetDBPolicy = 'error' | 'create' | 'overwrite';
export type MigrationType = 'FULL';

export interface DatabasePair {
  source: string;
  target: string;
}

export interface Connection {
  id: number;
  name: string;
  type: ConnType;
  host: string;
  port: number;
  username: string;
  database: string;
  ssl_mode?: string;
  description?: string;
  created_at: string;
  updated_at: string;
}

export interface ConnectionInput {
  name: string;
  type: ConnType;
  host: string;
  port: number;
  username?: string;
  password?: string;
  database?: string;
  ssl_mode?: string;
  description?: string;
}

export interface ConnectionTestResult {
  success: boolean;
  version: string;
  latency_ms: number;
}

export interface MigrationTask {
  id: number;
  name: string;
  source_connection_id: number;
  target_connection_id: number;
  source_database: string;
  target_database: string;
  databases?: DatabasePair[];
  migration_type: string;
  target_db_policy: TargetDBPolicy;
  engine: string;
  status: TaskStatus;
  progress: number;
  tables_total: number;
  tables_completed: number;
  rows_total: number;
  rows_completed: number;
  bytes_total: number;
  bytes_transferred: number;
  speed: number;
  error_message?: string;
  started_at?: string | null;
  finished_at?: string | null;
  queued_at?: string | null;
  created_by?: string;
  created_at: string;
  updated_at: string;
  source_connection?: Connection;
  target_connection?: Connection;
}

export interface MigrationInput {
  name: string;
  source_connection_id: number;
  target_connection_id: number;
  source_database?: string;
  target_database?: string;
  databases?: DatabasePair[];
  migration_type: MigrationType;
  target_db_policy: TargetDBPolicy;
  created_by?: string;
}

export interface MigrationLog {
  id: number;
  task_id: number;
  level: string;
  message: string;
  created_at: string;
}

export interface PageResult<T> {
  items: T[];
  total: number;
  page: number;
  page_size: number;
}

export interface Stats {
  connections: {
    total: number;
    mysql: number;
    postgresql: number;
    dm8: number;
    redis: number;
  };
  migrations: {
    total: number;
    pending: number;
    preparing: number;
    running: number;
    success: number;
    failed: number;
    cancelled: number;
  };
  recent_migrations: MigrationTask[];
}

export interface AppInfo {
  name: string;
  version: string;
  execution_mode: string;
  max_concurrent_migrations: number;
  supported_databases: string[];
  supported_migration_types: string[];
}
