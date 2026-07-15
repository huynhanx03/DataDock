package service_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/huynhanx03/datadock/internal/core/apperror"
	"github.com/huynhanx03/datadock/internal/core/dto"
	"github.com/huynhanx03/datadock/internal/core/service"
	"github.com/huynhanx03/datadock/internal/ports"
)

func TestSavedQueryServiceCreateNormalizesAndPersists(t *testing.T) {
	queries := newMemorySavedQueries()
	folders := newMemoryFolders(queries)
	subject := newSavedQueryService(queries, folders, newMemoryShares(queries))
	connectionID := " connection-1 "

	created, err := subject.Create(context.Background(), dto.SavedQueryCreateInput{
		ConnectionID: &connectionID,
		Title:        "  Daily revenue  ",
		SQL:          "  SELECT sum(total) FROM orders  ",
		Tags:         []string{" Revenue ", "daily", "revenue", ""},
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.ID == "" || created.ConnectionID == nil || *created.ConnectionID != "connection-1" {
		t.Fatalf("Create() identity = %#v", created)
	}
	if created.Title != "Daily revenue" || created.SQL != "SELECT sum(total) FROM orders" || created.Folder != "General" {
		t.Fatalf("Create() content = %#v", created)
	}
	if strings.Join(created.Tags, ",") != "Revenue,daily" || created.Favorite {
		t.Fatalf("Create() metadata = %#v", created)
	}
	if created.CreatedAt == "" || created.UpdatedAt == "" || len(queries.items) != 1 {
		t.Fatalf("Create() persistence = %#v, items = %#v", created, queries.items)
	}
	if len(folders.items) != 1 {
		t.Fatalf("Create() folders = %#v", folders.items)
	}
	for _, folder := range folders.items {
		if folder.Name != "General" {
			t.Fatalf("Create() folder = %#v", folder)
		}
	}
}

func TestSavedQueryServiceEnsuresCanonicalFoldersForCreateAndUpdate(t *testing.T) {
	queries := newMemorySavedQueries()
	folders := newMemoryFolders(queries)
	subject := newSavedQueryService(queries, folders, newMemoryShares(queries))

	managed, err := subject.CreateFolder(context.Background(), dto.SavedQueryFolderCreateInput{Name: "Analytics", Position: 2})
	if err != nil {
		t.Fatalf("CreateFolder() error = %v", err)
	}
	created, err := subject.Create(context.Background(), dto.SavedQueryCreateInput{Title: "Query", SQL: "SELECT 1", Folder: "analytics"})
	if err != nil || created.Folder != managed.Name || len(folders.items) != 1 {
		t.Fatalf("Create() = %#v, %v, folders = %#v", created, err, folders.items)
	}
	newFolder := "Reporting"
	updated, err := subject.Update(context.Background(), created.ID, dto.SavedQueryUpdateInput{Folder: &newFolder})
	if err != nil || updated.Folder != newFolder || len(folders.items) != 2 {
		t.Fatalf("Update() = %#v, %v, folders = %#v", updated, err, folders.items)
	}
}

func TestSavedQueryServiceRejectsInvalidCreateAndPatchInputs(t *testing.T) {
	queries := newMemorySavedQueries()
	subject := newSavedQueryService(queries, newMemoryFolders(queries), newMemoryShares(queries))
	created := queries.seed(ports.SavedQuery{Title: "Valid", SQLText: "SELECT 1", Folder: "General"})
	empty := " "
	tooManyTags := make([]string, 31)
	for index := range tooManyTags {
		tooManyTags[index] = "tag"
	}

	tests := []struct {
		name string
		run  func() error
	}{
		{name: "missing title", run: func() error {
			_, err := subject.Create(context.Background(), dto.SavedQueryCreateInput{SQL: "SELECT 1"})
			return err
		}},
		{name: "missing SQL", run: func() error {
			_, err := subject.Create(context.Background(), dto.SavedQueryCreateInput{Title: "Query"})
			return err
		}},
		{name: "too many tags", run: func() error {
			_, err := subject.Create(context.Background(), dto.SavedQueryCreateInput{Title: "Query", SQL: "SELECT 1", Tags: tooManyTags})
			return err
		}},
		{name: "empty patch", run: func() error {
			_, err := subject.Update(context.Background(), created.ID, dto.SavedQueryUpdateInput{})
			return err
		}},
		{name: "blank patch title", run: func() error {
			_, err := subject.Update(context.Background(), created.ID, dto.SavedQueryUpdateInput{Title: &empty})
			return err
		}},
		{name: "blank id", run: func() error {
			_, err := subject.Get(context.Background(), " ")
			return err
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if code := savedQueryErrorCode(test.run()); code != apperror.CodeValidation {
				t.Fatalf("error code = %q", code)
			}
		})
	}
}

func TestSavedQueryServiceListsGetsUpdatesAndDeletes(t *testing.T) {
	queries := newMemorySavedQueries()
	subject := newSavedQueryService(queries, newMemoryFolders(queries), newMemoryShares(queries))
	connectionID := "connection-1"
	seeded := queries.seed(ports.SavedQuery{ConnectionID: &connectionID, Folder: "Revenue", Title: "Revenue", SQLText: "SELECT 1", Tags: []string{"old"}, Favorite: true})

	items, err := subject.List(context.Background())
	if err != nil || len(items) != 1 || items[0].ID != seeded.ID || !items[0].Favorite {
		t.Fatalf("List() = %#v, %v", items, err)
	}
	item, err := subject.Get(context.Background(), seeded.ID)
	if err != nil || item.ID != seeded.ID {
		t.Fatalf("Get() = %#v, %v", item, err)
	}
	newTitle := "  Net revenue "
	newFolder := " Reporting "
	newSQL := " SELECT 2 "
	tags := []string{"finance", " Finance ", "daily"}
	updated, err := subject.Update(context.Background(), seeded.ID, dto.SavedQueryUpdateInput{
		ClearConnection: true,
		Folder:          &newFolder,
		Title:           &newTitle,
		SQL:             &newSQL,
		Tags:            &tags,
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.ConnectionID != nil || updated.Title != "Net revenue" || updated.Folder != "Reporting" || updated.SQL != "SELECT 2" || strings.Join(updated.Tags, ",") != "finance,daily" {
		t.Fatalf("Update() = %#v", updated)
	}
	if err := subject.Delete(context.Background(), seeded.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, ok := queries.items[seeded.ID]; ok {
		t.Fatal("Delete() left saved query")
	}
	if _, err := subject.Get(context.Background(), seeded.ID); savedQueryErrorCode(err) != apperror.CodeNotFound {
		t.Fatalf("Get() missing error = %v", err)
	}
}

func TestSavedQueryServiceDuplicateUsesCleanCollisionSafeName(t *testing.T) {
	queries := newMemorySavedQueries()
	subject := newSavedQueryService(queries, newMemoryFolders(queries), newMemoryShares(queries))
	source := queries.seed(ports.SavedQuery{Folder: "Revenue", Title: "Revenue copy 2", SQLText: "SELECT 1", Tags: []string{"finance"}, Favorite: true})
	queries.seed(ports.SavedQuery{Folder: "Revenue", Title: "Revenue copy", SQLText: "SELECT 1"})
	queries.seed(ports.SavedQuery{Folder: "Revenue", Title: "Revenue copy 3", SQLText: "SELECT 1"})

	duplicated, err := subject.Duplicate(context.Background(), source.ID)
	if err != nil {
		t.Fatalf("Duplicate() error = %v", err)
	}
	if duplicated.ID == source.ID || duplicated.Title != "Revenue copy 4" || duplicated.Favorite {
		t.Fatalf("Duplicate() = %#v", duplicated)
	}
	if duplicated.SQL != "SELECT 1" || duplicated.Folder != "Revenue" || strings.Join(duplicated.Tags, ",") != "finance" {
		t.Fatalf("Duplicate() content = %#v", duplicated)
	}
}

func TestSavedQueryServiceSetsFavoriteAndMapsRepositoryErrors(t *testing.T) {
	queries := newMemorySavedQueries()
	subject := newSavedQueryService(queries, newMemoryFolders(queries), newMemoryShares(queries))
	seeded := queries.seed(ports.SavedQuery{Folder: "General", Title: "Query", SQLText: "SELECT 1"})

	updated, err := subject.SetFavorite(context.Background(), seeded.ID, true)
	if err != nil || !updated.Favorite {
		t.Fatalf("SetFavorite() = %#v, %v", updated, err)
	}
	if _, err := subject.SetFavorite(context.Background(), "missing", true); savedQueryErrorCode(err) != apperror.CodeNotFound {
		t.Fatalf("SetFavorite() missing error = %v", err)
	}
	queries.listErr = errors.New("metadata unavailable")
	if _, err := subject.List(context.Background()); savedQueryErrorCode(err) != apperror.CodeInternal || err.Error() != "saved query operation failed" {
		t.Fatalf("List() internal error = %v", err)
	}
}

func TestSavedQueryServiceManagesFoldersWithAtomicRenameAndDeletePolicies(t *testing.T) {
	queries := newMemorySavedQueries()
	folders := newMemoryFolders(queries)
	subject := newSavedQueryService(queries, folders, newMemoryShares(queries))
	query := queries.seed(ports.SavedQuery{Folder: "Revenue", Title: "Query", SQLText: "SELECT 1"})

	created, err := subject.CreateFolder(context.Background(), dto.SavedQueryFolderCreateInput{Name: " Revenue ", Position: 2})
	if err != nil || created.Name != "Revenue" || created.Position != 2 {
		t.Fatalf("CreateFolder() = %#v, %v", created, err)
	}
	name := " Finance "
	position := 3
	updated, err := subject.UpdateFolder(context.Background(), created.ID, dto.SavedQueryFolderUpdateInput{Name: &name, Position: &position})
	if err != nil || updated.Name != "Finance" || updated.Position != 3 {
		t.Fatalf("UpdateFolder() = %#v, %v", updated, err)
	}
	if queries.items[query.ID].Folder != "Finance" || folders.renameCalls != 1 {
		t.Fatalf("folder rename propagation = %#v, calls = %d", queries.items[query.ID], folders.renameCalls)
	}
	if err := subject.DeleteFolder(context.Background(), created.ID, dto.SavedQueryFolderDeleteInput{}); savedQueryErrorCode(err) != apperror.CodeConflict {
		t.Fatalf("DeleteFolder() reject error = %v", err)
	}
	if err := subject.DeleteFolder(context.Background(), created.ID, dto.SavedQueryFolderDeleteInput{Policy: dto.SavedQueryFolderDeleteMove, Destination: "General"}); err != nil {
		t.Fatalf("DeleteFolder() move error = %v", err)
	}
	if queries.items[query.ID].Folder != "General" {
		t.Fatalf("DeleteFolder() moved query = %#v", queries.items[query.ID])
	}
	listed, err := subject.ListFolders(context.Background())
	if err != nil || len(listed) != 0 {
		t.Fatalf("ListFolders() = %#v, %v", listed, err)
	}
}

func TestSavedQueryServiceValidatesFolderInputsAndMapsConflicts(t *testing.T) {
	queries := newMemorySavedQueries()
	folders := newMemoryFolders(queries)
	subject := newSavedQueryService(queries, folders, newMemoryShares(queries))
	created, err := subject.CreateFolder(context.Background(), dto.SavedQueryFolderCreateInput{Name: "Analytics"})
	if err != nil {
		t.Fatalf("CreateFolder() error = %v", err)
	}
	if _, err := subject.CreateFolder(context.Background(), dto.SavedQueryFolderCreateInput{Name: "analytics"}); savedQueryErrorCode(err) != apperror.CodeConflict {
		t.Fatalf("CreateFolder() duplicate error = %v", err)
	}
	if _, err := subject.CreateFolder(context.Background(), dto.SavedQueryFolderCreateInput{Name: " "}); savedQueryErrorCode(err) != apperror.CodeValidation {
		t.Fatalf("CreateFolder() blank error = %v", err)
	}
	if _, err := subject.UpdateFolder(context.Background(), created.ID, dto.SavedQueryFolderUpdateInput{}); savedQueryErrorCode(err) != apperror.CodeValidation {
		t.Fatalf("UpdateFolder() empty error = %v", err)
	}
	if err := subject.DeleteFolder(context.Background(), created.ID, dto.SavedQueryFolderDeleteInput{Policy: "discard"}); savedQueryErrorCode(err) != apperror.CodeValidation {
		t.Fatalf("DeleteFolder() policy error = %v", err)
	}
}

func TestSavedQueryServiceEnablesUniqueShareRevokesAndReturnsSafePublicView(t *testing.T) {
	queries := newMemorySavedQueries()
	shares := newMemoryShares(queries)
	shares.conflicts = 1
	subject := newSavedQueryService(queries, newMemoryFolders(queries), shares)
	connectionID := "private-connection"
	createdAt := time.Date(2026, time.July, 15, 10, 0, 0, 0, time.UTC)
	query := queries.seed(ports.SavedQuery{ConnectionID: &connectionID, Folder: "Private", Title: "Safe query", SQLText: "SELECT 1", Tags: []string{"safe"}, CreatedAt: createdAt})
	expiresAt := time.Now().UTC().Add(time.Hour)

	shared, err := subject.EnableShare(context.Background(), query.ID, dto.SavedQueryShareInput{ExpiresAt: &expiresAt})
	if err != nil {
		t.Fatalf("EnableShare() error = %v", err)
	}
	if shared.Code == "" || shared.SavedQueryID != query.ID || shared.ExpiresAt == nil || len(shares.attemptedCodes) != 2 || shares.attemptedCodes[0] == shares.attemptedCodes[1] {
		t.Fatalf("EnableShare() = %#v, attempts = %#v", shared, shares.attemptedCodes)
	}
	current, err := subject.GetShare(context.Background(), query.ID)
	if err != nil || current.Code != shared.Code || current.SavedQueryID != query.ID || current.ExpiresAt == nil {
		t.Fatalf("GetShare() = %#v, %v", current, err)
	}
	public, err := subject.PublicShare(context.Background(), shared.Code)
	if err != nil {
		t.Fatalf("PublicShare() error = %v", err)
	}
	if public.Title != "Safe query" || public.SQL != "SELECT 1" || public.CreatedAt != createdAt.Format(time.RFC3339Nano) {
		t.Fatalf("PublicShare() = %#v", public)
	}
	encoded, err := json.Marshal(public)
	if err != nil {
		t.Fatalf("marshal public share: %v", err)
	}
	if strings.Contains(strings.ToLower(string(encoded)), "connection") || strings.Contains(string(encoded), connectionID) {
		t.Fatalf("public share leaked connection data: %s", encoded)
	}
	if err := subject.RevokeShare(context.Background(), query.ID); err != nil {
		t.Fatalf("RevokeShare() error = %v", err)
	}
	if _, err := subject.GetShare(context.Background(), query.ID); savedQueryErrorCode(err) != apperror.CodeNotFound {
		t.Fatalf("GetShare() revoked error = %v", err)
	}
	if _, err := subject.PublicShare(context.Background(), shared.Code); savedQueryErrorCode(err) != apperror.CodeNotFound {
		t.Fatalf("PublicShare() revoked error = %v", err)
	}
}

func TestSavedQueryServiceRejectsExpiredShareAndMissingPublicCode(t *testing.T) {
	queries := newMemorySavedQueries()
	shares := newMemoryShares(queries)
	subject := newSavedQueryService(queries, newMemoryFolders(queries), shares)
	query := queries.seed(ports.SavedQuery{Folder: "General", Title: "Query", SQLText: "SELECT 1"})
	expired := time.Now().UTC().Add(-time.Second)

	if _, err := subject.EnableShare(context.Background(), query.ID, dto.SavedQueryShareInput{ExpiresAt: &expired}); savedQueryErrorCode(err) != apperror.CodeValidation {
		t.Fatalf("EnableShare() expired error = %v", err)
	}
	if _, err := subject.PublicShare(context.Background(), "missing-code"); savedQueryErrorCode(err) != apperror.CodeNotFound {
		t.Fatalf("PublicShare() missing error = %v", err)
	}
	if _, err := subject.PublicShare(context.Background(), " "); savedQueryErrorCode(err) != apperror.CodeValidation {
		t.Fatalf("PublicShare() blank error = %v", err)
	}
}

func newSavedQueryService(queries ports.SavedQueryRepository, folders ports.SavedQueryFolderRepository, shares ports.SharedQueryRepository) *service.SavedQueryService {
	return service.NewSavedQueryService(queries, folders, shares)
}

type memorySavedQueries struct {
	items   map[string]ports.SavedQuery
	nextID  int
	listErr error
}

func newMemorySavedQueries() *memorySavedQueries {
	return &memorySavedQueries{items: make(map[string]ports.SavedQuery)}
}

func (repository *memorySavedQueries) seed(query ports.SavedQuery) ports.SavedQuery {
	repository.nextID++
	if query.ID == "" {
		query.ID = "saved-" + time.Unix(int64(repository.nextID), 0).UTC().Format("150405")
	}
	if query.Tags == nil {
		query.Tags = []string{}
	}
	if query.CreatedAt.IsZero() {
		query.CreatedAt = time.Now().UTC()
	}
	if query.UpdatedAt.IsZero() {
		query.UpdatedAt = query.CreatedAt
	}
	repository.items[query.ID] = cloneSavedQuery(query)
	return query
}

func (repository *memorySavedQueries) Create(_ context.Context, query ports.SavedQuery) (ports.SavedQuery, error) {
	return repository.seed(query), nil
}

func (repository *memorySavedQueries) Get(_ context.Context, id string) (ports.SavedQuery, error) {
	query, ok := repository.items[id]
	if !ok {
		return ports.SavedQuery{}, ports.ErrNotFound
	}
	return cloneSavedQuery(query), nil
}

func (repository *memorySavedQueries) List(context.Context) ([]ports.SavedQuery, error) {
	if repository.listErr != nil {
		return nil, repository.listErr
	}
	items := make([]ports.SavedQuery, 0, len(repository.items))
	for _, query := range repository.items {
		items = append(items, cloneSavedQuery(query))
	}
	return items, nil
}

func (repository *memorySavedQueries) Patch(_ context.Context, id string, patch ports.SavedQueryPatch) (ports.SavedQuery, error) {
	query, ok := repository.items[id]
	if !ok {
		return ports.SavedQuery{}, ports.ErrNotFound
	}
	if patch.ConnectionIDSet {
		query.ConnectionID = patch.ConnectionID
	}
	if patch.Folder != nil {
		query.Folder = *patch.Folder
	}
	if patch.Title != nil {
		query.Title = *patch.Title
	}
	if patch.SQLText != nil {
		query.SQLText = *patch.SQLText
	}
	if patch.Tags != nil {
		query.Tags = append([]string(nil), (*patch.Tags)...)
	}
	query.UpdatedAt = patch.UpdatedAt
	repository.items[id] = cloneSavedQuery(query)
	return query, nil
}

func (repository *memorySavedQueries) Delete(_ context.Context, id string) error {
	if _, ok := repository.items[id]; !ok {
		return ports.ErrNotFound
	}
	delete(repository.items, id)
	return nil
}

func (repository *memorySavedQueries) DuplicateWithUniqueTitle(_ context.Context, id string, selectTitle ports.SavedQueryTitleSelector) (ports.SavedQuery, error) {
	source, ok := repository.items[id]
	if !ok {
		return ports.SavedQuery{}, ports.ErrNotFound
	}
	titles := make([]string, 0, len(repository.items))
	for _, query := range repository.items {
		titles = append(titles, query.Title)
	}
	copy := cloneSavedQuery(source)
	copy.ID = ""
	copy.Title = selectTitle(source.Title, titles)
	copy.Favorite = false
	copy.CreatedAt = time.Now().UTC()
	copy.UpdatedAt = copy.CreatedAt
	return repository.seed(copy), nil
}

func (repository *memorySavedQueries) SetFavoriteAndGet(_ context.Context, id string, favorite bool, updatedAt time.Time) (ports.SavedQuery, error) {
	query, ok := repository.items[id]
	if !ok {
		return ports.SavedQuery{}, ports.ErrNotFound
	}
	query.Favorite = favorite
	query.UpdatedAt = updatedAt
	repository.items[id] = query
	return query, nil
}

type memoryFolders struct {
	items       map[string]ports.SavedQueryFolder
	queries     *memorySavedQueries
	nextID      int
	renameCalls int
}

func newMemoryFolders(queries *memorySavedQueries) *memoryFolders {
	return &memoryFolders{items: make(map[string]ports.SavedQueryFolder), queries: queries}
}

func (repository *memoryFolders) Create(_ context.Context, folder ports.SavedQueryFolder) (ports.SavedQueryFolder, error) {
	for _, existing := range repository.items {
		if strings.EqualFold(existing.Name, folder.Name) {
			return ports.SavedQueryFolder{}, ports.ErrConflict
		}
	}
	repository.nextID++
	if folder.ID == "" {
		folder.ID = "folder-" + time.Unix(int64(repository.nextID), 0).UTC().Format("150405")
	}
	repository.items[folder.ID] = folder
	return folder, nil
}

func (repository *memoryFolders) Ensure(ctx context.Context, folder ports.SavedQueryFolder) (ports.SavedQueryFolder, error) {
	for _, existing := range repository.items {
		if strings.EqualFold(existing.Name, folder.Name) {
			return existing, nil
		}
	}
	return repository.Create(ctx, folder)
}

func (repository *memoryFolders) Get(_ context.Context, id string) (ports.SavedQueryFolder, error) {
	folder, ok := repository.items[id]
	if !ok {
		return ports.SavedQueryFolder{}, ports.ErrNotFound
	}
	return folder, nil
}

func (repository *memoryFolders) List(context.Context) ([]ports.SavedQueryFolder, error) {
	items := make([]ports.SavedQueryFolder, 0, len(repository.items))
	for _, folder := range repository.items {
		folder.QueryCount = 0
		for _, query := range repository.queries.items {
			if query.Folder == folder.Name {
				folder.QueryCount++
			}
		}
		items = append(items, folder)
	}
	return items, nil
}

func (repository *memoryFolders) UpdateAndPropagate(_ context.Context, id string, patch ports.SavedQueryFolderPatch) (ports.SavedQueryFolder, error) {
	folder, ok := repository.items[id]
	if !ok {
		return ports.SavedQueryFolder{}, ports.ErrNotFound
	}
	oldName := folder.Name
	if patch.Name != nil {
		for existingID, existing := range repository.items {
			if existingID != id && strings.EqualFold(existing.Name, *patch.Name) {
				return ports.SavedQueryFolder{}, ports.ErrConflict
			}
		}
		folder.Name = *patch.Name
	}
	if patch.Position != nil {
		folder.Position = *patch.Position
	}
	folder.UpdatedAt = patch.UpdatedAt
	repository.items[id] = folder
	if oldName != folder.Name {
		repository.renameCalls++
		for queryID, query := range repository.queries.items {
			if query.Folder == oldName {
				query.Folder = folder.Name
				query.UpdatedAt = patch.UpdatedAt
				repository.queries.items[queryID] = query
			}
		}
	}
	return folder, nil
}

func (repository *memoryFolders) DeleteWithPolicy(_ context.Context, id string, policy ports.SavedQueryFolderDeletePolicy) error {
	folder, ok := repository.items[id]
	if !ok {
		return ports.ErrNotFound
	}
	queryIDs := make([]string, 0)
	for queryID, query := range repository.queries.items {
		if query.Folder == folder.Name {
			queryIDs = append(queryIDs, queryID)
		}
	}
	if len(queryIDs) > 0 && policy.Mode == ports.SavedQueryFolderDeleteReject {
		return ports.ErrConflict
	}
	for _, queryID := range queryIDs {
		query := repository.queries.items[queryID]
		query.Folder = policy.Destination
		query.UpdatedAt = policy.UpdatedAt
		repository.queries.items[queryID] = query
	}
	delete(repository.items, id)
	return nil
}

type memoryShares struct {
	queries        *memorySavedQueries
	items          map[string]ports.PublicSharedQuery
	bySavedQuery   map[string]string
	details        map[string]ports.SharedQuery
	conflicts      int
	attemptedCodes []string
}

func newMemoryShares(queries *memorySavedQueries) *memoryShares {
	return &memoryShares{queries: queries, items: make(map[string]ports.PublicSharedQuery), bySavedQuery: make(map[string]string), details: make(map[string]ports.SharedQuery)}
}

func (repository *memoryShares) Enable(_ context.Context, shared ports.SharedQuery) (ports.SharedQuery, error) {
	repository.attemptedCodes = append(repository.attemptedCodes, shared.ShareCode)
	if repository.conflicts > 0 {
		repository.conflicts--
		return ports.SharedQuery{}, ports.ErrConflict
	}
	query, ok := repository.queries.items[shared.SavedQueryID]
	if !ok {
		return ports.SharedQuery{}, ports.ErrNotFound
	}
	if previous := repository.bySavedQuery[shared.SavedQueryID]; previous != "" {
		delete(repository.items, previous)
	}
	repository.bySavedQuery[shared.SavedQueryID] = shared.ShareCode
	repository.details[shared.SavedQueryID] = shared
	repository.items[shared.ShareCode] = ports.PublicSharedQuery{
		Title:          query.Title,
		SQLText:        query.SQLText,
		Tags:           append([]string(nil), query.Tags...),
		QueryCreatedAt: query.CreatedAt,
		SharedAt:       shared.CreatedAt,
		ExpiresAt:      shared.ExpiresAt,
	}
	return shared, nil
}

func (repository *memoryShares) GetBySavedQuery(_ context.Context, savedQueryID string) (ports.SharedQuery, error) {
	shared, ok := repository.details[savedQueryID]
	if !ok {
		return ports.SharedQuery{}, ports.ErrNotFound
	}
	return shared, nil
}

func (repository *memoryShares) Revoke(_ context.Context, savedQueryID string) error {
	if _, ok := repository.queries.items[savedQueryID]; !ok {
		return ports.ErrNotFound
	}
	if code := repository.bySavedQuery[savedQueryID]; code != "" {
		delete(repository.items, code)
		delete(repository.bySavedQuery, savedQueryID)
		delete(repository.details, savedQueryID)
	}
	return nil
}

func (repository *memoryShares) LookupPublic(_ context.Context, code string, now time.Time) (ports.PublicSharedQuery, error) {
	shared, ok := repository.items[code]
	if !ok || shared.ExpiresAt != nil && !shared.ExpiresAt.After(now) {
		return ports.PublicSharedQuery{}, ports.ErrNotFound
	}
	return shared, nil
}

func cloneSavedQuery(query ports.SavedQuery) ports.SavedQuery {
	if query.ConnectionID != nil {
		connectionID := *query.ConnectionID
		query.ConnectionID = &connectionID
	}
	query.Tags = append([]string(nil), query.Tags...)
	return query
}

func savedQueryErrorCode(err error) string {
	var applicationError *apperror.Error
	if errors.As(err, &applicationError) {
		return applicationError.Code
	}
	return ""
}
