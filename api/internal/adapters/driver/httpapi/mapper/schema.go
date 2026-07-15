package mapper

import (
	"time"

	"github.com/huynhanx03/datadock/internal/core/dto"
	"github.com/huynhanx03/datadock/internal/core/entity"
)

type SchemaPreviewRequest struct {
	Actions []dto.SchemaAction `json:"actions"`
}

type SchemaApplyRequest struct {
	Actions            []dto.SchemaAction `json:"actions"`
	PreviewHash        string             `json:"previewHash"`
	ConfirmDestructive bool               `json:"confirmDestructive"`
}

type SchemaPreviewResponse struct {
	ConnectionID string             `json:"connectionId"`
	Engine       entity.Engine      `json:"engine"`
	Actions      []dto.SchemaAction `json:"actions"`
	Steps        []dto.SchemaStep   `json:"steps"`
	SQL          string             `json:"sql"`
	Hash         string             `json:"hash"`
	Destructive  bool               `json:"destructive"`
	GeneratedAt  time.Time          `json:"generatedAt"`
}

type SchemaApplyResponse struct {
	PreviewHash  string    `json:"previewHash"`
	AppliedSteps int       `json:"appliedSteps"`
	AppliedAt    time.Time `json:"appliedAt"`
}

func ToSchemaPreviewInput(request SchemaPreviewRequest) dto.SchemaPreviewInput {
	return dto.SchemaPreviewInput{Actions: request.Actions}
}

func ToSchemaApplyInput(request SchemaApplyRequest) dto.SchemaApplyInput {
	return dto.SchemaApplyInput{
		Actions:            request.Actions,
		PreviewHash:        request.PreviewHash,
		ConfirmDestructive: request.ConfirmDestructive,
	}
}

func ToSchemaPreviewResponse(preview dto.SchemaPreview) SchemaPreviewResponse {
	actions := preview.Actions
	if actions == nil {
		actions = []dto.SchemaAction{}
	}
	steps := preview.Steps
	if steps == nil {
		steps = []dto.SchemaStep{}
	}
	return SchemaPreviewResponse{
		ConnectionID: preview.ConnectionID,
		Engine:       preview.Engine,
		Actions:      actions,
		Steps:        steps,
		SQL:          preview.SQL,
		Hash:         preview.Hash,
		Destructive:  preview.Destructive,
		GeneratedAt:  preview.GeneratedAt,
	}
}

func ToSchemaApplyResponse(result dto.SchemaApplyResult) SchemaApplyResponse {
	return SchemaApplyResponse{PreviewHash: result.PreviewHash, AppliedSteps: result.AppliedSteps, AppliedAt: result.AppliedAt}
}
