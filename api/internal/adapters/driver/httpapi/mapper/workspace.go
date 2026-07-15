package mapper

import (
	"time"

	"github.com/huynhanx03/datadock/internal/core/dto"
	"github.com/huynhanx03/datadock/internal/core/entity"
)

type WorkspaceCreateRequest struct {
	Name  string `json:"name"`
	Icon  string `json:"icon"`
	Color string `json:"color"`
}

type WorkspaceUpdateRequest struct {
	Name      string `json:"name"`
	Icon      string `json:"icon"`
	Color     string `json:"color"`
	Collapsed bool   `json:"collapsed"`
}

type WorkspaceOrderRequest struct {
	IDs []string `json:"ids"`
}

type WorkspaceResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Icon      string    `json:"icon"`
	Color     string    `json:"color"`
	Position  int       `json:"position"`
	Collapsed bool      `json:"collapsed"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func ToWorkspaceCreateInput(request WorkspaceCreateRequest) dto.CreateWorkspaceInput {
	return dto.CreateWorkspaceInput{Name: request.Name, Icon: request.Icon, Color: request.Color}
}

func ToWorkspaceUpdateInput(request WorkspaceUpdateRequest) dto.UpdateWorkspaceInput {
	return dto.UpdateWorkspaceInput{Name: request.Name, Icon: request.Icon, Color: request.Color, Collapsed: request.Collapsed}
}

func ToWorkspaceOrderInput(request WorkspaceOrderRequest) dto.ReorderWorkspaceInput {
	return dto.ReorderWorkspaceInput{IDs: request.IDs}
}

func ToWorkspaceResponse(workspace entity.Workspace) WorkspaceResponse {
	return WorkspaceResponse{
		ID:        workspace.ID,
		Name:      workspace.Name,
		Icon:      workspace.Icon,
		Color:     workspace.Color,
		Position:  workspace.Position,
		Collapsed: workspace.Collapsed,
		CreatedAt: workspace.CreatedAt,
		UpdatedAt: workspace.UpdatedAt,
	}
}

func ToWorkspaceResponses(workspaces []entity.Workspace) []WorkspaceResponse {
	result := make([]WorkspaceResponse, len(workspaces))
	for index, workspace := range workspaces {
		result[index] = ToWorkspaceResponse(workspace)
	}
	return result
}
