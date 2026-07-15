package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/huynhanx03/datadock/internal/core/entity"
	"github.com/huynhanx03/datadock/internal/ports"
)

var ErrNotFound = ports.ErrNotFound

type WorkspaceRepository struct {
	database *sql.DB
}

func NewWorkspaceRepository(database *sql.DB) *WorkspaceRepository {
	return &WorkspaceRepository{database: database}
}

func (repository *WorkspaceRepository) Create(ctx context.Context, workspace entity.Workspace) (entity.Workspace, error) {
	_, err := repository.database.ExecContext(ctx, `INSERT INTO workspaces (id, name, icon, color, position, collapsed, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, workspace.ID, workspace.Name, workspace.Icon, workspace.Color, workspace.Position, workspace.Collapsed, workspace.CreatedAt.Format(timeFormat), workspace.UpdatedAt.Format(timeFormat))
	return workspace, err
}

func (repository *WorkspaceRepository) List(ctx context.Context) ([]entity.Workspace, error) {
	rows, err := repository.database.QueryContext(ctx, `SELECT id, name, icon, color, position, collapsed, created_at, updated_at FROM workspaces ORDER BY position, created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	workspaces := make([]entity.Workspace, 0)
	for rows.Next() {
		workspace, err := scanWorkspace(rows)
		if err != nil {
			return nil, err
		}
		workspaces = append(workspaces, workspace)
	}
	return workspaces, rows.Err()
}

func (repository *WorkspaceRepository) Get(ctx context.Context, id string) (entity.Workspace, error) {
	return scanWorkspace(repository.database.QueryRowContext(ctx, `SELECT id, name, icon, color, position, collapsed, created_at, updated_at FROM workspaces WHERE id = ?`, id))
}

func (repository *WorkspaceRepository) Update(ctx context.Context, workspace entity.Workspace) (entity.Workspace, error) {
	result, err := repository.database.ExecContext(ctx, `UPDATE workspaces SET name = ?, icon = ?, color = ?, collapsed = ?, updated_at = ? WHERE id = ?`, workspace.Name, workspace.Icon, workspace.Color, workspace.Collapsed, workspace.UpdatedAt.Format(timeFormat), workspace.ID)
	if err != nil {
		return entity.Workspace{}, err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return entity.Workspace{}, err
	}
	if changed == 0 {
		return entity.Workspace{}, ErrNotFound
	}
	return workspace, nil
}

func (repository *WorkspaceRepository) Delete(ctx context.Context, id string) error {
	result, err := repository.database.ExecContext(ctx, `DELETE FROM workspaces WHERE id = ?`, id)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed == 0 {
		return ErrNotFound
	}
	return nil
}

func (repository *WorkspaceRepository) Reorder(ctx context.Context, ids []string) error {
	transaction, err := repository.database.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer transaction.Rollback()
	for position, id := range ids {
		result, err := transaction.ExecContext(ctx, `UPDATE workspaces SET position = ? WHERE id = ?`, position, id)
		if err != nil {
			return err
		}
		changed, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if changed == 0 {
			return fmt.Errorf("workspace %s: %w", id, ErrNotFound)
		}
	}
	return transaction.Commit()
}

type workspaceScanner interface {
	Scan(...any) error
}

func scanWorkspace(scanner workspaceScanner) (entity.Workspace, error) {
	var workspace entity.Workspace
	var createdAt string
	var updatedAt string
	err := scanner.Scan(&workspace.ID, &workspace.Name, &workspace.Icon, &workspace.Color, &workspace.Position, &workspace.Collapsed, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return entity.Workspace{}, ErrNotFound
	}
	if err != nil {
		return entity.Workspace{}, err
	}
	workspace.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return entity.Workspace{}, err
	}
	workspace.UpdatedAt, err = parseTime(updatedAt)
	return workspace, err
}
