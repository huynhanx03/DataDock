package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/huynhanx03/datadock/internal/ports"
)

type SharedQueryRepository struct {
	database *sql.DB
}

func NewSharedQueryRepository(database *sql.DB) *SharedQueryRepository {
	return &SharedQueryRepository{database: database}
}

func (repository *SharedQueryRepository) Create(ctx context.Context, shared ports.SharedQuery) (ports.SharedQuery, error) {
	if shared.ID == "" {
		shared.ID = uuid.NewString()
	}
	if shared.CreatedAt.IsZero() {
		shared.CreatedAt = time.Now().UTC()
	}
	_, err := repository.database.ExecContext(ctx, `INSERT INTO shared_queries (id, saved_query_id, share_code, created_at, expires_at) VALUES (?, ?, ?, ?, ?)`, shared.ID, shared.SavedQueryID, shared.ShareCode, shared.CreatedAt.Format(timeFormat), nullableTime(shared.ExpiresAt))
	if err != nil {
		return ports.SharedQuery{}, fmt.Errorf("create shared query: %w", sqliteRepositoryError(err))
	}
	return shared, nil
}

func (repository *SharedQueryRepository) Get(ctx context.Context, id string) (ports.SharedQuery, error) {
	shared, err := scanSharedQuery(repository.database.QueryRowContext(ctx, `SELECT `+sharedQueryColumns+` FROM shared_queries WHERE id = ?`, id))
	if err != nil {
		return ports.SharedQuery{}, fmt.Errorf("get shared query: %w", err)
	}
	return shared, nil
}

func (repository *SharedQueryRepository) GetByCode(ctx context.Context, code string) (ports.SharedQuery, error) {
	shared, err := scanSharedQuery(repository.database.QueryRowContext(ctx, `SELECT `+sharedQueryColumns+` FROM shared_queries WHERE share_code = ?`, code))
	if err != nil {
		return ports.SharedQuery{}, fmt.Errorf("get shared query by code: %w", err)
	}
	return shared, nil
}

func (repository *SharedQueryRepository) GetBySavedQuery(ctx context.Context, savedQueryID string) (ports.SharedQuery, error) {
	shared, err := scanSharedQuery(repository.database.QueryRowContext(ctx, `SELECT `+sharedQueryColumns+` FROM shared_queries WHERE saved_query_id = ? ORDER BY created_at DESC, id DESC LIMIT 1`, savedQueryID))
	if err != nil {
		return ports.SharedQuery{}, fmt.Errorf("get saved query share: %w", err)
	}
	return shared, nil
}

func (repository *SharedQueryRepository) ListBySavedQuery(ctx context.Context, savedQueryID string) ([]ports.SharedQuery, error) {
	rows, err := repository.database.QueryContext(ctx, `SELECT `+sharedQueryColumns+` FROM shared_queries WHERE saved_query_id = ? ORDER BY created_at DESC, id DESC`, savedQueryID)
	if err != nil {
		return nil, fmt.Errorf("list shared queries: %w", err)
	}
	defer rows.Close()
	items := make([]ports.SharedQuery, 0)
	for rows.Next() {
		shared, err := scanSharedQuery(rows)
		if err != nil {
			return nil, fmt.Errorf("scan shared query: %w", err)
		}
		items = append(items, shared)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate shared queries: %w", err)
	}
	return items, nil
}

func (repository *SharedQueryRepository) Delete(ctx context.Context, id string) error {
	result, err := repository.database.ExecContext(ctx, `DELETE FROM shared_queries WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete shared query: %w", sqliteRepositoryError(err))
	}
	if err := requireChanged(result); err != nil {
		return fmt.Errorf("delete shared query: %w", err)
	}
	return nil
}

func (repository *SharedQueryRepository) DeleteBySavedQuery(ctx context.Context, savedQueryID string) error {
	if _, err := repository.database.ExecContext(ctx, `DELETE FROM shared_queries WHERE saved_query_id = ?`, savedQueryID); err != nil {
		return fmt.Errorf("delete saved query shares: %w", sqliteRepositoryError(err))
	}
	return nil
}

func (repository *SharedQueryRepository) Enable(ctx context.Context, shared ports.SharedQuery) (ports.SharedQuery, error) {
	if shared.ID == "" {
		shared.ID = uuid.NewString()
	}
	if shared.CreatedAt.IsZero() {
		shared.CreatedAt = time.Now().UTC()
	}
	transaction, err := repository.database.BeginTx(ctx, nil)
	if err != nil {
		return ports.SharedQuery{}, fmt.Errorf("begin enable shared query: %w", err)
	}
	defer transaction.Rollback()
	var savedQueryID string
	if err := transaction.QueryRowContext(ctx, `SELECT id FROM saved_queries WHERE id = ?`, shared.SavedQueryID).Scan(&savedQueryID); err != nil {
		return ports.SharedQuery{}, fmt.Errorf("get saved query to share: %w", sqliteRepositoryError(err))
	}
	if _, err := transaction.ExecContext(ctx, `DELETE FROM shared_queries WHERE saved_query_id = ?`, shared.SavedQueryID); err != nil {
		return ports.SharedQuery{}, fmt.Errorf("replace saved query share: %w", sqliteRepositoryError(err))
	}
	if _, err := transaction.ExecContext(ctx, `INSERT INTO shared_queries (id, saved_query_id, share_code, created_at, expires_at) VALUES (?, ?, ?, ?, ?)`, shared.ID, shared.SavedQueryID, shared.ShareCode, shared.CreatedAt.Format(timeFormat), nullableTime(shared.ExpiresAt)); err != nil {
		return ports.SharedQuery{}, fmt.Errorf("enable saved query share: %w", sqliteRepositoryError(err))
	}
	if err := transaction.Commit(); err != nil {
		return ports.SharedQuery{}, fmt.Errorf("commit saved query share: %w", sqliteRepositoryError(err))
	}
	return shared, nil
}

func (repository *SharedQueryRepository) Revoke(ctx context.Context, savedQueryID string) error {
	transaction, err := repository.database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin revoke shared query: %w", err)
	}
	defer transaction.Rollback()
	var existingID string
	if err := transaction.QueryRowContext(ctx, `SELECT id FROM saved_queries WHERE id = ?`, savedQueryID).Scan(&existingID); err != nil {
		return fmt.Errorf("get saved query to revoke: %w", sqliteRepositoryError(err))
	}
	if _, err := transaction.ExecContext(ctx, `DELETE FROM shared_queries WHERE saved_query_id = ?`, savedQueryID); err != nil {
		return fmt.Errorf("revoke saved query share: %w", sqliteRepositoryError(err))
	}
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit saved query share revoke: %w", sqliteRepositoryError(err))
	}
	return nil
}

func (repository *SharedQueryRepository) LookupPublic(ctx context.Context, code string, now time.Time) (ports.PublicSharedQuery, error) {
	shared, err := scanPublicSharedQuery(repository.database.QueryRowContext(ctx, `SELECT queries.title, queries.sql_text, queries.tags, queries.created_at, shares.created_at, shares.expires_at FROM shared_queries shares JOIN saved_queries queries ON queries.id = shares.saved_query_id WHERE shares.share_code = ?`, code))
	if err != nil {
		return ports.PublicSharedQuery{}, fmt.Errorf("get public shared query: %w", sqliteRepositoryError(err))
	}
	if shared.ExpiresAt != nil && !shared.ExpiresAt.After(now) {
		return ports.PublicSharedQuery{}, ports.ErrNotFound
	}
	return shared, nil
}

const sharedQueryColumns = `id, saved_query_id, share_code, created_at, expires_at`

type sharedQueryScanner interface {
	Scan(...any) error
}

func scanSharedQuery(scanner sharedQueryScanner) (ports.SharedQuery, error) {
	var shared ports.SharedQuery
	var createdAt string
	var expiresAt sql.NullString
	err := scanner.Scan(&shared.ID, &shared.SavedQueryID, &shared.ShareCode, &createdAt, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ports.SharedQuery{}, ports.ErrNotFound
	}
	if err != nil {
		return ports.SharedQuery{}, err
	}
	shared.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return ports.SharedQuery{}, err
	}
	if expiresAt.Valid {
		value, parseErr := parseTime(expiresAt.String)
		if parseErr != nil {
			return ports.SharedQuery{}, parseErr
		}
		shared.ExpiresAt = &value
	}
	return shared, nil
}

func nullableTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UTC().Format(timeFormat)
}

func scanPublicSharedQuery(scanner sharedQueryScanner) (ports.PublicSharedQuery, error) {
	var shared ports.PublicSharedQuery
	var tags string
	var queryCreatedAt string
	var sharedAt string
	var expiresAt sql.NullString
	err := scanner.Scan(&shared.Title, &shared.SQLText, &tags, &queryCreatedAt, &sharedAt, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ports.PublicSharedQuery{}, ports.ErrNotFound
	}
	if err != nil {
		return ports.PublicSharedQuery{}, err
	}
	if err := json.Unmarshal([]byte(tags), &shared.Tags); err != nil {
		return ports.PublicSharedQuery{}, fmt.Errorf("decode public shared query tags: %w", err)
	}
	if shared.Tags == nil {
		shared.Tags = []string{}
	}
	shared.QueryCreatedAt, err = parseTime(queryCreatedAt)
	if err != nil {
		return ports.PublicSharedQuery{}, err
	}
	shared.SharedAt, err = parseTime(sharedAt)
	if err != nil {
		return ports.PublicSharedQuery{}, err
	}
	if expiresAt.Valid {
		value, parseErr := parseTime(expiresAt.String)
		if parseErr != nil {
			return ports.PublicSharedQuery{}, parseErr
		}
		shared.ExpiresAt = &value
	}
	return shared, nil
}
