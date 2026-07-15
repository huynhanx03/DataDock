package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type migration struct {
	version int
	name    string
	apply   func(context.Context, *sql.Tx) error
}

var migrations = []migration{
	{version: 1, name: "initial metadata", apply: migrateInitialMetadata},
	{version: 2, name: "connection metadata", apply: migrateConnectionMetadata},
	{version: 3, name: "query metadata", apply: migrateQueryMetadata},
}

func migrate(ctx context.Context, database *sql.DB) error {
	if _, err := database.ExecContext(ctx, `PRAGMA foreign_keys = ON`); err != nil {
		return fmt.Errorf("enable sqlite foreign keys: %w", err)
	}
	if _, err := database.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY, name TEXT NOT NULL, applied_at TEXT NOT NULL)`); err != nil {
		return fmt.Errorf("create schema migrations table: %w", err)
	}
	for _, item := range migrations {
		if err := applyMigration(ctx, database, item); err != nil {
			return err
		}
	}
	return nil
}

func applyMigration(ctx context.Context, database *sql.DB, item migration) error {
	transaction, err := database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration %d: %w", item.version, err)
	}
	defer transaction.Rollback()
	var applied int
	err = transaction.QueryRowContext(ctx, `SELECT COUNT(*) FROM schema_migrations WHERE version = ?`, item.version).Scan(&applied)
	if err != nil {
		return fmt.Errorf("check migration %d: %w", item.version, err)
	}
	if applied != 0 {
		return transaction.Commit()
	}
	if err := item.apply(ctx, transaction); err != nil {
		return fmt.Errorf("apply migration %d (%s): %w", item.version, item.name, err)
	}
	if _, err := transaction.ExecContext(ctx, `INSERT INTO schema_migrations (version, name, applied_at) VALUES (?, ?, ?)`, item.version, item.name, time.Now().UTC().Format(timeFormat)); err != nil {
		return fmt.Errorf("record migration %d: %w", item.version, err)
	}
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit migration %d: %w", item.version, err)
	}
	return nil
}

func migrateInitialMetadata(ctx context.Context, transaction *sql.Tx) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS workspaces (id TEXT PRIMARY KEY, name TEXT NOT NULL, icon TEXT NOT NULL, color TEXT NOT NULL, position INTEGER NOT NULL, collapsed INTEGER NOT NULL DEFAULT 0, created_at TEXT NOT NULL, updated_at TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS connections (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE, name TEXT NOT NULL, engine TEXT NOT NULL, host TEXT NOT NULL, port INTEGER NOT NULL, database_name TEXT NOT NULL, username TEXT NOT NULL, password_cipher TEXT NOT NULL, ssl_mode TEXT NOT NULL, read_only INTEGER NOT NULL DEFAULT 0, auto_reconnect INTEGER NOT NULL DEFAULT 1, max_open_conns INTEGER NOT NULL, max_idle_conns INTEGER NOT NULL, conn_max_lifetime INTEGER NOT NULL, favorite INTEGER NOT NULL DEFAULT 0, created_at TEXT NOT NULL, updated_at TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS query_history (id TEXT PRIMARY KEY, connection_id TEXT NOT NULL, sql_text TEXT NOT NULL, status TEXT NOT NULL, duration_ms INTEGER NOT NULL, row_count INTEGER NOT NULL, executed_at TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS saved_queries (id TEXT PRIMARY KEY, connection_id TEXT, folder TEXT NOT NULL, title TEXT NOT NULL, sql_text TEXT NOT NULL, tags TEXT NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS query_bookmarks (id TEXT PRIMARY KEY, connection_id TEXT NOT NULL, title TEXT NOT NULL, sql_text TEXT NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS ui_preferences (key TEXT PRIMARY KEY, value TEXT NOT NULL, updated_at TEXT NOT NULL)`,
	}
	return executeStatements(ctx, transaction, statements)
}

func migrateConnectionMetadata(ctx context.Context, transaction *sql.Tx) error {
	columns := []struct {
		name       string
		definition string
	}{
		{name: "ssl_ca_path", definition: `TEXT NOT NULL DEFAULT ''`},
		{name: "ssl_cert_path", definition: `TEXT NOT NULL DEFAULT ''`},
		{name: "ssl_key_path", definition: `TEXT NOT NULL DEFAULT ''`},
		{name: "proxy_url", definition: `TEXT NOT NULL DEFAULT ''`},
		{name: "proxy_username_cipher", definition: `TEXT NOT NULL DEFAULT ''`},
		{name: "proxy_password_cipher", definition: `TEXT NOT NULL DEFAULT ''`},
		{name: "ssh_enabled", definition: `INTEGER NOT NULL DEFAULT 0`},
		{name: "ssh_host", definition: `TEXT NOT NULL DEFAULT ''`},
		{name: "ssh_port", definition: `INTEGER NOT NULL DEFAULT 0`},
		{name: "ssh_username", definition: `TEXT NOT NULL DEFAULT ''`},
		{name: "ssh_password_cipher", definition: `TEXT NOT NULL DEFAULT ''`},
		{name: "ssh_private_key_path", definition: `TEXT NOT NULL DEFAULT ''`},
		{name: "ssh_known_hosts_path", definition: `TEXT NOT NULL DEFAULT ''`},
		{name: "conn_max_idle_time", definition: `INTEGER NOT NULL DEFAULT 0`},
		{name: "connection_status", definition: `TEXT NOT NULL DEFAULT 'disconnected'`},
		{name: "last_connected_at", definition: `TEXT`},
		{name: "last_disconnected_at", definition: `TEXT`},
		{name: "last_error_code", definition: `TEXT NOT NULL DEFAULT ''`},
	}
	for _, column := range columns {
		if err := addColumnIfMissing(ctx, transaction, "connections", column.name, column.definition); err != nil {
			return err
		}
	}
	return nil
}

func migrateQueryMetadata(ctx context.Context, transaction *sql.Tx) error {
	if err := addColumnIfMissing(ctx, transaction, "query_history", "error_text", `TEXT NOT NULL DEFAULT ''`); err != nil {
		return err
	}
	if err := addColumnIfMissing(ctx, transaction, "saved_queries", "favorite", `INTEGER NOT NULL DEFAULT 0`); err != nil {
		return err
	}
	statements := []string{
		`CREATE TABLE IF NOT EXISTS saved_query_folders (id TEXT PRIMARY KEY, name TEXT NOT NULL UNIQUE, position INTEGER NOT NULL DEFAULT 0, created_at TEXT NOT NULL, updated_at TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS shared_queries (id TEXT PRIMARY KEY, saved_query_id TEXT NOT NULL REFERENCES saved_queries(id) ON DELETE CASCADE, share_code TEXT NOT NULL UNIQUE, created_at TEXT NOT NULL, expires_at TEXT)`,
		`CREATE INDEX IF NOT EXISTS idx_query_history_connection_executed ON query_history(connection_id, executed_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_saved_queries_updated ON saved_queries(favorite DESC, updated_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_saved_queries_folder ON saved_queries(folder)`,
		`CREATE INDEX IF NOT EXISTS idx_shared_queries_saved_query ON shared_queries(saved_query_id)`,
	}
	return executeStatements(ctx, transaction, statements)
}

func executeStatements(ctx context.Context, transaction *sql.Tx, statements []string) error {
	for _, statement := range statements {
		if _, err := transaction.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}

func addColumnIfMissing(ctx context.Context, transaction *sql.Tx, table, column, definition string) error {
	present, err := tableHasColumn(ctx, transaction, table, column)
	if err != nil {
		return err
	}
	if present {
		return nil
	}
	statement := fmt.Sprintf(`ALTER TABLE "%s" ADD COLUMN "%s" %s`, table, column, definition)
	if _, err := transaction.ExecContext(ctx, statement); err != nil {
		return fmt.Errorf("add %s.%s: %w", table, column, err)
	}
	return nil
}

func tableHasColumn(ctx context.Context, transaction *sql.Tx, table, column string) (bool, error) {
	rows, err := transaction.QueryContext(ctx, fmt.Sprintf(`PRAGMA table_info("%s")`, table))
	if err != nil {
		return false, fmt.Errorf("inspect table %s: %w", table, err)
	}
	defer rows.Close()
	for rows.Next() {
		var index int
		var name string
		var dataType string
		var notNull int
		var defaultValue sql.NullString
		var primaryKey int
		if err := rows.Scan(&index, &name, &dataType, &notNull, &defaultValue, &primaryKey); err != nil {
			return false, fmt.Errorf("inspect table %s column: %w", table, err)
		}
		if strings.EqualFold(name, column) {
			return true, nil
		}
	}
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("inspect table %s columns: %w", table, err)
	}
	return false, nil
}
