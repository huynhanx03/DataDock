package service

import (
	"context"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
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
)

type ConnectionService struct {
	repository ports.ConnectionRepository
	cipher     ports.SecretCipher
	tester     ports.ConnectionTester
}

func NewConnectionService(repository ports.ConnectionRepository, cipher ports.SecretCipher, tester ports.ConnectionTester) *ConnectionService {
	return &ConnectionService{repository: repository, cipher: cipher, tester: tester}
}

func (service *ConnectionService) Create(ctx context.Context, input dto.ConnectionInput) (dto.ConnectionView, error) {
	connection, err := service.connectionFromInput(ctx, entity.Connection{ID: uuid.NewString()}, input, false)
	if err != nil {
		return dto.ConnectionView{}, err
	}
	created, err := service.repository.Create(ctx, connection)
	if err != nil {
		return dto.ConnectionView{}, err
	}
	return connectionView(created), nil
}

func (service *ConnectionService) List(ctx context.Context, workspaceID string) ([]dto.ConnectionView, error) {
	connections, err := service.repository.List(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	views := make([]dto.ConnectionView, 0, len(connections))
	for _, connection := range connections {
		views = append(views, connectionView(connection))
	}
	return views, nil
}

func (service *ConnectionService) Get(ctx context.Context, id string) (dto.ConnectionView, error) {
	connection, err := service.repository.Get(ctx, id)
	if err != nil {
		return dto.ConnectionView{}, err
	}
	return connectionView(connection), nil
}

func (service *ConnectionService) Update(ctx context.Context, id string, input dto.ConnectionInput) (dto.ConnectionView, error) {
	existing, err := service.repository.Get(ctx, id)
	if err != nil {
		return dto.ConnectionView{}, err
	}
	connection, err := service.connectionFromInput(ctx, existing, input, true)
	if err != nil {
		return dto.ConnectionView{}, err
	}
	updated, err := service.repository.Update(ctx, connection)
	if err != nil {
		return dto.ConnectionView{}, err
	}
	return connectionView(updated), nil
}

func (service *ConnectionService) Duplicate(ctx context.Context, id string) (dto.ConnectionView, error) {
	existing, err := service.repository.Get(ctx, id)
	if err != nil {
		return dto.ConnectionView{}, err
	}
	now := time.Now().UTC()
	existing.ID = uuid.NewString()
	existing.Name = existing.Name + " copy"
	existing.Favorite = false
	existing.CreatedAt = now
	existing.UpdatedAt = now
	created, err := service.repository.Create(ctx, existing)
	if err != nil {
		return dto.ConnectionView{}, err
	}
	return connectionView(created), nil
}

func (service *ConnectionService) Delete(ctx context.Context, id string) error {
	return service.repository.Delete(ctx, id)
}

func (service *ConnectionService) Test(ctx context.Context, id string) error {
	connection, err := service.repository.Get(ctx, id)
	if err != nil {
		return err
	}
	password := ""
	if connection.PasswordCipher != "" {
		password, err = service.cipher.Decrypt(connection.PasswordCipher)
		if err != nil {
			return err
		}
	}
	if connection.SSHTunnel.PasswordCipher != "" {
		connection.SSHTunnel.Password, err = service.cipher.Decrypt(connection.SSHTunnel.PasswordCipher)
		if err != nil {
			return err
		}
	}
	return service.tester.Test(ctx, connection, password)
}

func (service *ConnectionService) connectionFromInput(ctx context.Context, current entity.Connection, input dto.ConnectionInput, update bool) (entity.Connection, error) {
	_ = ctx
	if !input.Engine.Valid() {
		return entity.Connection{}, ErrUnsupportedEngine
	}
	current.WorkspaceID = input.WorkspaceID
	current.Name = strings.TrimSpace(input.Name)
	current.Host = strings.TrimSpace(input.Host)
	if current.Name == "" {
		return entity.Connection{}, ErrConnectionNameRequired
	}
	if current.Host == "" {
		return entity.Connection{}, ErrConnectionHostRequired
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
	current.ProxyURL = strings.TrimSpace(input.ProxyURL)
	if err := validateConnectionProfile(current, input); err != nil {
		return entity.Connection{}, err
	}
	current.SSHTunnel.Enabled = input.SSHTunnel.Enabled
	current.SSHTunnel.Host = strings.TrimSpace(input.SSHTunnel.Host)
	current.SSHTunnel.Port = input.SSHTunnel.Port
	current.SSHTunnel.Username = strings.TrimSpace(input.SSHTunnel.Username)
	current.SSHTunnel.PrivateKeyPath = strings.TrimSpace(input.SSHTunnel.PrivateKeyPath)
	current.SSHTunnel.KnownHostsPath = strings.TrimSpace(input.SSHTunnel.KnownHostsPath)
	if input.SSHTunnel.Password != "" {
		ciphertext, err := service.cipher.Encrypt(input.SSHTunnel.Password)
		if err != nil {
			return entity.Connection{}, err
		}
		current.SSHTunnel.PasswordCipher = ciphertext
	}
	current.ReadOnly = input.ReadOnly
	current.AutoReconnect = input.AutoReconnect
	current.MaxOpenConns = input.MaxOpenConns
	current.MaxIdleConns = input.MaxIdleConns
	current.ConnMaxLifetime = input.ConnMaxLifetime
	if input.Password != "" {
		ciphertext, err := service.cipher.Encrypt(input.Password)
		if err != nil {
			return entity.Connection{}, err
		}
		current.PasswordCipher = ciphertext
	}
	if !update {
		now := time.Now().UTC()
		current.CreatedAt = now
	}
	current.UpdatedAt = time.Now().UTC()
	return current, nil
}

func connectionView(connection entity.Connection) dto.ConnectionView {
	return dto.ConnectionView{ID: connection.ID, WorkspaceID: connection.WorkspaceID, Name: connection.Name, Engine: connection.Engine, Host: connection.Host, Port: connection.Port, Database: connection.Database, Username: connection.Username, SSLMode: connection.SSLMode, SSLCAPath: connection.SSLCAPath, SSLCertPath: connection.SSLCertPath, SSLKeyPath: connection.SSLKeyPath, ProxyURL: connection.ProxyURL, SSHTunnel: dto.SSHTunnelView{Enabled: connection.SSHTunnel.Enabled, Host: connection.SSHTunnel.Host, Port: connection.SSHTunnel.Port, Username: connection.SSHTunnel.Username, PrivateKeyPath: connection.SSHTunnel.PrivateKeyPath, KnownHostsPath: connection.SSHTunnel.KnownHostsPath, HasPassword: connection.SSHTunnel.PasswordCipher != ""}, ReadOnly: connection.ReadOnly, AutoReconnect: connection.AutoReconnect, MaxOpenConns: connection.MaxOpenConns, MaxIdleConns: connection.MaxIdleConns, ConnMaxLifetime: connection.ConnMaxLifetime, Favorite: connection.Favorite, HasPassword: connection.PasswordCipher != "", CreatedAt: connection.CreatedAt.Format(time.RFC3339), UpdatedAt: connection.UpdatedAt.Format(time.RFC3339)}
}

func validateConnectionProfile(connection entity.Connection, input dto.ConnectionInput) error {
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
	if connection.ProxyURL != "" {
		proxy, err := url.Parse(connection.ProxyURL)
		if err != nil || proxy.Host == "" || (proxy.Scheme != "http" && proxy.Scheme != "https" && proxy.Scheme != "socks5" && proxy.Scheme != "socks5h") {
			return ErrInvalidProxyProfile
		}
	}
	if !input.SSHTunnel.Enabled {
		return nil
	}
	if strings.TrimSpace(input.SSHTunnel.Host) == "" || input.SSHTunnel.Port < 1 || strings.TrimSpace(input.SSHTunnel.Username) == "" || !regularFile(strings.TrimSpace(input.SSHTunnel.KnownHostsPath)) || (strings.TrimSpace(input.SSHTunnel.PrivateKeyPath) == "" && input.SSHTunnel.Password == "" && connection.SSHTunnel.PasswordCipher == "") || (strings.TrimSpace(input.SSHTunnel.PrivateKeyPath) != "" && !regularFile(strings.TrimSpace(input.SSHTunnel.PrivateKeyPath))) {
		return ErrInvalidSSHTunnel
	}
	return nil
}

func regularFile(path string) bool {
	if !filepath.IsAbs(path) {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}
