package service

import (
	"context"

	"github.com/huynhanx03/datadock/internal/core/entity"
	"github.com/huynhanx03/datadock/internal/ports"
)

var _ ports.ConnectionProfileResolver = (*ConnectionProfileResolver)(nil)

type ConnectionProfileResolver struct {
	store  ports.ConnectionStore
	cipher ports.SecretCipher
}

func NewConnectionProfileResolver(store ports.ConnectionStore, cipher ports.SecretCipher) *ConnectionProfileResolver {
	return &ConnectionProfileResolver{store: store, cipher: cipher}
}

func (resolver *ConnectionProfileResolver) Resolve(ctx context.Context, connectionID string) (entity.Connection, string, error) {
	record, err := resolver.store.GetRecord(ctx, connectionID)
	if err != nil {
		return entity.Connection{}, "", serviceError(err)
	}
	password, err := decryptOptional(resolver.cipher, record.Connection.PasswordCipher)
	if err != nil {
		return entity.Connection{}, "", serviceError(err)
	}
	record.Connection.SSHTunnel.Password, err = decryptOptional(resolver.cipher, record.Connection.SSHTunnel.PasswordCipher)
	if err != nil {
		return entity.Connection{}, "", serviceError(err)
	}
	record.Connection.ProxyUsername, err = decryptOptional(resolver.cipher, record.Metadata.ProxyUsernameCipher)
	if err != nil {
		return entity.Connection{}, "", serviceError(err)
	}
	record.Connection.ProxyPassword, err = decryptOptional(resolver.cipher, record.Metadata.ProxyPasswordCipher)
	if err != nil {
		return entity.Connection{}, "", serviceError(err)
	}
	return record.Connection, password, nil
}
