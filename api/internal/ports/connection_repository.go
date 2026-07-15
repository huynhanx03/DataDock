package ports

import (
	"context"
	"errors"
	"time"

	"github.com/huynhanx03/datadock/internal/core/entity"
)

var ErrNotFound = errors.New("repository record not found")

type ConnectionRepository interface {
	Create(context.Context, entity.Connection) (entity.Connection, error)
	List(context.Context, string) ([]entity.Connection, error)
	Get(context.Context, string) (entity.Connection, error)
	Update(context.Context, entity.Connection) (entity.Connection, error)
	Delete(context.Context, string) error
}

type ConnectionMetadata struct {
	ConnectionID        string
	ProxyUsernameCipher string
	ProxyPasswordCipher string
	Status              entity.ConnectionState
	LastConnectedAt     *time.Time
	LastDisconnectedAt  *time.Time
	LastErrorCode       string
}

type ConnectionMetadataRepository interface {
	GetMetadata(context.Context, string) (ConnectionMetadata, error)
	UpdateMetadata(context.Context, ConnectionMetadata) error
}

type ConnectionRecord struct {
	Connection entity.Connection
	Metadata   ConnectionMetadata
}

type ConnectionStore interface {
	ConnectionRepository
	ConnectionMetadataRepository
	CreateRecord(context.Context, ConnectionRecord) (ConnectionRecord, error)
	ListRecords(context.Context, string) ([]ConnectionRecord, error)
	GetRecord(context.Context, string) (ConnectionRecord, error)
	UpdateRecord(context.Context, ConnectionRecord) (ConnectionRecord, error)
}
