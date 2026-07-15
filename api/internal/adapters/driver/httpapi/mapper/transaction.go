package mapper

import (
	"time"

	"github.com/huynhanx03/datadock/internal/core/dto"
)

type TransactionSavepointRequest struct {
	Name string `json:"name"`
}

type TransactionSavepointResponse struct {
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
}

type TransactionResponse struct {
	ID             string                         `json:"id"`
	ConnectionID   string                         `json:"connectionId"`
	State          dto.TransactionLifecycleState  `json:"state"`
	StartedAt      time.Time                      `json:"startedAt"`
	LastActivityAt time.Time                      `json:"lastActivityAt"`
	ExpiresAt      time.Time                      `json:"expiresAt"`
	CompletedAt    *time.Time                     `json:"completedAt,omitempty"`
	Savepoints     []TransactionSavepointResponse `json:"savepoints"`
}

func ToTransactionBeginInput(connectionID string) dto.TransactionBeginInput {
	return dto.TransactionBeginInput{ConnectionID: connectionID}
}

func ToTransactionActionInput(connectionID, transactionID string) dto.TransactionActionInput {
	return dto.TransactionActionInput{ConnectionID: connectionID, TransactionID: transactionID}
}

func ToTransactionSavepointInput(connectionID, transactionID string, request TransactionSavepointRequest) dto.TransactionSavepointInput {
	return dto.TransactionSavepointInput{ConnectionID: connectionID, TransactionID: transactionID, Name: request.Name}
}

func ToTransactionResponse(view dto.TransactionView) TransactionResponse {
	savepoints := make([]TransactionSavepointResponse, len(view.Savepoints))
	for index, savepoint := range view.Savepoints {
		savepoints[index] = TransactionSavepointResponse{Name: savepoint.Name, CreatedAt: savepoint.CreatedAt}
	}
	return TransactionResponse{
		ID:             view.ID,
		ConnectionID:   view.ConnectionID,
		State:          view.State,
		StartedAt:      view.StartedAt,
		LastActivityAt: view.LastActivityAt,
		ExpiresAt:      view.ExpiresAt,
		CompletedAt:    view.CompletedAt,
		Savepoints:     savepoints,
	}
}
