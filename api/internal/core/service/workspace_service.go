package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/huynhanx03/datadock/internal/core/apperror"
	"github.com/huynhanx03/datadock/internal/core/dto"
	"github.com/huynhanx03/datadock/internal/core/entity"
	"github.com/huynhanx03/datadock/internal/ports"
)

type WorkspaceService struct {
	repository ports.WorkspaceRepository
}

func NewWorkspaceService(repository ports.WorkspaceRepository) *WorkspaceService {
	return &WorkspaceService{repository: repository}
}

func (service *WorkspaceService) Create(ctx context.Context, input dto.CreateWorkspaceInput) (entity.Workspace, error) {
	input.Name = strings.TrimSpace(input.Name)
	if !validWorkspacePresentation(input.Name, input.Icon, input.Color) {
		return entity.Workspace{}, apperror.NewValidation("invalid workspace", nil)
	}
	workspaces, err := service.repository.List(ctx)
	if err != nil {
		return entity.Workspace{}, workspaceServiceError(err)
	}
	now := time.Now().UTC()
	workspace, err := service.repository.Create(ctx, entity.Workspace{ID: uuid.NewString(), Name: input.Name, Icon: input.Icon, Color: input.Color, Position: len(workspaces), CreatedAt: now, UpdatedAt: now})
	return workspace, workspaceServiceError(err)
}

func (service *WorkspaceService) List(ctx context.Context) ([]entity.Workspace, error) {
	workspaces, err := service.repository.List(ctx)
	if err != nil {
		return nil, workspaceServiceError(err)
	}
	if workspaces == nil {
		workspaces = []entity.Workspace{}
	}
	return workspaces, nil
}

func (service *WorkspaceService) Update(ctx context.Context, id string, input dto.UpdateWorkspaceInput) (entity.Workspace, error) {
	if !validWorkspaceIdentifier(id) {
		return entity.Workspace{}, apperror.NewValidation("invalid workspace", nil)
	}
	workspace, err := service.repository.Get(ctx, id)
	if err != nil {
		return entity.Workspace{}, workspaceServiceError(err)
	}
	workspace.Name = strings.TrimSpace(input.Name)
	if !validWorkspacePresentation(workspace.Name, input.Icon, input.Color) {
		return entity.Workspace{}, apperror.NewValidation("invalid workspace", nil)
	}
	workspace.Icon = input.Icon
	workspace.Color = input.Color
	workspace.Collapsed = input.Collapsed
	workspace.UpdatedAt = time.Now().UTC()
	workspace, err = service.repository.Update(ctx, workspace)
	return workspace, workspaceServiceError(err)
}

func (service *WorkspaceService) Delete(ctx context.Context, id string) error {
	if !validWorkspaceIdentifier(id) {
		return apperror.NewValidation("invalid workspace", nil)
	}
	return workspaceServiceError(service.repository.Delete(ctx, id))
}

func (service *WorkspaceService) Reorder(ctx context.Context, ids []string) error {
	if len(ids) == 0 || len(ids) > 1000 {
		return apperror.NewValidation("invalid workspace order", nil)
	}
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if !validWorkspaceIdentifier(id) {
			return apperror.NewValidation("invalid workspace order", nil)
		}
		if _, exists := seen[id]; exists {
			return apperror.NewValidation("invalid workspace order", nil)
		}
		seen[id] = struct{}{}
	}
	workspaces, err := service.repository.List(ctx)
	if err != nil {
		return workspaceServiceError(err)
	}
	if len(workspaces) != len(ids) {
		return apperror.NewConflict("workspace order is stale", nil)
	}
	for _, workspace := range workspaces {
		if _, exists := seen[workspace.ID]; !exists {
			return apperror.NewConflict("workspace order is stale", nil)
		}
	}
	return workspaceServiceError(service.repository.Reorder(ctx, ids))
}

func validWorkspacePresentation(name, icon, color string) bool {
	return name != "" && len(name) <= 80 && len(icon) <= 32 && len(color) <= 32 && validWorkspaceText(name) && validWorkspaceText(icon) && validWorkspaceText(color)
}

func validWorkspaceIdentifier(value string) bool {
	return value != "" && len(value) <= 128 && strings.TrimSpace(value) == value && validWorkspaceText(value)
}

func validWorkspaceText(value string) bool {
	for _, character := range value {
		if character < 32 || character == 127 {
			return false
		}
	}
	return true
}

func workspaceServiceError(err error) error {
	if err == nil {
		return nil
	}
	var applicationError *apperror.Error
	if errors.As(err, &applicationError) {
		return err
	}
	if errors.Is(err, ports.ErrNotFound) {
		return apperror.NewNotFound("workspace was not found", err)
	}
	if errors.Is(err, context.Canceled) {
		return apperror.NewCancellation("workspace operation was cancelled", err)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return apperror.NewTimeout("workspace operation timed out", err)
	}
	return apperror.NewInternal("workspace operation failed", err)
}
