package ports

import (
	"context"

	"github.com/huynhanx03/datadock/internal/core/entity"
)

type ConnectionLifecycle interface {
	TestDraft(context.Context, entity.Connection, string) (entity.ConnectionRuntimeStatus, error)
	Connect(context.Context, entity.Connection, string) (entity.ConnectionRuntimeStatus, error)
	Disconnect(context.Context, string) error
	Status(string) entity.ConnectionRuntimeStatus
	Invalidate(context.Context, entity.Connection, string) error
	Remove(string) error
	Close() error
}
