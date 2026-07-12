package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

func Open(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, err
	}
	database, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	database.SetMaxOpenConns(1)
	if err := database.PingContext(context.Background()); err != nil {
		return nil, err
	}
	if err := migrate(context.Background(), database); err != nil {
		return nil, err
	}
	return database, nil
}

func migrate(ctx context.Context, database *sql.DB) error {
	statements := []string{
		`PRAGMA foreign_keys = ON`,
		`CREATE TABLE IF NOT EXISTS workspaces (id TEXT PRIMARY KEY, name TEXT NOT NULL, icon TEXT NOT NULL, color TEXT NOT NULL, position INTEGER NOT NULL, collapsed INTEGER NOT NULL DEFAULT 0, created_at TEXT NOT NULL, updated_at TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS connections (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE, name TEXT NOT NULL, engine TEXT NOT NULL, host TEXT NOT NULL, port INTEGER NOT NULL, database_name TEXT NOT NULL, username TEXT NOT NULL, password_cipher TEXT NOT NULL, ssl_mode TEXT NOT NULL, read_only INTEGER NOT NULL DEFAULT 0, auto_reconnect INTEGER NOT NULL DEFAULT 1, max_open_conns INTEGER NOT NULL, max_idle_conns INTEGER NOT NULL, conn_max_lifetime INTEGER NOT NULL, favorite INTEGER NOT NULL DEFAULT 0, created_at TEXT NOT NULL, updated_at TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS query_history (id TEXT PRIMARY KEY, connection_id TEXT NOT NULL, sql_text TEXT NOT NULL, status TEXT NOT NULL, duration_ms INTEGER NOT NULL, row_count INTEGER NOT NULL, executed_at TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS saved_queries (id TEXT PRIMARY KEY, connection_id TEXT, folder TEXT NOT NULL, title TEXT NOT NULL, sql_text TEXT NOT NULL, tags TEXT NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS query_bookmarks (id TEXT PRIMARY KEY, connection_id TEXT NOT NULL, title TEXT NOT NULL, sql_text TEXT NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS ui_preferences (key TEXT PRIMARY KEY, value TEXT NOT NULL, updated_at TEXT NOT NULL)`,
	}
	for _, statement := range statements {
		if _, err := database.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("migrate application database: %w", err)
		}
	}
	for _, statement := range []string{
		`ALTER TABLE connections ADD COLUMN ssl_ca_path TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE connections ADD COLUMN ssl_cert_path TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE connections ADD COLUMN ssl_key_path TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE connections ADD COLUMN proxy_url TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE connections ADD COLUMN ssh_enabled INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE connections ADD COLUMN ssh_host TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE connections ADD COLUMN ssh_port INTEGER NOT NULL DEFAULT 0`,
		`ALTER TABLE connections ADD COLUMN ssh_username TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE connections ADD COLUMN ssh_password_cipher TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE connections ADD COLUMN ssh_private_key_path TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE connections ADD COLUMN ssh_known_hosts_path TEXT NOT NULL DEFAULT ''`,
	} {
		if _, err := database.ExecContext(ctx, statement); err != nil && !strings.Contains(err.Error(), "duplicate column name") {
			return fmt.Errorf("migrate connection profiles: %w", err)
		}
	}
	return nil
}
