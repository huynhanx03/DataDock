package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/huynhanx03/datadock/internal/core/dto"
	"time"
)

type SavedQueryService struct{ database *sql.DB }

func NewSavedQueryService(database *sql.DB) *SavedQueryService { return &SavedQueryService{database} }
func (s *SavedQueryService) List(ctx context.Context) ([]dto.SavedQuery, error) {
	rows, err := s.database.QueryContext(ctx, "SELECT id,connection_id,folder,title,sql_text,tags,created_at,updated_at FROM saved_queries ORDER BY updated_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []dto.SavedQuery{}
	for rows.Next() {
		var q dto.SavedQuery
		var id sql.NullString
		var tags string
		if err := rows.Scan(&q.ID, &id, &q.Folder, &q.Title, &q.SQL, &tags, &q.CreatedAt, &q.UpdatedAt); err != nil {
			return nil, err
		}
		if id.Valid {
			q.ConnectionID = &id.String
		}
		_ = json.Unmarshal([]byte(tags), &q.Tags)
		out = append(out, q)
	}
	return out, rows.Err()
}
func (s *SavedQueryService) Create(ctx context.Context, in dto.SavedQueryInput) (dto.SavedQuery, error) {
	if in.Title == "" || in.SQL == "" {
		return dto.SavedQuery{}, errors.New("query title and SQL are required")
	}
	if in.Folder == "" {
		in.Folder = "General"
	}
	tags, _ := json.Marshal(in.Tags)
	now := time.Now().UTC().Format(time.RFC3339)
	q := dto.SavedQuery{ID: uuid.NewString(), ConnectionID: in.ConnectionID, Folder: in.Folder, Title: in.Title, SQL: in.SQL, Tags: in.Tags, CreatedAt: now, UpdatedAt: now}
	_, err := s.database.ExecContext(ctx, "INSERT INTO saved_queries (id,connection_id,folder,title,sql_text,tags,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?)", q.ID, in.ConnectionID, q.Folder, q.Title, q.SQL, string(tags), now, now)
	return q, err
}
func (s *SavedQueryService) Delete(ctx context.Context, id string) error {
	r, err := s.database.ExecContext(ctx, "DELETE FROM saved_queries WHERE id=?", id)
	if err != nil {
		return err
	}
	n, _ := r.RowsAffected()
	if n == 0 {
		return errors.New("saved query not found")
	}
	return nil
}
