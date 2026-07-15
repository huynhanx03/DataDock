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

type QueryHistoryRepository struct {
	database *sql.DB
}

func NewQueryHistoryRepository(database *sql.DB) *QueryHistoryRepository {
	return &QueryHistoryRepository{database: database}
}

func (repository *QueryHistoryRepository) Create(ctx context.Context, history ports.QueryHistory) (ports.QueryHistory, error) {
	if history.ID == "" {
		history.ID = uuid.NewString()
	}
	if history.ExecutedAt.IsZero() {
		history.ExecutedAt = time.Now().UTC()
	}
	_, err := repository.database.ExecContext(ctx, `INSERT INTO query_history (id, connection_id, sql_text, status, duration_ms, row_count, error_text, executed_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, history.ID, history.ConnectionID, history.SQLText, history.Status, history.DurationMS, history.RowCount, history.Error, history.ExecutedAt.Format(timeFormat))
	if err != nil {
		return ports.QueryHistory{}, fmt.Errorf("create query history: %w", err)
	}
	return history, nil
}

func (repository *QueryHistoryRepository) List(ctx context.Context, connectionID string) ([]ports.QueryHistory, error) {
	query := `SELECT id, connection_id, sql_text, status, duration_ms, row_count, error_text, executed_at FROM query_history`
	args := make([]any, 0, 1)
	if connectionID != "" {
		query += ` WHERE connection_id = ?`
		args = append(args, connectionID)
	}
	query += ` ORDER BY executed_at DESC, id DESC`
	rows, err := repository.database.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list query history: %w", err)
	}
	defer rows.Close()
	items := make([]ports.QueryHistory, 0)
	for rows.Next() {
		item, err := scanQueryHistory(rows)
		if err != nil {
			return nil, fmt.Errorf("scan query history: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate query history: %w", err)
	}
	return items, nil
}

func (repository *QueryHistoryRepository) DeleteByIDs(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, len(ids))
	for index, id := range ids {
		args[index] = id
	}
	if _, err := repository.database.ExecContext(ctx, `DELETE FROM query_history WHERE id IN (`+placeholders+`)`, args...); err != nil {
		return fmt.Errorf("delete query history: %w", err)
	}
	return nil
}

func (repository *QueryHistoryRepository) ClearByConnection(ctx context.Context, connectionID string) error {
	query := `DELETE FROM query_history`
	args := make([]any, 0, 1)
	if connectionID != "" {
		query += ` WHERE connection_id = ?`
		args = append(args, connectionID)
	}
	if _, err := repository.database.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("clear query history: %w", err)
	}
	return nil
}

type queryHistoryScanner interface {
	Scan(...any) error
}

func scanQueryHistory(scanner queryHistoryScanner) (ports.QueryHistory, error) {
	var history ports.QueryHistory
	var executedAt string
	err := scanner.Scan(&history.ID, &history.ConnectionID, &history.SQLText, &history.Status, &history.DurationMS, &history.RowCount, &history.Error, &executedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ports.QueryHistory{}, ports.ErrNotFound
	}
	if err != nil {
		return ports.QueryHistory{}, err
	}
	history.ExecutedAt, err = parseTime(executedAt)
	if err != nil {
		return ports.QueryHistory{}, err
	}
	return history, nil
}
