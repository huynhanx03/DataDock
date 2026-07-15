package dto

import "time"

const (
	SavedQueryFolderDeleteReject = "reject"
	SavedQueryFolderDeleteMove   = "move"
)

type SavedQueryCreateInput struct {
	ConnectionID *string  `json:"connectionId"`
	Folder       string   `json:"folder"`
	Title        string   `json:"title"`
	SQL          string   `json:"sql"`
	Tags         []string `json:"tags"`
}

type SavedQueryUpdateInput struct {
	ConnectionID    *string   `json:"connectionId"`
	ClearConnection bool      `json:"clearConnection"`
	Folder          *string   `json:"folder"`
	Title           *string   `json:"title"`
	SQL             *string   `json:"sql"`
	Tags            *[]string `json:"tags"`
}

type SavedQueryDetail struct {
	ID           string   `json:"id"`
	ConnectionID *string  `json:"connectionId,omitempty"`
	Folder       string   `json:"folder"`
	Title        string   `json:"title"`
	SQL          string   `json:"sql"`
	Tags         []string `json:"tags"`
	Favorite     bool     `json:"favorite"`
	CreatedAt    string   `json:"createdAt"`
	UpdatedAt    string   `json:"updatedAt"`
}

type SavedQueryFolderCreateInput struct {
	Name     string `json:"name"`
	Position int    `json:"position"`
}

type SavedQueryFolderUpdateInput struct {
	Name     *string `json:"name"`
	Position *int    `json:"position"`
}

type SavedQueryFolderDeleteInput struct {
	Policy      string `json:"policy"`
	Destination string `json:"destination"`
}

type SavedQueryFolderDetail struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Position   int    `json:"position"`
	QueryCount int    `json:"queryCount"`
	CreatedAt  string `json:"createdAt"`
	UpdatedAt  string `json:"updatedAt"`
}

type SavedQueryShareInput struct {
	ExpiresAt *time.Time `json:"expiresAt"`
}

type SavedQueryShareDetail struct {
	SavedQueryID string  `json:"savedQueryId"`
	Code         string  `json:"shareCode"`
	CreatedAt    string  `json:"createdAt"`
	ExpiresAt    *string `json:"expiresAt,omitempty"`
}

type PublicSavedQuery struct {
	Title     string   `json:"title"`
	SQL       string   `json:"sql"`
	Tags      []string `json:"tags"`
	CreatedAt string   `json:"createdAt"`
	SharedAt  string   `json:"sharedAt"`
	ExpiresAt *string  `json:"expiresAt,omitempty"`
}
