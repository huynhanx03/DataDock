package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/huynhanx03/datadock/internal/core/apperror"
	"github.com/huynhanx03/datadock/internal/core/dto"
	"github.com/huynhanx03/datadock/internal/ports"
)

const (
	defaultSavedQueryFolder = "General"
	maximumSavedQueryID     = 128
	maximumSavedQueryFolder = 128
	maximumSavedQueryTitle  = 200
	maximumSavedQuerySQL    = 100000
	maximumSavedQueryTags   = 30
	maximumSavedQueryTag    = 64
	maximumFolderPosition   = 1000000
	maximumShareCode        = 256
	shareCodeBytes          = 18
	shareCodeAttempts       = 5
)

var savedQueryCopySuffix = regexp.MustCompile(`(?i)\s+copy(?:\s+\d+)?$`)

type SavedQueryService struct {
	queries ports.SavedQueryRepository
	folders ports.SavedQueryFolderRepository
	shares  ports.SharedQueryRepository
}

func NewSavedQueryService(queries ports.SavedQueryRepository, folders ports.SavedQueryFolderRepository, shares ports.SharedQueryRepository) *SavedQueryService {
	return &SavedQueryService{queries: queries, folders: folders, shares: shares}
}

func (service *SavedQueryService) List(ctx context.Context) ([]dto.SavedQueryDetail, error) {
	queries, err := service.queries.List(ctx)
	if err != nil {
		return nil, savedQueryOperationError(err, "saved query not found", "saved query conflict")
	}
	items := make([]dto.SavedQueryDetail, len(queries))
	for index := range queries {
		items[index] = savedQueryDetail(queries[index])
	}
	return items, nil
}

func (service *SavedQueryService) Get(ctx context.Context, id string) (dto.SavedQueryDetail, error) {
	id, err := normalizeSavedQueryID(id)
	if err != nil {
		return dto.SavedQueryDetail{}, err
	}
	query, err := service.queries.Get(ctx, id)
	if err != nil {
		return dto.SavedQueryDetail{}, savedQueryOperationError(err, "saved query not found", "saved query conflict")
	}
	return savedQueryDetail(query), nil
}

func (service *SavedQueryService) Create(ctx context.Context, input dto.SavedQueryCreateInput) (dto.SavedQueryDetail, error) {
	query, err := savedQueryFromCreateInput(input)
	if err != nil {
		return dto.SavedQueryDetail{}, err
	}
	query.Folder, err = service.ensureFolder(ctx, query.Folder)
	if err != nil {
		return dto.SavedQueryDetail{}, err
	}
	created, err := service.queries.Create(ctx, query)
	if err != nil {
		return dto.SavedQueryDetail{}, savedQueryOperationError(err, "saved query not found", "saved query already exists")
	}
	return savedQueryDetail(created), nil
}

func (service *SavedQueryService) Update(ctx context.Context, id string, input dto.SavedQueryUpdateInput) (dto.SavedQueryDetail, error) {
	id, err := normalizeSavedQueryID(id)
	if err != nil {
		return dto.SavedQueryDetail{}, err
	}
	patch, err := savedQueryPatch(input)
	if err != nil {
		return dto.SavedQueryDetail{}, err
	}
	if patch.Folder != nil {
		if _, err := service.queries.Get(ctx, id); err != nil {
			return dto.SavedQueryDetail{}, savedQueryOperationError(err, "saved query not found", "saved query conflict")
		}
		folder, ensureErr := service.ensureFolder(ctx, *patch.Folder)
		if ensureErr != nil {
			return dto.SavedQueryDetail{}, ensureErr
		}
		patch.Folder = &folder
	}
	updated, err := service.queries.Patch(ctx, id, patch)
	if err != nil {
		return dto.SavedQueryDetail{}, savedQueryOperationError(err, "saved query not found", "saved query conflict")
	}
	return savedQueryDetail(updated), nil
}

func (service *SavedQueryService) ensureFolder(ctx context.Context, name string) (string, error) {
	now := time.Now().UTC()
	folder, err := service.folders.Ensure(ctx, ports.SavedQueryFolder{
		ID:        uuid.NewString(),
		Name:      name,
		Position:  maximumFolderPosition,
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		return "", savedQueryOperationError(err, "saved query folder not found", "saved query folder conflict")
	}
	return folder.Name, nil
}

func (service *SavedQueryService) Delete(ctx context.Context, id string) error {
	id, err := normalizeSavedQueryID(id)
	if err != nil {
		return err
	}
	return savedQueryOperationError(service.queries.Delete(ctx, id), "saved query not found", "saved query conflict")
}

func (service *SavedQueryService) Duplicate(ctx context.Context, id string) (dto.SavedQueryDetail, error) {
	id, err := normalizeSavedQueryID(id)
	if err != nil {
		return dto.SavedQueryDetail{}, err
	}
	duplicated, err := service.queries.DuplicateWithUniqueTitle(ctx, id, nextSavedQueryCopyTitle)
	if err != nil {
		return dto.SavedQueryDetail{}, savedQueryOperationError(err, "saved query not found", "saved query conflict")
	}
	return savedQueryDetail(duplicated), nil
}

func (service *SavedQueryService) SetFavorite(ctx context.Context, id string, favorite bool) (dto.SavedQueryDetail, error) {
	id, err := normalizeSavedQueryID(id)
	if err != nil {
		return dto.SavedQueryDetail{}, err
	}
	updated, err := service.queries.SetFavoriteAndGet(ctx, id, favorite, time.Now().UTC())
	if err != nil {
		return dto.SavedQueryDetail{}, savedQueryOperationError(err, "saved query not found", "saved query conflict")
	}
	return savedQueryDetail(updated), nil
}

func (service *SavedQueryService) ListFolders(ctx context.Context) ([]dto.SavedQueryFolderDetail, error) {
	folders, err := service.folders.List(ctx)
	if err != nil {
		return nil, savedQueryOperationError(err, "saved query folder not found", "saved query folder conflict")
	}
	items := make([]dto.SavedQueryFolderDetail, len(folders))
	for index := range folders {
		items[index] = savedQueryFolderDetail(folders[index])
	}
	return items, nil
}

func (service *SavedQueryService) CreateFolder(ctx context.Context, input dto.SavedQueryFolderCreateInput) (dto.SavedQueryFolderDetail, error) {
	name, err := normalizeRequiredText(input.Name, maximumSavedQueryFolder, "folder name")
	if err != nil || input.Position < 0 || input.Position > maximumFolderPosition {
		return dto.SavedQueryFolderDetail{}, apperror.NewValidation("invalid saved query folder", err)
	}
	now := time.Now().UTC()
	created, err := service.folders.Create(ctx, ports.SavedQueryFolder{
		ID:        uuid.NewString(),
		Name:      name,
		Position:  input.Position,
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		return dto.SavedQueryFolderDetail{}, savedQueryOperationError(err, "saved query folder not found", "saved query folder already exists")
	}
	return savedQueryFolderDetail(created), nil
}

func (service *SavedQueryService) UpdateFolder(ctx context.Context, id string, input dto.SavedQueryFolderUpdateInput) (dto.SavedQueryFolderDetail, error) {
	id, err := normalizeSavedQueryID(id)
	if err != nil {
		return dto.SavedQueryFolderDetail{}, err
	}
	if input.Name == nil && input.Position == nil {
		return dto.SavedQueryFolderDetail{}, apperror.NewValidation("saved query folder update is empty", nil)
	}
	patch := ports.SavedQueryFolderPatch{UpdatedAt: time.Now().UTC()}
	if input.Name != nil {
		name, validationErr := normalizeRequiredText(*input.Name, maximumSavedQueryFolder, "folder name")
		if validationErr != nil {
			return dto.SavedQueryFolderDetail{}, validationErr
		}
		patch.Name = &name
	}
	if input.Position != nil {
		if *input.Position < 0 || *input.Position > maximumFolderPosition {
			return dto.SavedQueryFolderDetail{}, apperror.NewValidation("invalid saved query folder position", nil)
		}
		position := *input.Position
		patch.Position = &position
	}
	updated, err := service.folders.UpdateAndPropagate(ctx, id, patch)
	if err != nil {
		return dto.SavedQueryFolderDetail{}, savedQueryOperationError(err, "saved query folder not found", "saved query folder already exists")
	}
	return savedQueryFolderDetail(updated), nil
}

func (service *SavedQueryService) DeleteFolder(ctx context.Context, id string, input dto.SavedQueryFolderDeleteInput) error {
	id, err := normalizeSavedQueryID(id)
	if err != nil {
		return err
	}
	mode := input.Policy
	if mode == "" {
		mode = dto.SavedQueryFolderDeleteReject
	}
	policy := ports.SavedQueryFolderDeletePolicy{UpdatedAt: time.Now().UTC()}
	switch mode {
	case dto.SavedQueryFolderDeleteReject:
		policy.Mode = ports.SavedQueryFolderDeleteReject
	case dto.SavedQueryFolderDeleteMove:
		policy.Mode = ports.SavedQueryFolderDeleteMove
		destination := strings.TrimSpace(input.Destination)
		if destination == "" {
			destination = defaultSavedQueryFolder
		}
		if invalidText(destination, maximumSavedQueryFolder) {
			return apperror.NewValidation("invalid destination folder", nil)
		}
		policy.Destination = destination
	default:
		return apperror.NewValidation("invalid saved query folder delete policy", nil)
	}
	return savedQueryOperationError(service.folders.DeleteWithPolicy(ctx, id, policy), "saved query folder not found", "saved query folder is not empty")
}

func (service *SavedQueryService) EnableShare(ctx context.Context, savedQueryID string, input dto.SavedQueryShareInput) (dto.SavedQueryShareDetail, error) {
	savedQueryID, err := normalizeSavedQueryID(savedQueryID)
	if err != nil {
		return dto.SavedQueryShareDetail{}, err
	}
	now := time.Now().UTC()
	var expiresAt *time.Time
	if input.ExpiresAt != nil {
		expires := input.ExpiresAt.UTC()
		if !expires.After(now) {
			return dto.SavedQueryShareDetail{}, apperror.NewValidation("share expiry must be in the future", nil)
		}
		expiresAt = &expires
	}
	for attempt := 0; attempt < shareCodeAttempts; attempt++ {
		code, randomErr := newSavedQueryShareCode()
		if randomErr != nil {
			return dto.SavedQueryShareDetail{}, apperror.NewInternal("saved query sharing failed", randomErr)
		}
		shared, enableErr := service.shares.Enable(ctx, ports.SharedQuery{
			ID:           uuid.NewString(),
			SavedQueryID: savedQueryID,
			ShareCode:    code,
			CreatedAt:    now,
			ExpiresAt:    expiresAt,
		})
		if enableErr == nil {
			return savedQueryShareDetail(shared), nil
		}
		if !errors.Is(enableErr, ports.ErrConflict) {
			return dto.SavedQueryShareDetail{}, savedQueryOperationError(enableErr, "saved query not found", "share code conflict")
		}
	}
	return dto.SavedQueryShareDetail{}, apperror.NewConflict("could not allocate a unique share code", ports.ErrConflict)
}

func (service *SavedQueryService) GetShare(ctx context.Context, savedQueryID string) (dto.SavedQueryShareDetail, error) {
	savedQueryID, err := normalizeSavedQueryID(savedQueryID)
	if err != nil {
		return dto.SavedQueryShareDetail{}, err
	}
	shared, err := service.shares.GetBySavedQuery(ctx, savedQueryID)
	if err != nil {
		return dto.SavedQueryShareDetail{}, savedQueryOperationError(err, "saved query share not found", "saved query share conflict")
	}
	return savedQueryShareDetail(shared), nil
}

func (service *SavedQueryService) RevokeShare(ctx context.Context, savedQueryID string) error {
	savedQueryID, err := normalizeSavedQueryID(savedQueryID)
	if err != nil {
		return err
	}
	return savedQueryOperationError(service.shares.Revoke(ctx, savedQueryID), "saved query not found", "saved query share conflict")
}

func (service *SavedQueryService) PublicShare(ctx context.Context, code string) (dto.PublicSavedQuery, error) {
	code = strings.TrimSpace(code)
	if invalidText(code, maximumShareCode) {
		return dto.PublicSavedQuery{}, apperror.NewValidation("invalid share code", nil)
	}
	shared, err := service.shares.LookupPublic(ctx, code, time.Now().UTC())
	if err != nil {
		return dto.PublicSavedQuery{}, savedQueryOperationError(err, "shared query not found", "shared query conflict")
	}
	return publicSavedQuery(shared), nil
}

func savedQueryFromCreateInput(input dto.SavedQueryCreateInput) (ports.SavedQuery, error) {
	title, err := normalizeRequiredText(input.Title, maximumSavedQueryTitle, "saved query title")
	if err != nil {
		return ports.SavedQuery{}, err
	}
	sqlText, err := normalizeRequiredText(input.SQL, maximumSavedQuerySQL, "saved query SQL")
	if err != nil {
		return ports.SavedQuery{}, err
	}
	folder := strings.TrimSpace(input.Folder)
	if folder == "" {
		folder = defaultSavedQueryFolder
	}
	if invalidText(folder, maximumSavedQueryFolder) {
		return ports.SavedQuery{}, apperror.NewValidation("invalid saved query folder", nil)
	}
	tags, err := normalizeSavedQueryTags(input.Tags)
	if err != nil {
		return ports.SavedQuery{}, err
	}
	connectionID, err := normalizeOptionalConnectionID(input.ConnectionID)
	if err != nil {
		return ports.SavedQuery{}, err
	}
	now := time.Now().UTC()
	return ports.SavedQuery{
		ID:           uuid.NewString(),
		ConnectionID: connectionID,
		Folder:       folder,
		Title:        title,
		SQLText:      sqlText,
		Tags:         tags,
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}

func savedQueryPatch(input dto.SavedQueryUpdateInput) (ports.SavedQueryPatch, error) {
	if input.ConnectionID == nil && !input.ClearConnection && input.Folder == nil && input.Title == nil && input.SQL == nil && input.Tags == nil {
		return ports.SavedQueryPatch{}, apperror.NewValidation("saved query update is empty", nil)
	}
	if input.ConnectionID != nil && input.ClearConnection {
		return ports.SavedQueryPatch{}, apperror.NewValidation("connection update is ambiguous", nil)
	}
	patch := ports.SavedQueryPatch{UpdatedAt: time.Now().UTC()}
	if input.ClearConnection {
		patch.ConnectionIDSet = true
	}
	if input.ConnectionID != nil {
		connectionID, err := normalizeOptionalConnectionID(input.ConnectionID)
		if err != nil || connectionID == nil {
			if err == nil {
				err = apperror.NewValidation("connection id is required", nil)
			}
			return ports.SavedQueryPatch{}, err
		}
		patch.ConnectionIDSet = true
		patch.ConnectionID = connectionID
	}
	if input.Folder != nil {
		folder := strings.TrimSpace(*input.Folder)
		if folder == "" {
			folder = defaultSavedQueryFolder
		}
		if invalidText(folder, maximumSavedQueryFolder) {
			return ports.SavedQueryPatch{}, apperror.NewValidation("invalid saved query folder", nil)
		}
		patch.Folder = &folder
	}
	if input.Title != nil {
		title, err := normalizeRequiredText(*input.Title, maximumSavedQueryTitle, "saved query title")
		if err != nil {
			return ports.SavedQueryPatch{}, err
		}
		patch.Title = &title
	}
	if input.SQL != nil {
		sqlText, err := normalizeRequiredText(*input.SQL, maximumSavedQuerySQL, "saved query SQL")
		if err != nil {
			return ports.SavedQueryPatch{}, err
		}
		patch.SQLText = &sqlText
	}
	if input.Tags != nil {
		tags, err := normalizeSavedQueryTags(*input.Tags)
		if err != nil {
			return ports.SavedQueryPatch{}, err
		}
		patch.Tags = &tags
	}
	return patch, nil
}

func normalizeSavedQueryTags(input []string) ([]string, error) {
	if len(input) > maximumSavedQueryTags {
		return nil, apperror.NewValidation("too many saved query tags", nil)
	}
	tags := make([]string, 0, len(input))
	seen := make(map[string]struct{}, len(input))
	for _, value := range input {
		tag := strings.TrimSpace(value)
		if tag == "" {
			continue
		}
		if invalidText(tag, maximumSavedQueryTag) {
			return nil, apperror.NewValidation("invalid saved query tag", nil)
		}
		key := strings.ToLower(tag)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		tags = append(tags, tag)
	}
	return tags, nil
}

func normalizeOptionalConnectionID(input *string) (*string, error) {
	if input == nil {
		return nil, nil
	}
	value := strings.TrimSpace(*input)
	if value == "" {
		return nil, nil
	}
	if invalidText(value, maximumSavedQueryID) {
		return nil, apperror.NewValidation("invalid connection id", nil)
	}
	return &value, nil
}

func normalizeSavedQueryID(id string) (string, error) {
	id = strings.TrimSpace(id)
	if invalidText(id, maximumSavedQueryID) {
		return "", apperror.NewValidation("invalid saved query id", nil)
	}
	return id, nil
}

func normalizeRequiredText(value string, maximum int, field string) (string, error) {
	value = strings.TrimSpace(value)
	if invalidText(value, maximum) {
		return "", apperror.NewValidation(field+" is invalid", nil)
	}
	return value, nil
}

func invalidText(value string, maximum int) bool {
	return value == "" || utf8.RuneCountInString(value) > maximum || strings.ContainsRune(value, 0) || !utf8.ValidString(value)
}

func nextSavedQueryCopyTitle(source string, existing []string) string {
	base := strings.TrimSpace(source)
	for {
		next := savedQueryCopySuffix.ReplaceAllString(base, "")
		if next == base {
			break
		}
		base = strings.TrimSpace(next)
	}
	if base == "" {
		base = "Query"
	}
	occupied := make(map[string]struct{}, len(existing))
	for _, title := range existing {
		occupied[strings.ToLower(strings.TrimSpace(title))] = struct{}{}
	}
	for number := 1; number <= len(existing)+2; number++ {
		suffix := " copy"
		if number > 1 {
			suffix += " " + strconv.Itoa(number)
		}
		candidate := truncateSavedQueryTitle(base, maximumSavedQueryTitle-utf8.RuneCountInString(suffix)) + suffix
		if _, exists := occupied[strings.ToLower(candidate)]; !exists {
			return candidate
		}
	}
	return truncateSavedQueryTitle(base, maximumSavedQueryTitle-5) + " copy"
}

func truncateSavedQueryTitle(value string, maximum int) string {
	if maximum <= 0 {
		return ""
	}
	if utf8.RuneCountInString(value) <= maximum {
		return value
	}
	return string([]rune(value)[:maximum])
}

func newSavedQueryShareCode() (string, error) {
	buffer := make([]byte, shareCodeBytes)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

func savedQueryDetail(query ports.SavedQuery) dto.SavedQueryDetail {
	tags := append([]string(nil), query.Tags...)
	if tags == nil {
		tags = []string{}
	}
	return dto.SavedQueryDetail{
		ID:           query.ID,
		ConnectionID: cloneStringPointer(query.ConnectionID),
		Folder:       query.Folder,
		Title:        query.Title,
		SQL:          query.SQLText,
		Tags:         tags,
		Favorite:     query.Favorite,
		CreatedAt:    formatSavedQueryTime(query.CreatedAt),
		UpdatedAt:    formatSavedQueryTime(query.UpdatedAt),
	}
}

func savedQueryFolderDetail(folder ports.SavedQueryFolder) dto.SavedQueryFolderDetail {
	return dto.SavedQueryFolderDetail{
		ID:         folder.ID,
		Name:       folder.Name,
		Position:   folder.Position,
		QueryCount: folder.QueryCount,
		CreatedAt:  formatSavedQueryTime(folder.CreatedAt),
		UpdatedAt:  formatSavedQueryTime(folder.UpdatedAt),
	}
}

func savedQueryShareDetail(shared ports.SharedQuery) dto.SavedQueryShareDetail {
	return dto.SavedQueryShareDetail{
		SavedQueryID: shared.SavedQueryID,
		Code:         shared.ShareCode,
		CreatedAt:    formatSavedQueryTime(shared.CreatedAt),
		ExpiresAt:    formatOptionalSavedQueryTime(shared.ExpiresAt),
	}
}

func publicSavedQuery(shared ports.PublicSharedQuery) dto.PublicSavedQuery {
	tags := append([]string(nil), shared.Tags...)
	if tags == nil {
		tags = []string{}
	}
	return dto.PublicSavedQuery{
		Title:     shared.Title,
		SQL:       shared.SQLText,
		Tags:      tags,
		CreatedAt: formatSavedQueryTime(shared.QueryCreatedAt),
		SharedAt:  formatSavedQueryTime(shared.SharedAt),
		ExpiresAt: formatOptionalSavedQueryTime(shared.ExpiresAt),
	}
}

func formatSavedQueryTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}

func formatOptionalSavedQueryTime(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := formatSavedQueryTime(*value)
	return &formatted
}

func cloneStringPointer(value *string) *string {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}

func savedQueryOperationError(err error, notFoundMessage, conflictMessage string) error {
	if err == nil {
		return nil
	}
	var applicationError *apperror.Error
	if errors.As(err, &applicationError) {
		return err
	}
	if errors.Is(err, ports.ErrNotFound) {
		return apperror.NewNotFound(notFoundMessage, err)
	}
	if errors.Is(err, ports.ErrConflict) {
		return apperror.NewConflict(conflictMessage, err)
	}
	return apperror.NewInternal("saved query operation failed", err)
}
