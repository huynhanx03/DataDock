package ports

import (
	"context"

	"github.com/huynhanx03/datadock/internal/core/entity"
)

type ConnectionTester interface {
	Test(context.Context, entity.Connection, string) error
}
