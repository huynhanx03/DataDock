package ports

import (
	"context"
	"errors"
	"time"
)

var ErrConflict = errors.New("repository record conflict")

type SavedQuery struct {
	ID           string
	ConnectionID *string
	Folder       string
	Title        string
	SQLText      string
	Tags         []string
	Favorite     bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type SavedQueryRepository interface {
	Create(context.Context, SavedQuery) (SavedQuery, error)
	Get(context.Context, string) (SavedQuery, error)
	List(context.Context) ([]SavedQuery, error)
	Patch(context.Context, string, SavedQueryPatch) (SavedQuery, error)
	Delete(context.Context, string) error
	DuplicateWithUniqueTitle(context.Context, string, SavedQueryTitleSelector) (SavedQuery, error)
	SetFavoriteAndGet(context.Context, string, bool, time.Time) (SavedQuery, error)
}

type SavedQueryPatch struct {
	ConnectionIDSet bool
	ConnectionID    *string
	Folder          *string
	Title           *string
	SQLText         *string
	Tags            *[]string
	UpdatedAt       time.Time
}

type SavedQueryTitleSelector func(string, []string) string

type SavedQueryFolder struct {
	ID         string
	Name       string
	Position   int
	QueryCount int
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type SavedQueryFolderRepository interface {
	Create(context.Context, SavedQueryFolder) (SavedQueryFolder, error)
	Ensure(context.Context, SavedQueryFolder) (SavedQueryFolder, error)
	Get(context.Context, string) (SavedQueryFolder, error)
	List(context.Context) ([]SavedQueryFolder, error)
	UpdateAndPropagate(context.Context, string, SavedQueryFolderPatch) (SavedQueryFolder, error)
	DeleteWithPolicy(context.Context, string, SavedQueryFolderDeletePolicy) error
}

type SavedQueryFolderPatch struct {
	Name      *string
	Position  *int
	UpdatedAt time.Time
}

type SavedQueryFolderDeleteMode string

const (
	SavedQueryFolderDeleteReject SavedQueryFolderDeleteMode = "reject"
	SavedQueryFolderDeleteMove   SavedQueryFolderDeleteMode = "move"
)

type SavedQueryFolderDeletePolicy struct {
	Mode        SavedQueryFolderDeleteMode
	Destination string
	UpdatedAt   time.Time
}

type SharedQuery struct {
	ID           string
	SavedQueryID string
	ShareCode    string
	CreatedAt    time.Time
	ExpiresAt    *time.Time
}

type SharedQueryRepository interface {
	Enable(context.Context, SharedQuery) (SharedQuery, error)
	GetBySavedQuery(context.Context, string) (SharedQuery, error)
	Revoke(context.Context, string) error
	LookupPublic(context.Context, string, time.Time) (PublicSharedQuery, error)
}

type PublicSharedQuery struct {
	Title          string
	SQLText        string
	Tags           []string
	QueryCreatedAt time.Time
	SharedAt       time.Time
	ExpiresAt      *time.Time
}
