package service

import (
	"context"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/huynhanx03/datadock/internal/core/apperror"
	"github.com/huynhanx03/datadock/internal/core/dto"
	"github.com/huynhanx03/datadock/internal/core/entity"
	"github.com/huynhanx03/datadock/internal/ports"
)

var (
	ErrConnectionNameRequired = errors.New("connection name is required")
	ErrConnectionHostRequired = errors.New("connection host is required")
	ErrUnsupportedEngine      = errors.New("database engine is not supported")
	ErrInvalidSSLProfile      = errors.New("invalid SSL connection profile")
	ErrInvalidProxyProfile    = errors.New("invalid proxy connection profile")
	ErrInvalidSSHTunnel       = errors.New("invalid SSH tunnel profile")
	ErrInvalidPoolProfile     = errors.New("invalid connection pool profile")
)

type ConnectionService struct {
	store     ports.ConnectionStore
	cipher    ports.SecretCipher
	lifecycle ports.ConnectionLifecycle
	defaults  ConnectionProfileDefaults
}

type ConnectionProfileDefaults struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime int
	ConnMaxIdleTime int
}

func NewConnectionService(store ports.ConnectionStore, cipher ports.SecretCipher, lifecycle ports.ConnectionLifecycle) *ConnectionService {
	return &ConnectionService{store: store, cipher: cipher, lifecycle: lifecycle}
}

func NewConnectionServiceWithDefaults(store ports.ConnectionStore, cipher ports.SecretCipher, lifecycle ports.ConnectionLifecycle, defaults ConnectionProfileDefaults) *ConnectionService {
	return &ConnectionService{store: store, cipher: cipher, lifecycle: lifecycle, defaults: defaults}
}

func (service *ConnectionService) Create(ctx context.Context, input dto.ConnectionInput) (dto.ConnectionView, error) {
	connection, metadata, err := service.connectionFromInput(entity.Connection{ID: uuid.NewString()}, ports.ConnectionMetadata{}, input, false)
	if err != nil {
		return dto.ConnectionView{}, err
	}
	metadata.ConnectionID = connection.ID
	metadata.Status = entity.ConnectionStateDisconnected
	created, err := service.store.CreateRecord(ctx, ports.ConnectionRecord{Connection: connection, Metadata: metadata})
	if err != nil {
		return dto.ConnectionView{}, serviceError(err)
	}
	return service.recordView(created), nil
}

func (service *ConnectionService) List(ctx context.Context, workspaceID string) ([]dto.ConnectionView, error) {
	records, err := service.store.ListRecords(ctx, workspaceID)
	if err != nil {
		return nil, serviceError(err)
	}
	views := make([]dto.ConnectionView, 0, len(records))
	for _, record := range records {
		views = append(views, service.recordView(record))
	}
	return views, nil
}

func (service *ConnectionService) Get(ctx context.Context, id string) (dto.ConnectionView, error) {
	record, err := service.store.GetRecord(ctx, id)
	if err != nil {
		return dto.ConnectionView{}, serviceError(err)
	}
	return service.recordView(record), nil
}

func (service *ConnectionService) Update(ctx context.Context, id string, input dto.ConnectionInput) (dto.ConnectionView, error) {
	record, err := service.store.GetRecord(ctx, id)
	if err != nil {
		return dto.ConnectionView{}, serviceError(err)
	}
	return service.updateRecord(ctx, record, input)
}

func (service *ConnectionService) Patch(ctx context.Context, id string, patch dto.ConnectionPatchInput) (dto.ConnectionView, error) {
	record, err := service.store.GetRecord(ctx, id)
	if err != nil {
		return dto.ConnectionView{}, serviceError(err)
	}
	input := connectionInputFromRecord(record.Connection)
	applyConnectionPatch(&input, patch)
	return service.updateRecord(ctx, record, input)
}

func (service *ConnectionService) updateRecord(ctx context.Context, record ports.ConnectionRecord, input dto.ConnectionInput) (dto.ConnectionView, error) {
	connection, metadata, err := service.connectionFromInput(record.Connection, record.Metadata, input, true)
	if err != nil {
		return dto.ConnectionView{}, err
	}
	materialChanged := materialConnectionChanged(record.Connection, connection, record.Metadata, metadata)
	var hydrated entity.Connection
	var password string
	if materialChanged {
		hydrated, password, err = service.decryptConnection(connection, metadata)
		if err != nil {
			return dto.ConnectionView{}, err
		}
		metadata.Status = entity.ConnectionStateDisconnected
		metadata.LastErrorCode = ""
	}
	updated, err := service.store.UpdateRecord(ctx, ports.ConnectionRecord{Connection: connection, Metadata: metadata})
	if err != nil {
		return dto.ConnectionView{}, serviceError(err)
	}
	if materialChanged {
		if err := service.lifecycle.Invalidate(ctx, hydrated, password); err != nil {
			return dto.ConnectionView{}, serviceError(err)
		}
	}
	return service.recordView(updated), nil
}

func (service *ConnectionService) Duplicate(ctx context.Context, id string) (dto.ConnectionView, error) {
	record, err := service.store.GetRecord(ctx, id)
	if err != nil {
		return dto.ConnectionView{}, serviceError(err)
	}
	connections, err := service.store.List(ctx, record.Connection.WorkspaceID)
	if err != nil {
		return dto.ConnectionView{}, serviceError(err)
	}
	now := time.Now().UTC()
	existing := record.Connection
	existing.ID = uuid.NewString()
	existing.Name = duplicateConnectionName(existing.Name, connections)
	existing.Favorite = false
	existing.CreatedAt = now
	existing.UpdatedAt = now
	metadata := ports.ConnectionMetadata{
		ConnectionID:        existing.ID,
		ProxyUsernameCipher: record.Metadata.ProxyUsernameCipher,
		ProxyPasswordCipher: record.Metadata.ProxyPasswordCipher,
		Status:              entity.ConnectionStateDisconnected,
	}
	created, err := service.store.CreateRecord(ctx, ports.ConnectionRecord{Connection: existing, Metadata: metadata})
	if err != nil {
		return dto.ConnectionView{}, serviceError(err)
	}
	return service.recordView(created), nil
}

func (service *ConnectionService) Delete(ctx context.Context, id string) error {
	if _, err := service.store.GetRecord(ctx, id); err != nil {
		return serviceError(err)
	}
	if err := service.store.Delete(ctx, id); err != nil {
		return serviceError(err)
	}
	return serviceError(service.lifecycle.Remove(id))
}

func (service *ConnectionService) Test(ctx context.Context, id string) error {
	_, err := service.TestSaved(ctx, id)
	return err
}

func (service *ConnectionService) TestSaved(ctx context.Context, id string) (dto.ConnectionTestResult, error) {
	connection, password, err := service.hydratedConnection(ctx, id)
	if err != nil {
		return dto.ConnectionTestResult{}, err
	}
	status, err := service.lifecycle.TestDraft(ctx, connection, password)
	if err != nil {
		return dto.ConnectionTestResult{}, err
	}
	return connectionTestResult(status), nil
}

func (service *ConnectionService) TestDraft(ctx context.Context, input dto.ConnectionInput) (dto.ConnectionTestResult, error) {
	connection, metadata, err := service.connectionFromInput(entity.Connection{ID: uuid.NewString()}, ports.ConnectionMetadata{}, input, false)
	if err != nil {
		return dto.ConnectionTestResult{}, err
	}
	connection, password, err := service.decryptConnection(connection, metadata)
	if err != nil {
		return dto.ConnectionTestResult{}, err
	}
	status, err := service.lifecycle.TestDraft(ctx, connection, password)
	if err != nil {
		return dto.ConnectionTestResult{}, err
	}
	return connectionTestResult(status), nil
}

func (service *ConnectionService) Connect(ctx context.Context, id string) (dto.ConnectionView, error) {
	record, err := service.store.GetRecord(ctx, id)
	if err != nil {
		return dto.ConnectionView{}, serviceError(err)
	}
	connection, password, err := service.decryptConnection(record.Connection, record.Metadata)
	if err != nil {
		return dto.ConnectionView{}, err
	}
	metadata := record.Metadata
	status, connectErr := service.lifecycle.Connect(ctx, connection, password)
	metadata.Status = status.State
	metadata.LastErrorCode = status.LastErrorCode
	if status.LastConnectedAt != nil {
		metadata.LastConnectedAt = status.LastConnectedAt
	}
	if connectErr != nil && metadata.Status == "" {
		metadata.Status = entity.ConnectionStateError
	}
	record.Metadata = metadata
	updated, err := service.store.UpdateRecord(ctx, record)
	if err != nil {
		_ = service.lifecycle.Disconnect(context.Background(), id)
		return dto.ConnectionView{}, serviceError(err)
	}
	if connectErr != nil {
		return dto.ConnectionView{}, connectErr
	}
	return service.recordView(updated), nil
}

func (service *ConnectionService) Disconnect(ctx context.Context, id string) (dto.ConnectionView, error) {
	record, err := service.store.GetRecord(ctx, id)
	if err != nil {
		return dto.ConnectionView{}, serviceError(err)
	}
	if err := service.lifecycle.Disconnect(ctx, id); err != nil {
		return dto.ConnectionView{}, serviceError(err)
	}
	now := time.Now().UTC()
	record.Metadata.Status = entity.ConnectionStateDisconnected
	record.Metadata.LastDisconnectedAt = &now
	record.Metadata.LastErrorCode = ""
	updated, err := service.store.UpdateRecord(ctx, record)
	if err != nil {
		return dto.ConnectionView{}, serviceError(err)
	}
	return service.recordView(updated), nil
}

func (service *ConnectionService) SetFavorite(ctx context.Context, id string, favorite bool) (dto.ConnectionView, error) {
	record, err := service.store.GetRecord(ctx, id)
	if err != nil {
		return dto.ConnectionView{}, serviceError(err)
	}
	record.Connection.Favorite = favorite
	record.Connection.UpdatedAt = time.Now().UTC()
	updated, err := service.store.UpdateRecord(ctx, record)
	if err != nil {
		return dto.ConnectionView{}, serviceError(err)
	}
	return service.recordView(updated), nil
}

func (service *ConnectionService) Status(ctx context.Context, id string) (entity.ConnectionRuntimeStatus, error) {
	record, err := service.store.GetRecord(ctx, id)
	if err != nil {
		return entity.ConnectionRuntimeStatus{}, serviceError(err)
	}
	return mergeRuntimeStatus(service.lifecycle.Status(id), record.Metadata), nil
}

func (service *ConnectionService) connectionFromInput(current entity.Connection, metadata ports.ConnectionMetadata, input dto.ConnectionInput, update bool) (entity.Connection, ports.ConnectionMetadata, error) {
	if input.MaxOpenConns == 0 && input.MaxIdleConns == 0 && input.ConnMaxLifetime == 0 && input.ConnMaxIdleTime == 0 && service.defaults.MaxOpenConns > 0 {
		input.MaxOpenConns = service.defaults.MaxOpenConns
		input.MaxIdleConns = service.defaults.MaxIdleConns
		input.ConnMaxLifetime = service.defaults.ConnMaxLifetime
		input.ConnMaxIdleTime = service.defaults.ConnMaxIdleTime
	}
	if !input.Engine.Valid() {
		return entity.Connection{}, ports.ConnectionMetadata{}, validationError(ErrUnsupportedEngine)
	}
	current.WorkspaceID = strings.TrimSpace(input.WorkspaceID)
	current.Name = strings.TrimSpace(input.Name)
	current.Host = strings.TrimSpace(input.Host)
	if current.Name == "" {
		return entity.Connection{}, ports.ConnectionMetadata{}, validationError(ErrConnectionNameRequired)
	}
	if current.Host == "" {
		return entity.Connection{}, ports.ConnectionMetadata{}, validationError(ErrConnectionHostRequired)
	}
	current.Engine = input.Engine
	current.Port = input.Port
	if current.Port == 0 {
		current.Port = current.DefaultPort()
	}
	current.Database = strings.TrimSpace(input.Database)
	current.Username = strings.TrimSpace(input.Username)
	current.SSLMode = input.SSLMode
	if current.SSLMode == "" {
		current.SSLMode = entity.SSLModeDisable
	}
	current.SSLCAPath = strings.TrimSpace(input.SSLCAPath)
	current.SSLCertPath = strings.TrimSpace(input.SSLCertPath)
	current.SSLKeyPath = strings.TrimSpace(input.SSLKeyPath)
	proxyURL, proxyUsername, proxyPassword, proxyCredentialsProvided, err := parseProxyURL(input.ProxyURL)
	if err != nil {
		return entity.Connection{}, ports.ConnectionMetadata{}, validationError(ErrInvalidProxyProfile)
	}
	current.ProxyURL = proxyURL
	current.SSHTunnel.Enabled = input.SSHTunnel.Enabled
	current.SSHTunnel.Host = strings.TrimSpace(input.SSHTunnel.Host)
	current.SSHTunnel.Port = input.SSHTunnel.Port
	current.SSHTunnel.Username = strings.TrimSpace(input.SSHTunnel.Username)
	current.SSHTunnel.PrivateKeyPath = strings.TrimSpace(input.SSHTunnel.PrivateKeyPath)
	current.SSHTunnel.KnownHostsPath = strings.TrimSpace(input.SSHTunnel.KnownHostsPath)
	current.ReadOnly = input.ReadOnly
	current.AutoReconnect = input.AutoReconnect
	current.MaxOpenConns = input.MaxOpenConns
	current.MaxIdleConns = input.MaxIdleConns
	current.ConnMaxLifetime = input.ConnMaxLifetime
	current.ConnMaxIdleTime = input.ConnMaxIdleTime
	if input.ClearPassword {
		current.PasswordCipher = ""
	} else if input.Password != "" {
		current.PasswordCipher, err = service.cipher.Encrypt(input.Password)
		if err != nil {
			return entity.Connection{}, ports.ConnectionMetadata{}, serviceError(err)
		}
	}
	if input.SSHTunnel.ClearPassword {
		current.SSHTunnel.PasswordCipher = ""
	} else if input.SSHTunnel.Password != "" {
		current.SSHTunnel.PasswordCipher, err = service.cipher.Encrypt(input.SSHTunnel.Password)
		if err != nil {
			return entity.Connection{}, ports.ConnectionMetadata{}, serviceError(err)
		}
	}
	if input.ClearProxyAuth || current.ProxyURL == "" {
		metadata.ProxyUsernameCipher = ""
		metadata.ProxyPasswordCipher = ""
	} else if proxyCredentialsProvided {
		metadata.ProxyUsernameCipher, err = encryptOptional(service.cipher, proxyUsername)
		if err != nil {
			return entity.Connection{}, ports.ConnectionMetadata{}, serviceError(err)
		}
		metadata.ProxyPasswordCipher, err = encryptOptional(service.cipher, proxyPassword)
		if err != nil {
			return entity.Connection{}, ports.ConnectionMetadata{}, serviceError(err)
		}
	}
	if err := validateConnectionProfile(current, input); err != nil {
		return entity.Connection{}, ports.ConnectionMetadata{}, validationError(err)
	}
	if !update {
		now := time.Now().UTC()
		current.CreatedAt = now
	}
	current.UpdatedAt = time.Now().UTC()
	return current, metadata, nil
}

func (service *ConnectionService) hydratedConnection(ctx context.Context, id string) (entity.Connection, string, error) {
	record, err := service.store.GetRecord(ctx, id)
	if err != nil {
		return entity.Connection{}, "", serviceError(err)
	}
	return service.decryptConnection(record.Connection, record.Metadata)
}

func (service *ConnectionService) decryptConnection(connection entity.Connection, metadata ports.ConnectionMetadata) (entity.Connection, string, error) {
	password, err := decryptOptional(service.cipher, connection.PasswordCipher)
	if err != nil {
		return entity.Connection{}, "", serviceError(err)
	}
	connection.SSHTunnel.Password, err = decryptOptional(service.cipher, connection.SSHTunnel.PasswordCipher)
	if err != nil {
		return entity.Connection{}, "", serviceError(err)
	}
	connection.ProxyUsername, err = decryptOptional(service.cipher, metadata.ProxyUsernameCipher)
	if err != nil {
		return entity.Connection{}, "", serviceError(err)
	}
	connection.ProxyPassword, err = decryptOptional(service.cipher, metadata.ProxyPasswordCipher)
	if err != nil {
		return entity.Connection{}, "", serviceError(err)
	}
	return connection, password, nil
}

func (service *ConnectionService) recordView(record ports.ConnectionRecord) dto.ConnectionView {
	status := mergeRuntimeStatus(service.lifecycle.Status(record.Connection.ID), record.Metadata)
	return connectionView(record.Connection, record.Metadata, status)
}

func connectionView(connection entity.Connection, metadata ports.ConnectionMetadata, status entity.ConnectionRuntimeStatus) dto.ConnectionView {
	return dto.ConnectionView{
		ConnectionRuntimeStatus: status,
		ID:                      connection.ID,
		WorkspaceID:             connection.WorkspaceID,
		Name:                    connection.Name,
		Engine:                  connection.Engine,
		Host:                    connection.Host,
		Port:                    connection.Port,
		Database:                connection.Database,
		Username:                connection.Username,
		SSLMode:                 connection.SSLMode,
		SSLCAPath:               connection.SSLCAPath,
		SSLCertPath:             connection.SSLCertPath,
		SSLKeyPath:              connection.SSLKeyPath,
		ProxyURL:                connection.ProxyURL,
		SSHTunnel: dto.SSHTunnelView{
			Enabled:        connection.SSHTunnel.Enabled,
			Host:           connection.SSHTunnel.Host,
			Port:           connection.SSHTunnel.Port,
			Username:       connection.SSHTunnel.Username,
			PrivateKeyPath: connection.SSHTunnel.PrivateKeyPath,
			KnownHostsPath: connection.SSHTunnel.KnownHostsPath,
			HasPassword:    connection.SSHTunnel.PasswordCipher != "",
		},
		ReadOnly:        connection.ReadOnly,
		AutoReconnect:   connection.AutoReconnect,
		MaxOpenConns:    connection.MaxOpenConns,
		MaxIdleConns:    connection.MaxIdleConns,
		ConnMaxLifetime: connection.ConnMaxLifetime,
		ConnMaxIdleTime: connection.ConnMaxIdleTime,
		Favorite:        connection.Favorite,
		HasPassword:     connection.PasswordCipher != "",
		HasProxyAuth:    metadata.ProxyUsernameCipher != "" || metadata.ProxyPasswordCipher != "",
		CreatedAt:       connection.CreatedAt.Format(time.RFC3339),
		UpdatedAt:       connection.UpdatedAt.Format(time.RFC3339),
	}
}

func validateConnectionProfile(connection entity.Connection, input dto.ConnectionInput) error {
	if connection.Port < 1 || connection.Port > 65535 || connection.Database == "" || connection.Username == "" || connection.WorkspaceID == "" {
		return errors.New("connection endpoint profile is invalid")
	}
	if connection.SSLMode != entity.SSLModeDisable && connection.SSLMode != entity.SSLModeRequire && connection.SSLMode != entity.SSLModeVerifyCA && connection.SSLMode != entity.SSLModeVerifyFull {
		return ErrInvalidSSLProfile
	}
	if (connection.SSLCertPath == "") != (connection.SSLKeyPath == "") || (connection.SSLMode == entity.SSLModeVerifyCA || connection.SSLMode == entity.SSLModeVerifyFull) && connection.SSLCAPath == "" {
		return ErrInvalidSSLProfile
	}
	for _, path := range []string{connection.SSLCAPath, connection.SSLCertPath, connection.SSLKeyPath} {
		if path != "" && !regularFile(path) {
			return ErrInvalidSSLProfile
		}
	}
	if connection.MaxOpenConns < 1 || connection.MaxOpenConns > 100 || connection.MaxIdleConns < 0 || connection.MaxIdleConns > connection.MaxOpenConns || connection.ConnMaxLifetime < 0 || connection.ConnMaxLifetime > 86400 || connection.ConnMaxIdleTime < 0 || connection.ConnMaxIdleTime > 86400 {
		return ErrInvalidPoolProfile
	}
	if !input.SSHTunnel.Enabled {
		return nil
	}
	if connection.SSHTunnel.Host == "" || connection.SSHTunnel.Port < 1 || connection.SSHTunnel.Port > 65535 || connection.SSHTunnel.Username == "" || !regularFile(connection.SSHTunnel.KnownHostsPath) || (connection.SSHTunnel.PrivateKeyPath == "" && input.SSHTunnel.Password == "" && connection.SSHTunnel.PasswordCipher == "") || (connection.SSHTunnel.PrivateKeyPath != "" && !regularFile(connection.SSHTunnel.PrivateKeyPath)) {
		return ErrInvalidSSHTunnel
	}
	return nil
}

func parseProxyURL(value string) (string, string, string, bool, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", "", "", false, nil
	}
	proxy, err := url.Parse(value)
	if err != nil || proxy.Host == "" || (proxy.Scheme != "http" && proxy.Scheme != "https" && proxy.Scheme != "socks5" && proxy.Scheme != "socks5h") {
		return "", "", "", false, ErrInvalidProxyProfile
	}
	username := ""
	password := ""
	credentialsProvided := proxy.User != nil
	if proxy.User != nil {
		username = proxy.User.Username()
		password, _ = proxy.User.Password()
		proxy.User = nil
	}
	return proxy.String(), username, password, credentialsProvided, nil
}

func duplicateConnectionName(name string, connections []entity.Connection) string {
	used := make(map[string]struct{}, len(connections))
	for _, connection := range connections {
		used[strings.ToLower(strings.TrimSpace(connection.Name))] = struct{}{}
	}
	base := strings.TrimSpace(name) + " copy"
	if _, exists := used[strings.ToLower(base)]; !exists {
		return base
	}
	for sequence := 2; ; sequence++ {
		candidate := base + " " + strconv.Itoa(sequence)
		if _, exists := used[strings.ToLower(candidate)]; !exists {
			return candidate
		}
	}
}

func materialConnectionChanged(before, after entity.Connection, beforeMetadata, afterMetadata ports.ConnectionMetadata) bool {
	return before.Engine != after.Engine ||
		before.Host != after.Host ||
		before.Port != after.Port ||
		before.Database != after.Database ||
		before.Username != after.Username ||
		before.PasswordCipher != after.PasswordCipher ||
		before.SSLMode != after.SSLMode ||
		before.SSLCAPath != after.SSLCAPath ||
		before.SSLCertPath != after.SSLCertPath ||
		before.SSLKeyPath != after.SSLKeyPath ||
		before.ProxyURL != after.ProxyURL ||
		before.SSHTunnel != after.SSHTunnel ||
		before.ReadOnly != after.ReadOnly ||
		before.AutoReconnect != after.AutoReconnect ||
		before.MaxOpenConns != after.MaxOpenConns ||
		before.MaxIdleConns != after.MaxIdleConns ||
		before.ConnMaxLifetime != after.ConnMaxLifetime ||
		before.ConnMaxIdleTime != after.ConnMaxIdleTime ||
		beforeMetadata.ProxyUsernameCipher != afterMetadata.ProxyUsernameCipher ||
		beforeMetadata.ProxyPasswordCipher != afterMetadata.ProxyPasswordCipher
}

func mergeRuntimeStatus(runtime entity.ConnectionRuntimeStatus, metadata ports.ConnectionMetadata) entity.ConnectionRuntimeStatus {
	if runtime.State == "" {
		runtime.State = entity.ConnectionStateDisconnected
	}
	if runtime.LastConnectedAt == nil {
		runtime.LastConnectedAt = metadata.LastConnectedAt
	}
	if runtime.LastErrorCode == "" && runtime.State == entity.ConnectionStateError {
		runtime.LastErrorCode = metadata.LastErrorCode
	}
	return runtime
}

func connectionTestResult(status entity.ConnectionRuntimeStatus) dto.ConnectionTestResult {
	ok := status.State == entity.ConnectionStateConnected
	message := "Connection failed"
	if ok {
		message = "Connection successful"
	}
	return dto.ConnectionTestResult{OK: ok, LatencyMS: status.LatencyMS, Message: message}
}

func connectionInputFromRecord(connection entity.Connection) dto.ConnectionInput {
	return dto.ConnectionInput{
		WorkspaceID: connection.WorkspaceID,
		Name:        connection.Name,
		Engine:      connection.Engine,
		Host:        connection.Host,
		Port:        connection.Port,
		Database:    connection.Database,
		Username:    connection.Username,
		SSLMode:     connection.SSLMode,
		SSLCAPath:   connection.SSLCAPath,
		SSLCertPath: connection.SSLCertPath,
		SSLKeyPath:  connection.SSLKeyPath,
		ProxyURL:    connection.ProxyURL,
		SSHTunnel: dto.SSHTunnelInput{
			Enabled:        connection.SSHTunnel.Enabled,
			Host:           connection.SSHTunnel.Host,
			Port:           connection.SSHTunnel.Port,
			Username:       connection.SSHTunnel.Username,
			PrivateKeyPath: connection.SSHTunnel.PrivateKeyPath,
			KnownHostsPath: connection.SSHTunnel.KnownHostsPath,
		},
		ReadOnly:        connection.ReadOnly,
		AutoReconnect:   connection.AutoReconnect,
		MaxOpenConns:    connection.MaxOpenConns,
		MaxIdleConns:    connection.MaxIdleConns,
		ConnMaxLifetime: connection.ConnMaxLifetime,
		ConnMaxIdleTime: connection.ConnMaxIdleTime,
	}
}

func applyConnectionPatch(input *dto.ConnectionInput, patch dto.ConnectionPatchInput) {
	assign(patch.WorkspaceID, &input.WorkspaceID)
	assign(patch.Name, &input.Name)
	assign(patch.Engine, &input.Engine)
	assign(patch.Host, &input.Host)
	assign(patch.Port, &input.Port)
	assign(patch.Database, &input.Database)
	assign(patch.Username, &input.Username)
	assign(patch.Password, &input.Password)
	assign(patch.ClearPassword, &input.ClearPassword)
	assign(patch.SSLMode, &input.SSLMode)
	assign(patch.SSLCAPath, &input.SSLCAPath)
	assign(patch.SSLCertPath, &input.SSLCertPath)
	assign(patch.SSLKeyPath, &input.SSLKeyPath)
	assign(patch.ProxyURL, &input.ProxyURL)
	assign(patch.ClearProxyAuth, &input.ClearProxyAuth)
	assign(patch.ReadOnly, &input.ReadOnly)
	assign(patch.AutoReconnect, &input.AutoReconnect)
	assign(patch.MaxOpenConns, &input.MaxOpenConns)
	assign(patch.MaxIdleConns, &input.MaxIdleConns)
	assign(patch.ConnMaxLifetime, &input.ConnMaxLifetime)
	assign(patch.ConnMaxIdleTime, &input.ConnMaxIdleTime)
	if patch.SSHTunnel != nil {
		assign(patch.SSHTunnel.Enabled, &input.SSHTunnel.Enabled)
		assign(patch.SSHTunnel.Host, &input.SSHTunnel.Host)
		assign(patch.SSHTunnel.Port, &input.SSHTunnel.Port)
		assign(patch.SSHTunnel.Username, &input.SSHTunnel.Username)
		assign(patch.SSHTunnel.Password, &input.SSHTunnel.Password)
		assign(patch.SSHTunnel.ClearPassword, &input.SSHTunnel.ClearPassword)
		assign(patch.SSHTunnel.PrivateKeyPath, &input.SSHTunnel.PrivateKeyPath)
		assign(patch.SSHTunnel.KnownHostsPath, &input.SSHTunnel.KnownHostsPath)
	}
}

func assign[T any](source *T, target *T) {
	if source != nil {
		*target = *source
	}
}

func encryptOptional(cipher ports.SecretCipher, value string) (string, error) {
	if value == "" {
		return "", nil
	}
	return cipher.Encrypt(value)
}

func decryptOptional(cipher ports.SecretCipher, value string) (string, error) {
	if value == "" {
		return "", nil
	}
	return cipher.Decrypt(value)
}

func validationError(err error) error {
	return apperror.NewValidation(err.Error(), err)
}

func serviceError(err error) error {
	if err == nil {
		return nil
	}
	var appErr *apperror.Error
	if errors.As(err, &appErr) {
		return err
	}
	if errors.Is(err, ports.ErrNotFound) {
		return apperror.NewNotFound("connection not found", err)
	}
	return apperror.NewInternal("connection operation failed", err)
}

func regularFile(path string) bool {
	if !filepath.IsAbs(path) {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}
