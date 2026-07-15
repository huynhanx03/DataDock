package service_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/huynhanx03/datadock/internal/adapters/driven/engines"
	"github.com/huynhanx03/datadock/internal/core/apperror"
	"github.com/huynhanx03/datadock/internal/core/dto"
	"github.com/huynhanx03/datadock/internal/core/entity"
	"github.com/huynhanx03/datadock/internal/core/service"
	"github.com/huynhanx03/datadock/internal/ports"
)

func TestConnectionServiceUpdatePreservesSecretsAndSanitizesProxyCredentials(t *testing.T) {
	ctx := context.Background()
	repository := newConnectionMemoryRepository()
	metadata := newConnectionMetadataMemoryRepository()
	lifecycle := newConnectionLifecycleFake()
	cipher := prefixCipher{}
	existing := connectionFixture("connection-1", "Primary")
	existing.PasswordCipher = "enc:database-secret"
	keyPath := filepath.Join(t.TempDir(), "id_ed25519")
	knownHostsPath := filepath.Join(t.TempDir(), "known_hosts")
	if err := os.WriteFile(keyPath, []byte("key"), 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}
	if err := os.WriteFile(knownHostsPath, []byte("host"), 0o600); err != nil {
		t.Fatalf("write known hosts: %v", err)
	}
	existing.SSHTunnel = entity.SSHTunnel{Enabled: true, Host: "bastion.internal", Port: 22, Username: "deploy", PrivateKeyPath: keyPath, KnownHostsPath: knownHostsPath}
	existing.SSHTunnel.PasswordCipher = "enc:ssh-secret"
	existing.ProxyURL = "socks5://proxy.local:1080"
	repository.items[existing.ID] = existing
	metadata.items[existing.ID] = ports.ConnectionMetadata{
		ConnectionID:        existing.ID,
		ProxyUsernameCipher: "enc:proxy-user",
		ProxyPasswordCipher: "enc:proxy-secret",
		Status:              entity.ConnectionStateDisconnected,
	}
	subject := service.NewConnectionService(newConnectionMemoryStore(repository, metadata), cipher, lifecycle)
	input := connectionInputFixture(existing.Name)
	input.SSHTunnel = dto.SSHTunnelInput{
		Enabled:        true,
		Host:           existing.SSHTunnel.Host,
		Port:           existing.SSHTunnel.Port,
		Username:       existing.SSHTunnel.Username,
		PrivateKeyPath: existing.SSHTunnel.PrivateKeyPath,
		KnownHostsPath: existing.SSHTunnel.KnownHostsPath,
	}
	input.ProxyURL = "socks5://proxy.local:1080"

	view, err := subject.Update(ctx, existing.ID, input)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	saved := repository.items[existing.ID]
	if saved.PasswordCipher != "enc:database-secret" || saved.SSHTunnel.PasswordCipher != "enc:ssh-secret" {
		t.Fatalf("saved secrets = %q, %q", saved.PasswordCipher, saved.SSHTunnel.PasswordCipher)
	}
	savedMetadata := metadata.items[existing.ID]
	if savedMetadata.ProxyUsernameCipher != "enc:proxy-user" || savedMetadata.ProxyPasswordCipher != "enc:proxy-secret" {
		t.Fatalf("proxy secrets = %q, %q", savedMetadata.ProxyUsernameCipher, savedMetadata.ProxyPasswordCipher)
	}
	if view.ProxyURL != "socks5://proxy.local:1080" || strings.Contains(view.ProxyURL, "proxy-user") {
		t.Fatalf("ProxyURL = %q", view.ProxyURL)
	}
	if lifecycle.invalidateCount != 0 {
		t.Fatalf("Invalidate() count = %d", lifecycle.invalidateCount)
	}
}

func TestConnectionServiceRejectsInvalidPoolSettings(t *testing.T) {
	tests := []struct {
		name   string
		change func(*dto.ConnectionInput)
	}{
		{name: "zero open", change: func(input *dto.ConnectionInput) { input.MaxOpenConns = 0; input.MaxIdleConns = 0 }},
		{name: "idle exceeds open", change: func(input *dto.ConnectionInput) { input.MaxOpenConns = 2; input.MaxIdleConns = 3 }},
		{name: "negative lifetime", change: func(input *dto.ConnectionInput) { input.ConnMaxLifetime = -1 }},
		{name: "negative idle time", change: func(input *dto.ConnectionInput) { input.ConnMaxIdleTime = -1 }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := connectionInputFixture("Primary")
			test.change(&input)
			repository := newConnectionMemoryRepository()
			metadata := newConnectionMetadataMemoryRepository()
			subject := service.NewConnectionService(newConnectionMemoryStore(repository, metadata), prefixCipher{}, newConnectionLifecycleFake())

			_, err := subject.Create(context.Background(), input)
			var appErr *apperror.Error
			if !errors.As(err, &appErr) || appErr.Code != apperror.CodeValidation {
				t.Fatalf("Create() error = %#v", err)
			}
		})
	}
}

func TestConnectionServiceAppliesConfiguredPoolDefaultsWhenProfileOmitsPool(t *testing.T) {
	repository := newConnectionMemoryRepository()
	metadata := newConnectionMetadataMemoryRepository()
	subject := service.NewConnectionServiceWithDefaults(
		newConnectionMemoryStore(repository, metadata),
		prefixCipher{},
		newConnectionLifecycleFake(),
		service.ConnectionProfileDefaults{MaxOpenConns: 24, MaxIdleConns: 8, ConnMaxLifetime: 1200, ConnMaxIdleTime: 240},
	)
	input := connectionInputFixture("Default pool")
	input.MaxOpenConns = 0
	input.MaxIdleConns = 0
	input.ConnMaxLifetime = 0
	input.ConnMaxIdleTime = 0

	created, err := subject.Create(context.Background(), input)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.MaxOpenConns != 24 || created.MaxIdleConns != 8 || created.ConnMaxLifetime != 1200 || created.ConnMaxIdleTime != 240 {
		t.Fatalf("Create() pool = %#v", created)
	}
}

func TestConnectionServiceTestsDraftWithoutPersistence(t *testing.T) {
	repository := newConnectionMemoryRepository()
	metadata := newConnectionMetadataMemoryRepository()
	lifecycle := newConnectionLifecycleFake()
	lifecycle.testStatus = entity.ConnectionRuntimeStatus{State: entity.ConnectionStateConnected, LatencyMS: 17}
	subject := service.NewConnectionService(newConnectionMemoryStore(repository, metadata), prefixCipher{}, lifecycle)
	input := connectionInputFixture("Draft")
	input.Password = "database-secret"
	input.ProxyURL = "socks5://proxy-user:proxy-secret@proxy.local:1080"

	result, err := subject.TestDraft(context.Background(), input)
	if err != nil {
		t.Fatalf("TestDraft() error = %v", err)
	}
	if !result.OK || result.LatencyMS != 17 {
		t.Fatalf("TestDraft() result = %#v", result)
	}
	if len(repository.items) != 0 || len(metadata.items) != 0 {
		t.Fatalf("draft test persisted records = %d, %d", len(repository.items), len(metadata.items))
	}
	if lifecycle.lastPassword != "database-secret" || lifecycle.lastConnection.ProxyUsername != "proxy-user" || lifecycle.lastConnection.ProxyPassword != "proxy-secret" {
		t.Fatalf("draft secrets = %#v, %q", lifecycle.lastConnection, lifecycle.lastPassword)
	}
	if lifecycle.lastConnection.ProxyURL != "socks5://proxy.local:1080" {
		t.Fatalf("draft proxy URL = %q", lifecycle.lastConnection.ProxyURL)
	}
}

func TestConnectionServiceDuplicateUsesNextAvailableName(t *testing.T) {
	repository := newConnectionMemoryRepository()
	metadata := newConnectionMetadataMemoryRepository()
	lifecycle := newConnectionLifecycleFake()
	primary := connectionFixture("connection-1", "Primary")
	primary.Favorite = true
	repository.items[primary.ID] = primary
	repository.items["connection-2"] = connectionFixture("connection-2", "Primary copy")
	metadata.items[primary.ID] = ports.ConnectionMetadata{ConnectionID: primary.ID, ProxyUsernameCipher: "enc:user", ProxyPasswordCipher: "enc:secret"}
	subject := service.NewConnectionService(newConnectionMemoryStore(repository, metadata), prefixCipher{}, lifecycle)

	duplicate, err := subject.Duplicate(context.Background(), primary.ID)
	if err != nil {
		t.Fatalf("Duplicate() error = %v", err)
	}
	if duplicate.Name != "Primary copy 2" || duplicate.Favorite || duplicate.ID == primary.ID {
		t.Fatalf("Duplicate() = %#v", duplicate)
	}
	duplicateMetadata := metadata.items[duplicate.ID]
	if duplicateMetadata.ProxyUsernameCipher != "enc:user" || duplicateMetadata.ProxyPasswordCipher != "enc:secret" {
		t.Fatalf("duplicate metadata = %#v", duplicateMetadata)
	}
}

func TestConnectionServiceInvalidatesOnlyMaterialUpdates(t *testing.T) {
	repository := newConnectionMemoryRepository()
	metadata := newConnectionMetadataMemoryRepository()
	lifecycle := newConnectionLifecycleFake()
	existing := connectionFixture("connection-1", "Primary")
	repository.items[existing.ID] = existing
	metadata.items[existing.ID] = ports.ConnectionMetadata{ConnectionID: existing.ID}
	subject := service.NewConnectionService(newConnectionMemoryStore(repository, metadata), prefixCipher{}, lifecycle)

	nameOnly := connectionInputFixture("Renamed")
	if _, err := subject.Update(context.Background(), existing.ID, nameOnly); err != nil {
		t.Fatalf("name Update() error = %v", err)
	}
	if lifecycle.invalidateCount != 0 {
		t.Fatalf("name update invalidations = %d", lifecycle.invalidateCount)
	}

	material := connectionInputFixture("Renamed")
	material.Host = "replica.internal"
	if _, err := subject.Update(context.Background(), existing.ID, material); err != nil {
		t.Fatalf("material Update() error = %v", err)
	}
	if lifecycle.invalidateCount != 1 || lifecycle.lastInvalidatedID != existing.ID {
		t.Fatalf("material invalidation = %d, %q", lifecycle.invalidateCount, lifecycle.lastInvalidatedID)
	}
}

func TestConnectionServicePatchPreservesOmittedFields(t *testing.T) {
	repository := newConnectionMemoryRepository()
	metadata := newConnectionMetadataMemoryRepository()
	lifecycle := newConnectionLifecycleFake()
	existing := connectionFixture("connection-1", "Primary")
	repository.items[existing.ID] = existing
	metadata.items[existing.ID] = ports.ConnectionMetadata{ConnectionID: existing.ID, Status: entity.ConnectionStateDisconnected}
	subject := service.NewConnectionService(newConnectionMemoryStore(repository, metadata), prefixCipher{}, lifecycle)
	name := "Renamed"

	updated, err := subject.Patch(context.Background(), existing.ID, dto.ConnectionPatchInput{Name: &name})
	if err != nil {
		t.Fatalf("Patch() error = %v", err)
	}
	if updated.Name != name || updated.ConnMaxIdleTime != existing.ConnMaxIdleTime || updated.MaxOpenConns != existing.MaxOpenConns || updated.ProxyURL != existing.ProxyURL {
		t.Fatalf("Patch() = %#v", updated)
	}
	if lifecycle.invalidateCount != 0 {
		t.Fatalf("Patch() invalidations = %d", lifecycle.invalidateCount)
	}
}

func TestConnectionPoolRegistryRejectsInvalidatedAndRemovedProfiles(t *testing.T) {
	ctx := context.Background()
	manager := engines.NewManager()
	defer manager.Close()
	oldProfile := connectionFixture("connection-1", "Primary")
	newProfile := oldProfile
	newProfile.Host = "replica.internal"

	if err := manager.Invalidate(ctx, newProfile, "new-secret"); err != nil {
		t.Fatalf("Invalidate() error = %v", err)
	}
	if _, err := manager.Connect(ctx, oldProfile, "old-secret"); applicationErrorCode(err) != apperror.CodeConflict {
		t.Fatalf("stale Connect() error = %#v", err)
	}
	if err := manager.Remove(newProfile.ID); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	if _, err := manager.Connect(ctx, newProfile, "new-secret"); applicationErrorCode(err) != apperror.CodeConflict {
		t.Fatalf("removed Connect() error = %#v", err)
	}
}

func TestConnectionServiceConnectDisconnectAndFavoritePersistState(t *testing.T) {
	ctx := context.Background()
	repository := newConnectionMemoryRepository()
	metadata := newConnectionMetadataMemoryRepository()
	lifecycle := newConnectionLifecycleFake()
	connectedAt := time.Date(2026, time.July, 15, 9, 30, 0, 0, time.UTC)
	lifecycle.connectStatus = entity.ConnectionRuntimeStatus{State: entity.ConnectionStateConnected, LatencyMS: 12, LastConnectedAt: &connectedAt}
	connection := connectionFixture("connection-1", "Primary")
	connection.PasswordCipher = "enc:database-secret"
	connection.SSHTunnel = entity.SSHTunnel{Enabled: true, Host: "bastion.internal", Port: 22, Username: "deploy"}
	connection.SSHTunnel.PasswordCipher = "enc:ssh-secret"
	repository.items[connection.ID] = connection
	metadata.items[connection.ID] = ports.ConnectionMetadata{ConnectionID: connection.ID, ProxyUsernameCipher: "enc:proxy-user", ProxyPasswordCipher: "enc:proxy-secret"}
	subject := service.NewConnectionService(newConnectionMemoryStore(repository, metadata), prefixCipher{}, lifecycle)

	connected, err := subject.Connect(ctx, connection.ID)
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	if connected.State != entity.ConnectionStateConnected || connected.LatencyMS != 12 {
		t.Fatalf("Connect() = %#v", connected)
	}
	if lifecycle.lastPassword != "database-secret" || lifecycle.lastConnection.SSHTunnel.Password != "ssh-secret" || lifecycle.lastConnection.ProxyUsername != "proxy-user" || lifecycle.lastConnection.ProxyPassword != "proxy-secret" {
		t.Fatalf("connect secrets = %#v, %q", lifecycle.lastConnection, lifecycle.lastPassword)
	}
	connectedMetadata := metadata.items[connection.ID]
	if connectedMetadata.Status != entity.ConnectionStateConnected || connectedMetadata.LastConnectedAt == nil || !connectedMetadata.LastConnectedAt.Equal(connectedAt) {
		t.Fatalf("connected metadata = %#v", connectedMetadata)
	}

	favorite, err := subject.SetFavorite(ctx, connection.ID, true)
	if err != nil {
		t.Fatalf("SetFavorite() error = %v", err)
	}
	if !favorite.Favorite || !repository.items[connection.ID].Favorite {
		t.Fatalf("favorite = %#v", favorite)
	}

	disconnected, err := subject.Disconnect(ctx, connection.ID)
	if err != nil {
		t.Fatalf("Disconnect() error = %v", err)
	}
	if disconnected.State != entity.ConnectionStateDisconnected || lifecycle.disconnectCount != 1 {
		t.Fatalf("Disconnect() = %#v, calls = %d", disconnected, lifecycle.disconnectCount)
	}
	disconnectedMetadata := metadata.items[connection.ID]
	if disconnectedMetadata.Status != entity.ConnectionStateDisconnected || disconnectedMetadata.LastDisconnectedAt == nil {
		t.Fatalf("disconnected metadata = %#v", disconnectedMetadata)
	}
}

func TestConnectionServiceDeleteCleansLifecycleBeforeRecord(t *testing.T) {
	order := make([]string, 0, 2)
	repository := newConnectionMemoryRepository()
	repository.order = &order
	metadata := newConnectionMetadataMemoryRepository()
	lifecycle := newConnectionLifecycleFake()
	lifecycle.order = &order
	connection := connectionFixture("connection-1", "Primary")
	repository.items[connection.ID] = connection
	metadata.items[connection.ID] = ports.ConnectionMetadata{ConnectionID: connection.ID}
	subject := service.NewConnectionService(newConnectionMemoryStore(repository, metadata), prefixCipher{}, lifecycle)

	if err := subject.Delete(context.Background(), connection.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if strings.Join(order, ",") != "delete,remove" {
		t.Fatalf("cleanup order = %v", order)
	}
}

type connectionMemoryRepository struct {
	items map[string]entity.Connection
	order *[]string
}

func newConnectionMemoryRepository() *connectionMemoryRepository {
	return &connectionMemoryRepository{items: make(map[string]entity.Connection)}
}

func (repository *connectionMemoryRepository) Create(_ context.Context, connection entity.Connection) (entity.Connection, error) {
	repository.items[connection.ID] = connection
	return connection, nil
}

func (repository *connectionMemoryRepository) List(_ context.Context, workspaceID string) ([]entity.Connection, error) {
	connections := make([]entity.Connection, 0, len(repository.items))
	for _, connection := range repository.items {
		if workspaceID == "" || connection.WorkspaceID == workspaceID {
			connections = append(connections, connection)
		}
	}
	return connections, nil
}

func (repository *connectionMemoryRepository) Get(_ context.Context, id string) (entity.Connection, error) {
	connection, ok := repository.items[id]
	if !ok {
		return entity.Connection{}, ports.ErrNotFound
	}
	return connection, nil
}

func (repository *connectionMemoryRepository) Update(_ context.Context, connection entity.Connection) (entity.Connection, error) {
	if _, ok := repository.items[connection.ID]; !ok {
		return entity.Connection{}, ports.ErrNotFound
	}
	repository.items[connection.ID] = connection
	return connection, nil
}

func (repository *connectionMemoryRepository) Delete(_ context.Context, id string) error {
	if repository.order != nil {
		*repository.order = append(*repository.order, "delete")
	}
	if _, ok := repository.items[id]; !ok {
		return ports.ErrNotFound
	}
	delete(repository.items, id)
	return nil
}

type connectionMetadataMemoryRepository struct {
	items map[string]ports.ConnectionMetadata
}

type connectionMemoryStore struct {
	*connectionMemoryRepository
	*connectionMetadataMemoryRepository
}

func newConnectionMemoryStore(repository *connectionMemoryRepository, metadata *connectionMetadataMemoryRepository) *connectionMemoryStore {
	return &connectionMemoryStore{connectionMemoryRepository: repository, connectionMetadataMemoryRepository: metadata}
}

func (store *connectionMemoryStore) CreateRecord(ctx context.Context, record ports.ConnectionRecord) (ports.ConnectionRecord, error) {
	connection, err := store.Create(ctx, record.Connection)
	if err != nil {
		return ports.ConnectionRecord{}, err
	}
	record.Connection = connection
	record.Metadata.ConnectionID = connection.ID
	if err := store.UpdateMetadata(ctx, record.Metadata); err != nil {
		return ports.ConnectionRecord{}, err
	}
	return record, nil
}

func (store *connectionMemoryStore) ListRecords(ctx context.Context, workspaceID string) ([]ports.ConnectionRecord, error) {
	connections, err := store.List(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	records := make([]ports.ConnectionRecord, 0, len(connections))
	for _, connection := range connections {
		metadata, err := store.GetMetadata(ctx, connection.ID)
		if err != nil {
			return nil, err
		}
		records = append(records, ports.ConnectionRecord{Connection: connection, Metadata: metadata})
	}
	return records, nil
}

func (store *connectionMemoryStore) GetRecord(ctx context.Context, id string) (ports.ConnectionRecord, error) {
	connection, err := store.Get(ctx, id)
	if err != nil {
		return ports.ConnectionRecord{}, err
	}
	metadata, err := store.GetMetadata(ctx, id)
	if err != nil {
		return ports.ConnectionRecord{}, err
	}
	return ports.ConnectionRecord{Connection: connection, Metadata: metadata}, nil
}

func (store *connectionMemoryStore) UpdateRecord(ctx context.Context, record ports.ConnectionRecord) (ports.ConnectionRecord, error) {
	connection, err := store.Update(ctx, record.Connection)
	if err != nil {
		return ports.ConnectionRecord{}, err
	}
	record.Connection = connection
	record.Metadata.ConnectionID = connection.ID
	if err := store.UpdateMetadata(ctx, record.Metadata); err != nil {
		return ports.ConnectionRecord{}, err
	}
	return record, nil
}

func newConnectionMetadataMemoryRepository() *connectionMetadataMemoryRepository {
	return &connectionMetadataMemoryRepository{items: make(map[string]ports.ConnectionMetadata)}
}

func (repository *connectionMetadataMemoryRepository) GetMetadata(_ context.Context, id string) (ports.ConnectionMetadata, error) {
	metadata, ok := repository.items[id]
	if !ok {
		return ports.ConnectionMetadata{}, ports.ErrNotFound
	}
	return metadata, nil
}

func (repository *connectionMetadataMemoryRepository) UpdateMetadata(_ context.Context, metadata ports.ConnectionMetadata) error {
	repository.items[metadata.ConnectionID] = metadata
	return nil
}

type prefixCipher struct{}

func (prefixCipher) Encrypt(value string) (string, error) {
	return "enc:" + value, nil
}

func (prefixCipher) Decrypt(value string) (string, error) {
	if !strings.HasPrefix(value, "enc:") {
		return "", errors.New("invalid ciphertext")
	}
	return strings.TrimPrefix(value, "enc:"), nil
}

type connectionLifecycleFake struct {
	testStatus        entity.ConnectionRuntimeStatus
	connectStatus     entity.ConnectionRuntimeStatus
	statuses          map[string]entity.ConnectionRuntimeStatus
	lastConnection    entity.Connection
	lastPassword      string
	disconnectCount   int
	invalidateCount   int
	lastInvalidatedID string
	order             *[]string
}

func newConnectionLifecycleFake() *connectionLifecycleFake {
	return &connectionLifecycleFake{
		testStatus:    entity.ConnectionRuntimeStatus{State: entity.ConnectionStateConnected, LatencyMS: 1},
		connectStatus: entity.ConnectionRuntimeStatus{State: entity.ConnectionStateConnected, LatencyMS: 1},
		statuses:      make(map[string]entity.ConnectionRuntimeStatus),
	}
}

func (lifecycle *connectionLifecycleFake) TestDraft(_ context.Context, connection entity.Connection, password string) (entity.ConnectionRuntimeStatus, error) {
	lifecycle.lastConnection = connection
	lifecycle.lastPassword = password
	return lifecycle.testStatus, nil
}

func (lifecycle *connectionLifecycleFake) Connect(_ context.Context, connection entity.Connection, password string) (entity.ConnectionRuntimeStatus, error) {
	lifecycle.lastConnection = connection
	lifecycle.lastPassword = password
	lifecycle.statuses[connection.ID] = lifecycle.connectStatus
	return lifecycle.connectStatus, nil
}

func (lifecycle *connectionLifecycleFake) Disconnect(_ context.Context, connectionID string) error {
	lifecycle.disconnectCount++
	lifecycle.statuses[connectionID] = entity.ConnectionRuntimeStatus{State: entity.ConnectionStateDisconnected}
	return nil
}

func (lifecycle *connectionLifecycleFake) Status(connectionID string) entity.ConnectionRuntimeStatus {
	status, ok := lifecycle.statuses[connectionID]
	if !ok {
		return entity.ConnectionRuntimeStatus{State: entity.ConnectionStateDisconnected}
	}
	return status
}

func (lifecycle *connectionLifecycleFake) Invalidate(_ context.Context, connection entity.Connection, _ string) error {
	if lifecycle.order != nil {
		*lifecycle.order = append(*lifecycle.order, "invalidate")
	}
	lifecycle.invalidateCount++
	lifecycle.lastInvalidatedID = connection.ID
	delete(lifecycle.statuses, connection.ID)
	return nil
}

func (lifecycle *connectionLifecycleFake) Remove(connectionID string) error {
	if lifecycle.order != nil {
		*lifecycle.order = append(*lifecycle.order, "remove")
	}
	delete(lifecycle.statuses, connectionID)
	return nil
}

func (lifecycle *connectionLifecycleFake) Close() error {
	return nil
}

func connectionFixture(id, name string) entity.Connection {
	now := time.Date(2026, time.July, 15, 8, 0, 0, 0, time.UTC)
	return entity.Connection{
		ID:              id,
		WorkspaceID:     "workspace-1",
		Name:            name,
		Engine:          entity.EnginePostgreSQL,
		Host:            "database.internal",
		Port:            5432,
		Database:        "app",
		Username:        "app",
		SSLMode:         entity.SSLModeDisable,
		ProxyURL:        "socks5://proxy.local:1080",
		SSHTunnel:       entity.SSHTunnel{},
		AutoReconnect:   true,
		MaxOpenConns:    10,
		MaxIdleConns:    5,
		ConnMaxLifetime: 1800,
		ConnMaxIdleTime: 300,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
}

func connectionInputFixture(name string) dto.ConnectionInput {
	return dto.ConnectionInput{
		WorkspaceID:     "workspace-1",
		Name:            name,
		Engine:          entity.EnginePostgreSQL,
		Host:            "database.internal",
		Port:            5432,
		Database:        "app",
		Username:        "app",
		SSLMode:         entity.SSLModeDisable,
		ProxyURL:        "socks5://proxy.local:1080",
		SSHTunnel:       dto.SSHTunnelInput{},
		AutoReconnect:   true,
		MaxOpenConns:    10,
		MaxIdleConns:    5,
		ConnMaxLifetime: 1800,
		ConnMaxIdleTime: 300,
	}
}

func applicationErrorCode(err error) string {
	var appErr *apperror.Error
	if errors.As(err, &appErr) {
		return appErr.Code
	}
	return ""
}
