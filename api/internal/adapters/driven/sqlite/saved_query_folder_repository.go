package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/huynhanx03/datadock/internal/ports"
)

type SavedQueryFolderRepository struct {
	database *sql.DB
}

func NewSavedQueryFolderRepository(database *sql.DB) *SavedQueryFolderRepository {
	return &SavedQueryFolderRepository{database: database}
}

func (repository *SavedQueryFolderRepository) Create(ctx context.Context, folder ports.SavedQueryFolder) (ports.SavedQueryFolder, error) {
	now := time.Now().UTC()
	if folder.ID == "" {
		folder.ID = uuid.NewString()
	}
	if folder.CreatedAt.IsZero() {
		folder.CreatedAt = now
	}
	if folder.UpdatedAt.IsZero() {
		folder.UpdatedAt = folder.CreatedAt
	}
	transaction, err := repository.database.BeginTx(ctx, nil)
	if err != nil {
		return ports.SavedQueryFolder{}, fmt.Errorf("begin create saved query folder: %w", err)
	}
	defer transaction.Rollback()
	var duplicates int
	if err := transaction.QueryRowContext(ctx, `SELECT COUNT(*) FROM saved_query_folders WHERE name = ? COLLATE NOCASE`, folder.Name).Scan(&duplicates); err != nil {
		return ports.SavedQueryFolder{}, fmt.Errorf("check saved query folder name: %w", sqliteRepositoryError(err))
	}
	if duplicates != 0 {
		return ports.SavedQueryFolder{}, ports.ErrConflict
	}
	_, err = transaction.ExecContext(ctx, `INSERT INTO saved_query_folders (id, name, position, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`, folder.ID, folder.Name, folder.Position, folder.CreatedAt.Format(timeFormat), folder.UpdatedAt.Format(timeFormat))
	if err != nil {
		return ports.SavedQueryFolder{}, fmt.Errorf("create saved query folder: %w", sqliteRepositoryError(err))
	}
	if err := transaction.Commit(); err != nil {
		return ports.SavedQueryFolder{}, fmt.Errorf("commit create saved query folder: %w", sqliteRepositoryError(err))
	}
	return folder, nil
}

func (repository *SavedQueryFolderRepository) Ensure(ctx context.Context, folder ports.SavedQueryFolder) (ports.SavedQueryFolder, error) {
	now := time.Now().UTC()
	if folder.ID == "" {
		folder.ID = uuid.NewString()
	}
	if folder.CreatedAt.IsZero() {
		folder.CreatedAt = now
	}
	if folder.UpdatedAt.IsZero() {
		folder.UpdatedAt = folder.CreatedAt
	}
	transaction, err := repository.database.BeginTx(ctx, nil)
	if err != nil {
		return ports.SavedQueryFolder{}, fmt.Errorf("begin ensure saved query folder: %w", err)
	}
	defer transaction.Rollback()
	_, err = transaction.ExecContext(ctx, `INSERT INTO saved_query_folders (id, name, position, created_at, updated_at) SELECT ?, ?, COALESCE((SELECT MAX(position) + 1 FROM saved_query_folders), 0), ?, ? WHERE NOT EXISTS (SELECT 1 FROM saved_query_folders WHERE name = ? COLLATE NOCASE) ON CONFLICT(name) DO NOTHING`, folder.ID, folder.Name, folder.CreatedAt.Format(timeFormat), folder.UpdatedAt.Format(timeFormat), folder.Name)
	if err != nil {
		return ports.SavedQueryFolder{}, fmt.Errorf("ensure saved query folder: %w", sqliteRepositoryError(err))
	}
	persisted, err := scanSavedQueryFolder(transaction.QueryRowContext(ctx, savedQueryFolderSelect+` WHERE folders.name = ? COLLATE NOCASE LIMIT 1`, folder.Name))
	if err != nil {
		return ports.SavedQueryFolder{}, fmt.Errorf("get ensured saved query folder: %w", sqliteRepositoryError(err))
	}
	if err := transaction.Commit(); err != nil {
		return ports.SavedQueryFolder{}, fmt.Errorf("commit ensured saved query folder: %w", sqliteRepositoryError(err))
	}
	return persisted, nil
}

func (repository *SavedQueryFolderRepository) Get(ctx context.Context, id string) (ports.SavedQueryFolder, error) {
	folder, err := scanSavedQueryFolder(repository.database.QueryRowContext(ctx, savedQueryFolderSelect+` WHERE folders.id = ?`, id))
	if err != nil {
		return ports.SavedQueryFolder{}, fmt.Errorf("get saved query folder: %w", err)
	}
	return folder, nil
}

func (repository *SavedQueryFolderRepository) List(ctx context.Context) ([]ports.SavedQueryFolder, error) {
	rows, err := repository.database.QueryContext(ctx, savedQueryFolderSelect+` ORDER BY folders.position, folders.name, folders.id`)
	if err != nil {
		return nil, fmt.Errorf("list saved query folders: %w", err)
	}
	defer rows.Close()
	folders := make([]ports.SavedQueryFolder, 0)
	for rows.Next() {
		folder, err := scanSavedQueryFolder(rows)
		if err != nil {
			return nil, fmt.Errorf("scan saved query folder: %w", err)
		}
		folders = append(folders, folder)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate saved query folders: %w", err)
	}
	return folders, nil
}

func (repository *SavedQueryFolderRepository) Update(ctx context.Context, folder ports.SavedQueryFolder) (ports.SavedQueryFolder, error) {
	name := folder.Name
	position := folder.Position
	return repository.UpdateAndPropagate(ctx, folder.ID, ports.SavedQueryFolderPatch{Name: &name, Position: &position, UpdatedAt: folder.UpdatedAt})
}

func (repository *SavedQueryFolderRepository) Delete(ctx context.Context, id string) error {
	return repository.DeleteWithPolicy(ctx, id, ports.SavedQueryFolderDeletePolicy{Mode: ports.SavedQueryFolderDeleteReject, UpdatedAt: time.Now().UTC()})
}

func (repository *SavedQueryFolderRepository) UpdateAndPropagate(ctx context.Context, id string, patch ports.SavedQueryFolderPatch) (ports.SavedQueryFolder, error) {
	transaction, err := repository.database.BeginTx(ctx, nil)
	if err != nil {
		return ports.SavedQueryFolder{}, fmt.Errorf("begin update saved query folder: %w", err)
	}
	defer transaction.Rollback()
	folder, err := scanSavedQueryFolder(transaction.QueryRowContext(ctx, savedQueryFolderSelect+` WHERE folders.id = ?`, id))
	if err != nil {
		return ports.SavedQueryFolder{}, fmt.Errorf("get saved query folder to update: %w", sqliteRepositoryError(err))
	}
	oldName := folder.Name
	if patch.Name != nil {
		folder.Name = *patch.Name
	}
	if patch.Position != nil {
		folder.Position = *patch.Position
	}
	folder.UpdatedAt = patch.UpdatedAt
	if folder.UpdatedAt.IsZero() {
		folder.UpdatedAt = time.Now().UTC()
	}
	var duplicates int
	if err := transaction.QueryRowContext(ctx, `SELECT COUNT(*) FROM saved_query_folders WHERE id <> ? AND name = ? COLLATE NOCASE`, id, folder.Name).Scan(&duplicates); err != nil {
		return ports.SavedQueryFolder{}, fmt.Errorf("check saved query folder name: %w", sqliteRepositoryError(err))
	}
	if duplicates != 0 {
		return ports.SavedQueryFolder{}, ports.ErrConflict
	}
	result, err := transaction.ExecContext(ctx, `UPDATE saved_query_folders SET name = ?, position = ?, updated_at = ? WHERE id = ?`, folder.Name, folder.Position, folder.UpdatedAt.Format(timeFormat), id)
	if err != nil {
		return ports.SavedQueryFolder{}, fmt.Errorf("update saved query folder: %w", sqliteRepositoryError(err))
	}
	if err := requireChanged(result); err != nil {
		return ports.SavedQueryFolder{}, fmt.Errorf("update saved query folder: %w", err)
	}
	if oldName != folder.Name {
		if _, err := transaction.ExecContext(ctx, `UPDATE saved_queries SET folder = ?, updated_at = ? WHERE folder = ?`, folder.Name, folder.UpdatedAt.Format(timeFormat), oldName); err != nil {
			return ports.SavedQueryFolder{}, fmt.Errorf("propagate saved query folder rename: %w", sqliteRepositoryError(err))
		}
	}
	folder, err = scanSavedQueryFolder(transaction.QueryRowContext(ctx, savedQueryFolderSelect+` WHERE folders.id = ?`, id))
	if err != nil {
		return ports.SavedQueryFolder{}, fmt.Errorf("get updated saved query folder: %w", sqliteRepositoryError(err))
	}
	if err := transaction.Commit(); err != nil {
		return ports.SavedQueryFolder{}, fmt.Errorf("commit saved query folder update: %w", sqliteRepositoryError(err))
	}
	return folder, nil
}

func (repository *SavedQueryFolderRepository) DeleteWithPolicy(ctx context.Context, id string, policy ports.SavedQueryFolderDeletePolicy) error {
	transaction, err := repository.database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin delete saved query folder: %w", err)
	}
	defer transaction.Rollback()
	folder, err := scanSavedQueryFolder(transaction.QueryRowContext(ctx, savedQueryFolderSelect+` WHERE folders.id = ?`, id))
	if err != nil {
		return fmt.Errorf("get saved query folder to delete: %w", sqliteRepositoryError(err))
	}
	updatedAt := policy.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}
	switch policy.Mode {
	case ports.SavedQueryFolderDeleteReject:
		if folder.QueryCount != 0 {
			return ports.ErrConflict
		}
	case ports.SavedQueryFolderDeleteMove:
		destination := strings.TrimSpace(policy.Destination)
		if destination == "" || strings.EqualFold(destination, "General") {
			destination = "General"
		} else {
			canonical, lookupErr := savedQueryFolderName(ctx, transaction, destination)
			if lookupErr != nil {
				return fmt.Errorf("get destination saved query folder: %w", sqliteRepositoryError(lookupErr))
			}
			destination = canonical
		}
		if strings.EqualFold(destination, folder.Name) {
			return ports.ErrConflict
		}
		if _, err := transaction.ExecContext(ctx, `UPDATE saved_queries SET folder = ?, updated_at = ? WHERE folder = ?`, destination, updatedAt.Format(timeFormat), folder.Name); err != nil {
			return fmt.Errorf("move saved queries before folder delete: %w", sqliteRepositoryError(err))
		}
	default:
		return fmt.Errorf("unsupported saved query folder delete mode %q", policy.Mode)
	}
	result, err := transaction.ExecContext(ctx, `DELETE FROM saved_query_folders WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete saved query folder: %w", sqliteRepositoryError(err))
	}
	if err := requireChanged(result); err != nil {
		return fmt.Errorf("delete saved query folder: %w", err)
	}
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit saved query folder delete: %w", sqliteRepositoryError(err))
	}
	return nil
}

const savedQueryFolderSelect = `SELECT folders.id, folders.name, folders.position, (SELECT COUNT(*) FROM saved_queries queries WHERE queries.folder = folders.name), folders.created_at, folders.updated_at FROM saved_query_folders folders`

type savedQueryFolderScanner interface {
	Scan(...any) error
}

func scanSavedQueryFolder(scanner savedQueryFolderScanner) (ports.SavedQueryFolder, error) {
	var folder ports.SavedQueryFolder
	var createdAt string
	var updatedAt string
	err := scanner.Scan(&folder.ID, &folder.Name, &folder.Position, &folder.QueryCount, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ports.SavedQueryFolder{}, ports.ErrNotFound
	}
	if err != nil {
		return ports.SavedQueryFolder{}, err
	}
	folder.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return ports.SavedQueryFolder{}, err
	}
	folder.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return ports.SavedQueryFolder{}, err
	}
	return folder, nil
}

func savedQueryFolderName(ctx context.Context, transaction *sql.Tx, name string) (string, error) {
	var canonical string
	err := transaction.QueryRowContext(ctx, `SELECT name FROM saved_query_folders WHERE name = ? COLLATE NOCASE LIMIT 1`, name).Scan(&canonical)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ports.ErrNotFound
	}
	return canonical, err
}
