package service_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/huynhanx03/datadock/internal/adapters/driven/sqlite"
	"github.com/huynhanx03/datadock/internal/ports"
	_ "modernc.org/sqlite"
)

func TestMetadataMigrationsPreserveLegacyRowsAcrossReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metadata.db")
	legacy, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open legacy database: %v", err)
	}
	statements := []string{
		`CREATE TABLE workspaces (id TEXT PRIMARY KEY, name TEXT NOT NULL, icon TEXT NOT NULL, color TEXT NOT NULL, position INTEGER NOT NULL, collapsed INTEGER NOT NULL DEFAULT 0, created_at TEXT NOT NULL, updated_at TEXT NOT NULL)`,
		`CREATE TABLE connections (id TEXT PRIMARY KEY, workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE, name TEXT NOT NULL, engine TEXT NOT NULL, host TEXT NOT NULL, port INTEGER NOT NULL, database_name TEXT NOT NULL, username TEXT NOT NULL, password_cipher TEXT NOT NULL, ssl_mode TEXT NOT NULL, read_only INTEGER NOT NULL DEFAULT 0, auto_reconnect INTEGER NOT NULL DEFAULT 1, max_open_conns INTEGER NOT NULL, max_idle_conns INTEGER NOT NULL, conn_max_lifetime INTEGER NOT NULL, favorite INTEGER NOT NULL DEFAULT 0, created_at TEXT NOT NULL, updated_at TEXT NOT NULL)`,
		`CREATE TABLE saved_queries (id TEXT PRIMARY KEY, connection_id TEXT, folder TEXT NOT NULL, title TEXT NOT NULL, sql_text TEXT NOT NULL, tags TEXT NOT NULL, created_at TEXT NOT NULL, updated_at TEXT NOT NULL)`,
		`INSERT INTO workspaces VALUES ('workspace-1', 'Default', '', '', 0, 0, '2026-07-15T00:00:00Z', '2026-07-15T00:00:00Z')`,
		`INSERT INTO connections (id, workspace_id, name, engine, host, port, database_name, username, password_cipher, ssl_mode, read_only, auto_reconnect, max_open_conns, max_idle_conns, conn_max_lifetime, favorite, created_at, updated_at) VALUES ('connection-1', 'workspace-1', 'Primary', 'postgresql', 'localhost', 5432, 'app', 'app', 'ciphertext', 'disable', 0, 1, 10, 5, 1800, 1, '2026-07-15T00:00:00Z', '2026-07-15T00:00:00Z')`,
		`INSERT INTO saved_queries VALUES ('saved-legacy', 'connection-1', 'General', 'Legacy', 'SELECT 1', '["legacy"]', '2026-07-15T00:00:00Z', '2026-07-15T00:00:00Z')`,
	}
	for _, statement := range statements {
		if _, err := legacy.Exec(statement); err != nil {
			legacy.Close()
			t.Fatalf("prepare legacy database: %v", err)
		}
	}
	if err := legacy.Close(); err != nil {
		t.Fatalf("close legacy database: %v", err)
	}

	database, err := sqlite.Open(path)
	if err != nil {
		t.Fatalf("Open() first migration: %v", err)
	}
	var name string
	var idleTime int
	if err := database.QueryRow(`SELECT name, conn_max_idle_time FROM connections WHERE id = 'connection-1'`).Scan(&name, &idleTime); err != nil {
		database.Close()
		t.Fatalf("read migrated connection: %v", err)
	}
	if name != "Primary" || idleTime != 0 {
		database.Close()
		t.Fatalf("migrated connection = %q, %d", name, idleTime)
	}
	var favorite bool
	if err := database.QueryRow(`SELECT favorite FROM saved_queries WHERE id = 'saved-legacy'`).Scan(&favorite); err != nil {
		database.Close()
		t.Fatalf("read migrated saved query: %v", err)
	}
	if favorite {
		database.Close()
		t.Fatal("legacy saved query unexpectedly became a favorite")
	}
	if err := database.Close(); err != nil {
		t.Fatalf("close migrated database: %v", err)
	}

	reopened, err := sqlite.Open(path)
	if err != nil {
		t.Fatalf("Open() after migration: %v", err)
	}
	defer reopened.Close()
	var applied, distinct int
	if err := reopened.QueryRow(`SELECT COUNT(*), COUNT(DISTINCT version) FROM schema_migrations`).Scan(&applied, &distinct); err != nil {
		t.Fatalf("read migration versions: %v", err)
	}
	if applied == 0 || applied != distinct {
		t.Fatalf("migration versions = %d applied, %d distinct", applied, distinct)
	}
}

func TestMetadataRepositoriesPersistCRUDAndReturnPortErrors(t *testing.T) {
	database, err := sqlite.Open(filepath.Join(t.TempDir(), "repositories.db"))
	if err != nil {
		t.Fatalf("Open(): %v", err)
	}
	defer database.Close()
	ctx := context.Background()
	now := time.Date(2026, time.July, 15, 8, 0, 0, 0, time.UTC)

	historyRepository := sqlite.NewQueryHistoryRepository(database)
	history := ports.QueryHistory{ID: "history-1", ConnectionID: "connection-1", SQLText: "SELECT 1", Status: "success", DurationMS: 12, RowCount: 1, ExecutedAt: now}
	if _, err := historyRepository.Create(ctx, history); err != nil {
		t.Fatalf("create history: %v", err)
	}
	historyItems, err := historyRepository.List(ctx, "connection-1")
	if err != nil || len(historyItems) != 1 || historyItems[0].SQLText != "SELECT 1" {
		t.Fatalf("list history = %#v, %v", historyItems, err)
	}
	if err := historyRepository.DeleteByIDs(ctx, []string{"history-1"}); err != nil {
		t.Fatalf("delete history: %v", err)
	}
	if err := historyRepository.ClearByConnection(ctx, "connection-1"); err != nil {
		t.Fatalf("clear history: %v", err)
	}

	savedRepository := sqlite.NewSavedQueryRepository(database)
	saved := ports.SavedQuery{ID: "saved-1", Folder: "General", Title: "Accounts", SQLText: "SELECT * FROM accounts", Tags: []string{"accounts"}, CreatedAt: now, UpdatedAt: now}
	if _, err := savedRepository.Create(ctx, saved); err != nil {
		t.Fatalf("create saved query: %v", err)
	}
	saved.Favorite = true
	saved.Title = "Active accounts"
	if _, err := savedRepository.Update(ctx, saved); err != nil {
		t.Fatalf("update saved query: %v", err)
	}
	if err := savedRepository.SetFavorite(ctx, saved.ID, true); err != nil {
		t.Fatalf("favorite saved query: %v", err)
	}
	duplicate, err := savedRepository.Duplicate(ctx, saved.ID)
	if err != nil || duplicate.ID == saved.ID || duplicate.Title != "Active accounts copy" {
		t.Fatalf("duplicate saved query = %#v, %v", duplicate, err)
	}
	queries, err := savedRepository.List(ctx)
	if err != nil || len(queries) != 2 {
		t.Fatalf("list saved queries = %#v, %v", queries, err)
	}
	if err := savedRepository.Delete(ctx, "missing"); !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("delete missing saved query error = %v", err)
	}

	folderRepository := sqlite.NewSavedQueryFolderRepository(database)
	folder := ports.SavedQueryFolder{ID: "folder-1", Name: "Analytics", Position: 1, CreatedAt: now, UpdatedAt: now}
	if _, err := folderRepository.Create(ctx, folder); err != nil {
		t.Fatalf("create folder: %v", err)
	}
	folder.Name = "Reporting"
	if _, err := folderRepository.Update(ctx, folder); err != nil {
		t.Fatalf("update folder: %v", err)
	}
	folders, err := folderRepository.List(ctx)
	if err != nil || len(folders) != 1 || folders[0].Name != "Reporting" {
		t.Fatalf("list folders = %#v, %v", folders, err)
	}

	shareRepository := sqlite.NewSharedQueryRepository(database)
	share := ports.SharedQuery{ID: "share-1", SavedQueryID: saved.ID, ShareCode: "public-token", CreatedAt: now}
	if _, err := shareRepository.Create(ctx, share); err != nil {
		t.Fatalf("create share: %v", err)
	}
	shared, err := shareRepository.GetByCode(ctx, share.ShareCode)
	if err != nil || shared.SavedQueryID != saved.ID {
		t.Fatalf("get share = %#v, %v", shared, err)
	}
	if err := shareRepository.Delete(ctx, share.ID); err != nil {
		t.Fatalf("delete share: %v", err)
	}
}
