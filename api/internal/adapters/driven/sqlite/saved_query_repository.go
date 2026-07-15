package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/huynhanx03/datadock/internal/ports"
	modernsqlite "modernc.org/sqlite"
)

type SavedQueryRepository struct {
	database *sql.DB
}

func NewSavedQueryRepository(database *sql.DB) *SavedQueryRepository {
	return &SavedQueryRepository{database: database}
}

func (repository *SavedQueryRepository) Create(ctx context.Context, query ports.SavedQuery) (ports.SavedQuery, error) {
	query = normalizeSavedQuery(query)
	tags, err := json.Marshal(query.Tags)
	if err != nil {
		return ports.SavedQuery{}, fmt.Errorf("encode saved query tags: %w", err)
	}
	_, err = repository.database.ExecContext(ctx, `INSERT INTO saved_queries (id, connection_id, folder, title, sql_text, tags, favorite, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, query.ID, nullableString(query.ConnectionID), query.Folder, query.Title, query.SQLText, string(tags), query.Favorite, query.CreatedAt.Format(timeFormat), query.UpdatedAt.Format(timeFormat))
	if err != nil {
		return ports.SavedQuery{}, fmt.Errorf("create saved query: %w", sqliteRepositoryError(err))
	}
	return query, nil
}

func (repository *SavedQueryRepository) Get(ctx context.Context, id string) (ports.SavedQuery, error) {
	query, err := scanSavedQuery(repository.database.QueryRowContext(ctx, `SELECT `+savedQueryColumns+` FROM saved_queries WHERE id = ?`, id))
	if err != nil {
		return ports.SavedQuery{}, fmt.Errorf("get saved query: %w", err)
	}
	return query, nil
}

func (repository *SavedQueryRepository) List(ctx context.Context) ([]ports.SavedQuery, error) {
	rows, err := repository.database.QueryContext(ctx, `SELECT `+savedQueryColumns+` FROM saved_queries ORDER BY favorite DESC, updated_at DESC, title, id`)
	if err != nil {
		return nil, fmt.Errorf("list saved queries: %w", err)
	}
	defer rows.Close()
	queries := make([]ports.SavedQuery, 0)
	for rows.Next() {
		query, err := scanSavedQuery(rows)
		if err != nil {
			return nil, fmt.Errorf("scan saved query: %w", err)
		}
		queries = append(queries, query)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate saved queries: %w", err)
	}
	return queries, nil
}

func (repository *SavedQueryRepository) Update(ctx context.Context, query ports.SavedQuery) (ports.SavedQuery, error) {
	if query.Folder == "" {
		query.Folder = "General"
	}
	if query.UpdatedAt.IsZero() {
		query.UpdatedAt = time.Now().UTC()
	}
	if query.Tags == nil {
		query.Tags = []string{}
	}
	tags, err := json.Marshal(query.Tags)
	if err != nil {
		return ports.SavedQuery{}, fmt.Errorf("encode saved query tags: %w", err)
	}
	result, err := repository.database.ExecContext(ctx, `UPDATE saved_queries SET connection_id = ?, folder = ?, title = ?, sql_text = ?, tags = ?, favorite = ?, updated_at = ? WHERE id = ?`, nullableString(query.ConnectionID), query.Folder, query.Title, query.SQLText, string(tags), query.Favorite, query.UpdatedAt.Format(timeFormat), query.ID)
	if err != nil {
		return ports.SavedQuery{}, fmt.Errorf("update saved query: %w", sqliteRepositoryError(err))
	}
	if err := requireChanged(result); err != nil {
		return ports.SavedQuery{}, fmt.Errorf("update saved query: %w", err)
	}
	return query, nil
}

func (repository *SavedQueryRepository) Patch(ctx context.Context, id string, patch ports.SavedQueryPatch) (ports.SavedQuery, error) {
	assignments := make([]string, 0, 6)
	arguments := make([]any, 0, 7)
	if patch.ConnectionIDSet {
		assignments = append(assignments, `connection_id = ?`)
		arguments = append(arguments, nullableString(patch.ConnectionID))
	}
	if patch.Folder != nil {
		assignments = append(assignments, `folder = ?`)
		arguments = append(arguments, *patch.Folder)
	}
	if patch.Title != nil {
		assignments = append(assignments, `title = ?`)
		arguments = append(arguments, *patch.Title)
	}
	if patch.SQLText != nil {
		assignments = append(assignments, `sql_text = ?`)
		arguments = append(arguments, *patch.SQLText)
	}
	if patch.Tags != nil {
		tags := append([]string(nil), (*patch.Tags)...)
		if tags == nil {
			tags = []string{}
		}
		encoded, err := json.Marshal(tags)
		if err != nil {
			return ports.SavedQuery{}, fmt.Errorf("encode saved query tags: %w", err)
		}
		assignments = append(assignments, `tags = ?`)
		arguments = append(arguments, string(encoded))
	}
	updatedAt := patch.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}
	assignments = append(assignments, `updated_at = ?`)
	arguments = append(arguments, updatedAt.Format(timeFormat), id)
	statement := `UPDATE saved_queries SET ` + strings.Join(assignments, `, `) + ` WHERE id = ? RETURNING ` + savedQueryColumns
	query, err := scanSavedQuery(repository.database.QueryRowContext(ctx, statement, arguments...))
	if err != nil {
		return ports.SavedQuery{}, fmt.Errorf("patch saved query: %w", sqliteRepositoryError(err))
	}
	return query, nil
}

func (repository *SavedQueryRepository) Delete(ctx context.Context, id string) error {
	result, err := repository.database.ExecContext(ctx, `DELETE FROM saved_queries WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete saved query: %w", sqliteRepositoryError(err))
	}
	if err := requireChanged(result); err != nil {
		return fmt.Errorf("delete saved query: %w", err)
	}
	return nil
}

func (repository *SavedQueryRepository) Duplicate(ctx context.Context, id string) (ports.SavedQuery, error) {
	return repository.DuplicateWithUniqueTitle(ctx, id, func(source string, _ []string) string {
		return source + " copy"
	})
}

func (repository *SavedQueryRepository) DuplicateWithUniqueTitle(ctx context.Context, id string, selectTitle ports.SavedQueryTitleSelector) (ports.SavedQuery, error) {
	if selectTitle == nil {
		return ports.SavedQuery{}, errors.New("saved query title selector is required")
	}
	transaction, err := repository.database.BeginTx(ctx, nil)
	if err != nil {
		return ports.SavedQuery{}, fmt.Errorf("begin duplicate saved query: %w", err)
	}
	defer transaction.Rollback()
	query, err := scanSavedQuery(transaction.QueryRowContext(ctx, `SELECT `+savedQueryColumns+` FROM saved_queries WHERE id = ?`, id))
	if err != nil {
		return ports.SavedQuery{}, fmt.Errorf("get saved query to duplicate: %w", sqliteRepositoryError(err))
	}
	titleRows, err := transaction.QueryContext(ctx, `SELECT title FROM saved_queries ORDER BY title COLLATE NOCASE, id`)
	if err != nil {
		return ports.SavedQuery{}, fmt.Errorf("list saved query titles: %w", sqliteRepositoryError(err))
	}
	titles := make([]string, 0)
	for titleRows.Next() {
		var title string
		if err := titleRows.Scan(&title); err != nil {
			titleRows.Close()
			return ports.SavedQuery{}, fmt.Errorf("scan saved query title: %w", sqliteRepositoryError(err))
		}
		titles = append(titles, title)
	}
	if err := titleRows.Err(); err != nil {
		titleRows.Close()
		return ports.SavedQuery{}, fmt.Errorf("iterate saved query titles: %w", sqliteRepositoryError(err))
	}
	if err := titleRows.Close(); err != nil {
		return ports.SavedQuery{}, fmt.Errorf("close saved query titles: %w", err)
	}
	now := time.Now().UTC()
	query.ID = uuid.NewString()
	query.Title = selectTitle(query.Title, titles)
	query.Favorite = false
	query.CreatedAt = now
	query.UpdatedAt = now
	tags, err := json.Marshal(query.Tags)
	if err != nil {
		return ports.SavedQuery{}, fmt.Errorf("encode duplicated saved query tags: %w", err)
	}
	_, err = transaction.ExecContext(ctx, `INSERT INTO saved_queries (id, connection_id, folder, title, sql_text, tags, favorite, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, query.ID, nullableString(query.ConnectionID), query.Folder, query.Title, query.SQLText, string(tags), query.Favorite, query.CreatedAt.Format(timeFormat), query.UpdatedAt.Format(timeFormat))
	if err != nil {
		return ports.SavedQuery{}, fmt.Errorf("duplicate saved query: %w", sqliteRepositoryError(err))
	}
	if err := transaction.Commit(); err != nil {
		return ports.SavedQuery{}, fmt.Errorf("commit duplicate saved query: %w", sqliteRepositoryError(err))
	}
	return query, nil
}

func (repository *SavedQueryRepository) SetFavorite(ctx context.Context, id string, favorite bool) error {
	_, err := repository.SetFavoriteAndGet(ctx, id, favorite, time.Now().UTC())
	return err
}

func (repository *SavedQueryRepository) SetFavoriteAndGet(ctx context.Context, id string, favorite bool, updatedAt time.Time) (ports.SavedQuery, error) {
	if updatedAt.IsZero() {
		updatedAt = time.Now().UTC()
	}
	query, err := scanSavedQuery(repository.database.QueryRowContext(ctx, `UPDATE saved_queries SET favorite = ?, updated_at = ? WHERE id = ? RETURNING `+savedQueryColumns, favorite, updatedAt.Format(timeFormat), id))
	if err != nil {
		return ports.SavedQuery{}, fmt.Errorf("set saved query favorite: %w", sqliteRepositoryError(err))
	}
	return query, nil
}

const savedQueryColumns = `id, connection_id, folder, title, sql_text, tags, favorite, created_at, updated_at`

type savedQueryScanner interface {
	Scan(...any) error
}

func scanSavedQuery(scanner savedQueryScanner) (ports.SavedQuery, error) {
	var query ports.SavedQuery
	var connectionID sql.NullString
	var tags string
	var createdAt string
	var updatedAt string
	err := scanner.Scan(&query.ID, &connectionID, &query.Folder, &query.Title, &query.SQLText, &tags, &query.Favorite, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ports.SavedQuery{}, ports.ErrNotFound
	}
	if err != nil {
		return ports.SavedQuery{}, err
	}
	if connectionID.Valid {
		query.ConnectionID = &connectionID.String
	}
	if err := json.Unmarshal([]byte(tags), &query.Tags); err != nil {
		return ports.SavedQuery{}, fmt.Errorf("decode saved query tags: %w", err)
	}
	if query.Tags == nil {
		query.Tags = []string{}
	}
	query.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return ports.SavedQuery{}, err
	}
	query.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return ports.SavedQuery{}, err
	}
	return query, nil
}

func normalizeSavedQuery(query ports.SavedQuery) ports.SavedQuery {
	now := time.Now().UTC()
	if query.ID == "" {
		query.ID = uuid.NewString()
	}
	if query.Folder == "" {
		query.Folder = "General"
	}
	if query.Tags == nil {
		query.Tags = []string{}
	}
	if query.CreatedAt.IsZero() {
		query.CreatedAt = now
	}
	if query.UpdatedAt.IsZero() {
		query.UpdatedAt = query.CreatedAt
	}
	return query
}

func nullableString(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}

func requireChanged(result sql.Result) error {
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed == 0 {
		return ports.ErrNotFound
	}
	return nil
}

func sqliteRepositoryError(err error) error {
	if err == nil || errors.Is(err, ports.ErrNotFound) || errors.Is(err, ports.ErrConflict) {
		return err
	}
	if errors.Is(err, sql.ErrNoRows) {
		return ports.ErrNotFound
	}
	var sqliteError *modernsqlite.Error
	if errors.As(err, &sqliteError) && sqliteError.Code()&0xff == 19 {
		return fmt.Errorf("%w: %v", ports.ErrConflict, err)
	}
	return err
}
