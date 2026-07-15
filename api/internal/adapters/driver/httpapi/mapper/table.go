package mapper

import "github.com/huynhanx03/datadock/internal/core/dto"

type TableMutationRequest struct {
	Reference     string              `json:"reference"`
	TransactionID string              `json:"transactionId,omitempty"`
	Mutations     []dto.TableMutation `json:"mutations"`
}

type TableMutationResponse struct {
	Status    dto.TableMutationBatchStatus `json:"status"`
	Atomic    bool                         `json:"atomic"`
	Applied   int                          `json:"applied"`
	Conflicts []dto.TableMutationConflict  `json:"conflicts"`
}

func ToTableMutationInput(connectionID string, request TableMutationRequest) dto.TableMutationBatchInput {
	return dto.TableMutationBatchInput{
		ConnectionID:  connectionID,
		Reference:     request.Reference,
		TransactionID: request.TransactionID,
		Mutations:     request.Mutations,
	}
}

func ToTableMutationResponse(result dto.TableMutationBatchResult) TableMutationResponse {
	conflicts := result.Conflicts
	if conflicts == nil {
		conflicts = []dto.TableMutationConflict{}
	}
	return TableMutationResponse{
		Status:    result.Status,
		Atomic:    result.Atomic,
		Applied:   result.Applied,
		Conflicts: conflicts,
	}
}
