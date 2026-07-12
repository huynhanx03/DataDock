package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/huynhanx03/datadock/internal/core/dto"
	"github.com/huynhanx03/datadock/internal/core/entity"
	"github.com/huynhanx03/datadock/internal/ports"
)

var ErrWorkspaceNameRequired = errors.New("workspace name is required")

type WorkspaceService struct {
	repository ports.WorkspaceRepository
}

func NewWorkspaceService(repository ports.WorkspaceRepository) *WorkspaceService {
	return &WorkspaceService{repository: repository}
}

func (service *WorkspaceService) Create(ctx context.Context, input dto.CreateWorkspaceInput) (entity.Workspace, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return entity.Workspace{}, ErrWorkspaceNameRequired
	}
	workspaces, err := service.repository.List(ctx)
	if err != nil {
		return entity.Workspace{}, err
	}
	now := time.Now().UTC()
	return service.repository.Create(ctx, entity.Workspace{ID: uuid.NewString(), Name: name, Icon: input.Icon, Color: input.Color, Position: len(workspaces), CreatedAt: now, UpdatedAt: now})
}

func (service *WorkspaceService) List(ctx context.Context) ([]entity.Workspace, error) {
	return service.repository.List(ctx)
}

func (service *WorkspaceService) Update(ctx context.Context, id string, input dto.UpdateWorkspaceInput) (entity.Workspace, error) {
	workspace, err := service.repository.Get(ctx, id)
	if err != nil {
		return entity.Workspace{}, err
	}
	workspace.Name = strings.TrimSpace(input.Name)
	if workspace.Name == "" {
		return entity.Workspace{}, ErrWorkspaceNameRequired
	}
	workspace.Icon = input.Icon
	workspace.Color = input.Color
	workspace.Collapsed = input.Collapsed
	workspace.UpdatedAt = time.Now().UTC()
	return service.repository.Update(ctx, workspace)
}

func (service *WorkspaceService) Delete(ctx context.Context, id string) error {
	return service.repository.Delete(ctx, id)
}

func (service *WorkspaceService) Reorder(ctx context.Context, ids []string) error {
	return service.repository.Reorder(ctx, ids)
}
