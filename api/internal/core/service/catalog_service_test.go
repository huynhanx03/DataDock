package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/huynhanx03/datadock/internal/core/apperror"
	"github.com/huynhanx03/datadock/internal/core/dto"
	"github.com/huynhanx03/datadock/internal/core/entity"
	"github.com/huynhanx03/datadock/internal/core/service"
	"github.com/huynhanx03/datadock/internal/ports"
)

func TestCatalogServiceReturnsStableCapabilityAwareCatalog(t *testing.T) {
	profile := connectionFixture("connection-1", "Primary")
	profile.ProxyUsername = "proxy-user"
	profile.ProxyPassword = "proxy-secret"
	resolver := &catalogProfileResolverFake{connection: profile, password: "database-secret"}
	reader := &catalogRuntimeFake{
		catalog: dto.CatalogTree{
			Capabilities: []string{"table", "view", "materialized-view", "function", "procedure", "sequence", "extension", "trigger"},
			Databases: []dto.CatalogObject{{
				ID:            "database-id",
				Reference:     "eyJ2IjoxLCJraW5kIjoiZGF0YWJhc2UifQ",
				Name:          "app",
				QualifiedName: "app",
				Kind:          dto.CatalogObjectDatabase,
				ChildrenState: dto.CatalogChildrenLoaded,
			}},
		},
	}
	subject := service.NewCatalogService(resolver, reader, reader)

	result, err := subject.Catalog(context.Background(), profile.ID, dto.CatalogInput{Depth: dto.CatalogDepthAll, Limit: 100})
	if err != nil {
		t.Fatalf("Catalog() error = %v", err)
	}
	if result.ConnectionID != profile.ID || result.Engine != entity.EnginePostgreSQL || result.LoadedAt.IsZero() {
		t.Fatalf("Catalog() identity = %#v", result)
	}
	if len(result.Databases) != 1 || result.Databases[0].Reference != "eyJ2IjoxLCJraW5kIjoiZGF0YWJhc2UifQ" || result.Databases[0].ConnectionID != profile.ID {
		t.Fatalf("Catalog() databases = %#v", result.Databases)
	}
	if len(result.Capabilities) != 8 {
		t.Fatalf("Catalog() capabilities = %v", result.Capabilities)
	}
	if reader.lastConnection.ProxyUsername != "proxy-user" || reader.lastConnection.ProxyPassword != "proxy-secret" || reader.lastPassword != "database-secret" {
		t.Fatalf("resolved profile = %#v, %q", reader.lastConnection, reader.lastPassword)
	}
}

func TestCatalogServicePreservesUnsupportedGroupsAndConnectionErrors(t *testing.T) {
	profile := connectionFixture("connection-1", "MySQL")
	profile.Engine = entity.EngineMySQL
	resolver := &catalogProfileResolverFake{connection: profile, password: "secret"}
	reader := &catalogRuntimeFake{
		catalog: dto.CatalogTree{
			Capabilities: []string{"table", "view", "function", "procedure", "trigger"},
			Databases: []dto.CatalogObject{{
				Name:          "app",
				Kind:          dto.CatalogObjectDatabase,
				ChildrenState: dto.CatalogChildrenLoaded,
				Children:      []dto.CatalogObject{{Name: "Tables", Kind: dto.CatalogObjectGroup}, {Name: "Views", Kind: dto.CatalogObjectGroup}},
			}},
		},
	}
	subject := service.NewCatalogService(resolver, reader, reader)

	result, err := subject.Catalog(context.Background(), profile.ID, dto.CatalogInput{Depth: dto.CatalogDepthAll, Limit: 100})
	if err != nil {
		t.Fatalf("Catalog() error = %v", err)
	}
	for _, capability := range result.Capabilities {
		if capability == "materialized-view" || capability == "extension" || capability == "sequence" {
			t.Fatalf("unsupported capability returned: %s", capability)
		}
	}

	reader.catalogErr = apperror.NewConnectionRequired("connection is disconnected", nil)
	_, err = subject.Catalog(context.Background(), profile.ID, dto.CatalogInput{Depth: dto.CatalogDepthAll, Limit: 100})
	var appErr *apperror.Error
	if !errors.As(err, &appErr) || appErr.Code != apperror.CodeConnectionRequired {
		t.Fatalf("Catalog() disconnected error = %#v", err)
	}
}

func TestCatalogServiceValidatesCatalogAndTableRequests(t *testing.T) {
	resolver := &catalogProfileResolverFake{connection: connectionFixture("connection-1", "Primary")}
	reader := &catalogRuntimeFake{}
	subject := service.NewCatalogService(resolver, reader, reader)
	tests := []struct {
		name string
		run  func() error
	}{
		{name: "catalog limit", run: func() error {
			_, err := subject.Catalog(context.Background(), "connection-1", dto.CatalogInput{Limit: 501})
			return err
		}},
		{name: "catalog depth", run: func() error {
			_, err := subject.Catalog(context.Background(), "connection-1", dto.CatalogInput{Depth: "invalid", Limit: 100})
			return err
		}},
		{name: "table reference", run: func() error {
			_, err := subject.TableRows(context.Background(), "connection-1", dto.TableRowsInput{Limit: 100})
			return err
		}},
		{name: "table limit", run: func() error {
			_, err := subject.TableRows(context.Background(), "connection-1", dto.TableRowsInput{Reference: "opaque", Limit: 501})
			return err
		}},
		{name: "table offset", run: func() error {
			_, err := subject.TableRows(context.Background(), "connection-1", dto.TableRowsInput{Reference: "opaque", Limit: 100, Offset: -1})
			return err
		}},
		{name: "filter operator", run: func() error {
			_, err := subject.TableRows(context.Background(), "connection-1", dto.TableRowsInput{Reference: "opaque", Limit: 100, Filters: []dto.TableFilter{{Column: "id", Operator: "raw sql", Value: 1}}})
			return err
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var appErr *apperror.Error
			if err := test.run(); !errors.As(err, &appErr) || appErr.Code != apperror.CodeValidation {
				t.Fatalf("error = %#v", err)
			}
		})
	}
	if reader.catalogCalls != 0 || reader.tableCalls != 0 {
		t.Fatalf("invalid requests reached runtime = %d, %d", reader.catalogCalls, reader.tableCalls)
	}
}

func TestCatalogServiceReturnsTypedLosslessTableData(t *testing.T) {
	profile := connectionFixture("connection-1", "Primary")
	resolver := &catalogProfileResolverFake{connection: profile, password: "secret"}
	total := int64(500)
	reader := &catalogRuntimeFake{rows: dto.TableRowsResult{
		Columns: []dto.DataColumn{
			{Key: "id", Name: "id", DatabaseType: "int8", LogicalType: dto.LogicalTypeBigInt, Nullable: false, PrimaryKey: true, ValueEncoding: dto.ValueEncodingDecimal},
			{Key: "amount", Name: "amount", DatabaseType: "numeric(38,12)", LogicalType: dto.LogicalTypeDecimal, Nullable: false, Precision: intPointer(38), Scale: intPointer(12), ValueEncoding: dto.ValueEncodingDecimal},
			{Key: "payload", Name: "payload", DatabaseType: "jsonb", LogicalType: dto.LogicalTypeJSON, Nullable: true, ValueEncoding: dto.ValueEncodingJSON},
			{Key: "content", Name: "content", DatabaseType: "bytea", LogicalType: dto.LogicalTypeBinary, Nullable: true, ValueEncoding: dto.ValueEncodingBase64},
		},
		Rows:              [][]any{{"9223372036854775807", "12345678901234567890.123456789012", `{"id":9223372036854775807}`, "AAEC", nil}},
		PrimaryKeyColumns: []string{"id"},
		Total:             &total,
		Limit:             100,
		Offset:            0,
		HasMore:           true,
		NextOffset:        100,
		DurationMS:        8,
	}}
	subject := service.NewCatalogService(resolver, reader, reader)

	result, err := subject.TableRows(context.Background(), profile.ID, dto.TableRowsInput{Reference: "opaque-table-reference", Limit: 100, IncludeTotal: true})
	if err != nil {
		t.Fatalf("TableRows() error = %v", err)
	}
	if len(result.Columns) != 4 || result.Columns[0].PrimaryKey != true || result.Columns[1].Precision == nil || *result.Columns[1].Precision != 38 {
		t.Fatalf("typed columns = %#v", result.Columns)
	}
	if result.Rows[0][0] != "9223372036854775807" || result.Rows[0][1] != "12345678901234567890.123456789012" || result.Rows[0][2] != `{"id":9223372036854775807}` || result.Rows[0][3] != "AAEC" || result.Rows[0][4] != nil {
		t.Fatalf("lossless row = %#v", result.Rows[0])
	}
	if !result.HasMore || result.NextOffset != 100 || result.Total == nil || *result.Total != 500 {
		t.Fatalf("pagination = %#v", result)
	}
}

type catalogProfileResolverFake struct {
	connection entity.Connection
	password   string
	err        error
}

func (resolver *catalogProfileResolverFake) Resolve(_ context.Context, _ string) (entity.Connection, string, error) {
	return resolver.connection, resolver.password, resolver.err
}

type catalogRuntimeFake struct {
	catalog        dto.CatalogTree
	catalogErr     error
	rows           dto.TableRowsResult
	rowsErr        error
	schema         dto.TableSchema
	ddl            string
	lastConnection entity.Connection
	lastPassword   string
	catalogCalls   int
	tableCalls     int
}

func (runtime *catalogRuntimeFake) ReadCatalog(_ context.Context, connection entity.Connection, password string, _ dto.CatalogInput) (dto.CatalogTree, error) {
	runtime.catalogCalls++
	runtime.lastConnection = connection
	runtime.lastPassword = password
	return runtime.catalog, runtime.catalogErr
}

func (runtime *catalogRuntimeFake) ReadTableSchema(_ context.Context, connection entity.Connection, password, _ string) (dto.TableSchema, error) {
	runtime.lastConnection = connection
	runtime.lastPassword = password
	return runtime.schema, nil
}

func (runtime *catalogRuntimeFake) ReadTableDDL(_ context.Context, connection entity.Connection, password, _ string) (string, error) {
	runtime.lastConnection = connection
	runtime.lastPassword = password
	return runtime.ddl, nil
}

func (runtime *catalogRuntimeFake) ReadRows(_ context.Context, connection entity.Connection, password string, _ dto.TableRowsInput) (dto.TableRowsResult, error) {
	runtime.tableCalls++
	runtime.lastConnection = connection
	runtime.lastPassword = password
	return runtime.rows, runtime.rowsErr
}

func intPointer(value int64) *int64 {
	return &value
}

var _ ports.ConnectionProfileResolver = (*catalogProfileResolverFake)(nil)
var _ ports.CatalogReader = (*catalogRuntimeFake)(nil)
var _ ports.TableReader = (*catalogRuntimeFake)(nil)
