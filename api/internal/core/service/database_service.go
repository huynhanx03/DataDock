package service

import (
	"context"
	"errors"
	"strings"

	"github.com/huynhanx03/datadock/internal/adapters/driven/engines"
	"github.com/huynhanx03/datadock/internal/core/dto"
	"github.com/huynhanx03/datadock/internal/core/entity"
	"github.com/huynhanx03/datadock/internal/ports"
)

var (
	ErrQueryRequired      = errors.New("query is required")
	ErrReadOnlyConnection = errors.New("write queries are disabled for this read-only connection")
	ErrTableRequired      = errors.New("table is required")
	ErrInvalidTableBrowse = errors.New("invalid table browser request")
	ErrInvalidSchemaInput = errors.New("invalid schema request")
	ErrInvalidRowMutation = errors.New("invalid row mutation request")
)

type DatabaseService struct {
	repository ports.ConnectionRepository
	cipher     ports.SecretCipher
	runtime    ports.DatabaseRuntime
}

func (service *DatabaseService) MutateRows(ctx context.Context, connectionID, table string, mutations []dto.RowMutation) (dto.TableMutateResult, error) {
	if !validTableReference(table) || len(mutations) == 0 || len(mutations) > 100 {
		return dto.TableMutateResult{}, ErrInvalidRowMutation
	}
	for _, mutation := range mutations {
		if mutation.Kind != "insert" && mutation.Kind != "update" && mutation.Kind != "delete" || len(mutation.Values) > 200 || len(mutation.Keys) > 32 {
			return dto.TableMutateResult{}, ErrInvalidRowMutation
		}
		if mutation.Kind == "insert" && len(mutation.Values) == 0 || mutation.Kind == "update" && (len(mutation.Values) == 0 || len(mutation.Keys) == 0) || mutation.Kind == "delete" && len(mutation.Keys) == 0 {
			return dto.TableMutateResult{}, ErrInvalidRowMutation
		}
		for column := range mutation.Values {
			if !validIdentifier(column) {
				return dto.TableMutateResult{}, ErrInvalidRowMutation
			}
		}
		for column := range mutation.Keys {
			if !validIdentifier(column) {
				return dto.TableMutateResult{}, ErrInvalidRowMutation
			}
		}
	}
	connection, password, err := service.connectionSecret(ctx, connectionID)
	if err != nil {
		return dto.TableMutateResult{}, err
	}
	if connection.ReadOnly {
		return dto.TableMutateResult{}, ErrReadOnlyConnection
	}
	return service.runtime.MutateRows(ctx, connection, password, table, mutations)
}

func (service *DatabaseService) TableSchema(ctx context.Context, connectionID, table string) (dto.TableSchema, error) {
	if !validTableReference(table) {
		return dto.TableSchema{}, ErrInvalidSchemaInput
	}
	connection, password, err := service.connectionSecret(ctx, connectionID)
	if err != nil {
		return dto.TableSchema{}, err
	}
	return service.runtime.TableSchema(ctx, connection, password, table)
}

func (service *DatabaseService) CreateTable(ctx context.Context, connectionID string, input dto.CreateTableInput) error {
	if !validIdentifier(input.Name) || (input.Schema != "" && !validIdentifier(input.Schema)) || len(input.Columns) == 0 {
		return ErrInvalidSchemaInput
	}
	connection, password, err := service.connectionSecret(ctx, connectionID)
	if err != nil {
		return err
	}
	if connection.ReadOnly {
		return ErrReadOnlyConnection
	}
	reference := input.Name
	if input.Schema != "" {
		reference = input.Schema + "." + input.Name
	}
	return service.runtime.CreateTable(ctx, connection, password, reference, input.Columns)
}

func (service *DatabaseService) AlterTable(ctx context.Context, connectionID, table string, input dto.AlterTableInput) error {
	if !validTableReference(table) || len(input.Actions) == 0 {
		return ErrInvalidSchemaInput
	}
	connection, password, err := service.connectionSecret(ctx, connectionID)
	if err != nil {
		return err
	}
	if connection.ReadOnly {
		return ErrReadOnlyConnection
	}
	return service.runtime.AlterTable(ctx, connection, password, table, input.Actions)
}

func (service *DatabaseService) CreateIndex(ctx context.Context, connectionID, table string, input dto.CreateIndexInput) error {
	if !validTableReference(table) || !validIdentifier(input.Name) || len(input.Columns) == 0 {
		return ErrInvalidSchemaInput
	}
	for _, column := range input.Columns {
		if !validIdentifier(column) {
			return ErrInvalidSchemaInput
		}
	}
	connection, password, err := service.connectionSecret(ctx, connectionID)
	if err != nil {
		return err
	}
	if connection.ReadOnly {
		return ErrReadOnlyConnection
	}
	return service.runtime.CreateIndex(ctx, connection, password, table, input)
}

func (service *DatabaseService) DropIndex(ctx context.Context, connectionID, table, index string) error {
	if !validTableReference(table) || !validIdentifier(index) {
		return ErrInvalidSchemaInput
	}
	connection, password, err := service.connectionSecret(ctx, connectionID)
	if err != nil {
		return err
	}
	if connection.ReadOnly {
		return ErrReadOnlyConnection
	}
	return service.runtime.DropIndex(ctx, connection, password, table, index)
}

func (service *DatabaseService) BrowseTable(ctx context.Context, input dto.TableRowsInput) (dto.TableRowsResult, error) {
	if strings.TrimSpace(input.Table) == "" {
		return dto.TableRowsResult{}, ErrTableRequired
	}
	if !validTableReference(input.Table) || (input.Sort != "" && !validIdentifier(input.Sort)) || input.Limit < 1 || input.Limit > 200 || input.Offset < 0 || len(input.Search) > 500 || len(input.Sort) > 255 {
		return dto.TableRowsResult{}, ErrInvalidTableBrowse
	}
	connection, password, err := service.connectionSecret(ctx, input.ConnectionID)
	if err != nil {
		return dto.TableRowsResult{}, err
	}
	return service.runtime.BrowseTable(ctx, connection, password, input)
}

func (service *DatabaseService) TableDDL(ctx context.Context, connectionID, table string) (string, error) {
	if strings.TrimSpace(table) == "" {
		return "", ErrTableRequired
	}
	if !validTableReference(table) {
		return "", ErrInvalidTableBrowse
	}
	connection, password, err := service.connectionSecret(ctx, connectionID)
	if err != nil {
		return "", err
	}
	return service.runtime.TableDDL(ctx, connection, password, table)
}

func (service *DatabaseService) Dashboard(ctx context.Context, connectionID string) (dto.DatabaseDashboard, error) {
	connection, password, err := service.connectionSecret(ctx, connectionID)
	if err != nil {
		return dto.DatabaseDashboard{}, err
	}
	return service.runtime.Dashboard(ctx, connection, password)
}

func (service *DatabaseService) Sessions(ctx context.Context, connectionID string) (dto.DatabaseSessions, error) {
	connection, password, err := service.connectionSecret(ctx, connectionID)
	if err != nil {
		return dto.DatabaseSessions{}, err
	}
	return service.runtime.Sessions(ctx, connection, password)
}

func (service *DatabaseService) Locks(ctx context.Context, connectionID string) (dto.DatabaseLocks, error) {
	connection, password, err := service.connectionSecret(ctx, connectionID)
	if err != nil {
		return dto.DatabaseLocks{}, err
	}
	return service.runtime.Locks(ctx, connection, password)
}

func (service *DatabaseService) Performance(ctx context.Context, connectionID string) (dto.DatabasePerformance, error) {
	connection, password, err := service.connectionSecret(ctx, connectionID)
	if err != nil {
		return dto.DatabasePerformance{}, err
	}
	return service.runtime.Performance(ctx, connection, password)
}

func validTableReference(value string) bool {
	parts := strings.Split(value, ".")
	if len(parts) < 1 || len(parts) > 2 {
		return false
	}
	for _, part := range parts {
		if !validIdentifier(part) {
			return false
		}
	}
	return true
}

func validIdentifier(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for index, character := range value {
		if !(character == '_' || character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || index > 0 && character >= '0' && character <= '9') {
			return false
		}
	}
	return true
}

func NewDatabaseService(repository ports.ConnectionRepository, cipher ports.SecretCipher, runtime ports.DatabaseRuntime) *DatabaseService {
	return &DatabaseService{repository: repository, cipher: cipher, runtime: runtime}
}

func (service *DatabaseService) ListTables(ctx context.Context, connectionID string) ([]string, error) {
	connection, password, err := service.connectionSecret(ctx, connectionID)
	if err != nil {
		return nil, err
	}
	return service.runtime.ListTables(ctx, connection, password)
}

func (service *DatabaseService) Execute(ctx context.Context, input dto.ExecuteQueryInput) (dto.QueryResult, error) {
	if strings.TrimSpace(input.SQL) == "" {
		return dto.QueryResult{}, ErrQueryRequired
	}
	connection, password, err := service.connectionSecret(ctx, input.ConnectionID)
	if err != nil {
		return dto.QueryResult{}, err
	}
	if connection.ReadOnly && service.runtime.IsWriteSQL(input.SQL) {
		return dto.QueryResult{}, ErrReadOnlyConnection
	}
	if input.TransactionID != "" {
		if runtime, ok := service.runtime.(*engines.Runtime); ok {
			return runtime.ExecuteTransaction(ctx, input.TransactionID, input.SQL, input.TimeoutSeconds)
		}
		return dto.QueryResult{}, errors.New("transaction runtime is unavailable")
	}
	return service.runtime.Execute(ctx, connection, password, input.SQL, input.TimeoutSeconds)
}

func (service *DatabaseService) BeginTransaction(ctx context.Context, connectionID string) (dto.TransactionState, error) {
	connection, password, err := service.connectionSecret(ctx, connectionID)
	if err != nil {
		return dto.TransactionState{}, err
	}
	if connection.ReadOnly {
		return dto.TransactionState{}, ErrReadOnlyConnection
	}
	return service.runtime.BeginTransaction(ctx, connection, password)
}
func (service *DatabaseService) TransactionAction(ctx context.Context, id, action, name string) (dto.TransactionState, error) {
	return service.runtime.TransactionAction(ctx, id, action, name)
}
func (service *DatabaseService) CancelSession(ctx context.Context, input dto.SessionActionInput) error {
	connection, password, err := service.connectionSecret(ctx, input.ConnectionID)
	if err != nil {
		return err
	}
	return service.runtime.CancelSession(ctx, connection, password, input.SessionID, input.Force)
}

func (service *DatabaseService) connectionSecret(ctx context.Context, connectionID string) (entity.Connection, string, error) {
	connection, err := service.repository.Get(ctx, connectionID)
	if err != nil {
		return entity.Connection{}, "", err
	}
	password := ""
	if connection.PasswordCipher != "" {
		password, err = service.cipher.Decrypt(connection.PasswordCipher)
		if err != nil {
			return entity.Connection{}, "", err
		}
	}
	if connection.SSHTunnel.PasswordCipher != "" {
		connection.SSHTunnel.Password, err = service.cipher.Decrypt(connection.SSHTunnel.PasswordCipher)
		if err != nil {
			return entity.Connection{}, "", err
		}
	}
	return connection, password, nil
}
