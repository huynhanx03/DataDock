package dto

import "time"

type TransactionLifecycleState string

const (
	TransactionActive     TransactionLifecycleState = "active"
	TransactionCommitted  TransactionLifecycleState = "committed"
	TransactionRolledBack TransactionLifecycleState = "rolled_back"
	TransactionExpired    TransactionLifecycleState = "expired"
)

type TransactionSavepoint struct {
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
}

type TransactionView struct {
	ID             string                    `json:"id"`
	ConnectionID   string                    `json:"connectionId"`
	State          TransactionLifecycleState `json:"state"`
	StartedAt      time.Time                 `json:"startedAt"`
	LastActivityAt time.Time                 `json:"lastActivityAt"`
	ExpiresAt      time.Time                 `json:"expiresAt"`
	CompletedAt    *time.Time                `json:"completedAt,omitempty"`
	Savepoints     []TransactionSavepoint    `json:"savepoints"`
}

type TransactionBeginInput struct {
	ConnectionID string `json:"connectionId"`
}

type TransactionActionInput struct {
	ConnectionID  string `json:"connectionId"`
	TransactionID string `json:"transactionId"`
}

type TransactionSavepointInput struct {
	ConnectionID  string `json:"connectionId"`
	TransactionID string `json:"transactionId"`
	Name          string `json:"name"`
}
