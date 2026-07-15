package ports

import (
	"context"
	"time"
)

type QueryHistory struct {
	ID           string
	ConnectionID string
	SQLText      string
	Status       string
	DurationMS   int64
	RowCount     int64
	Error        string
	ExecutedAt   time.Time
}

type QueryHistoryRepository interface {
	Create(context.Context, QueryHistory) (QueryHistory, error)
	List(context.Context, string) ([]QueryHistory, error)
	DeleteByIDs(context.Context, []string) error
	ClearByConnection(context.Context, string) error
}
