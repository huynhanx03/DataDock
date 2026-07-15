package engines

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/huynhanx03/datadock/internal/core/apperror"
	"github.com/huynhanx03/datadock/internal/core/dto"
	"github.com/huynhanx03/datadock/internal/core/entity"
)

type catalogReference struct {
	Version   int                   `json:"v"`
	Kind      dto.CatalogObjectKind `json:"kind"`
	Database  string                `json:"database,omitempty"`
	Schema    string                `json:"schema,omitempty"`
	Name      string                `json:"name,omitempty"`
	Signature string                `json:"signature,omitempty"`
	Owner     string                `json:"owner,omitempty"`
}

type engineDialect interface {
	quoteIdentifier(string) string
	placeholder(int) string
	qualified(catalogReference) (string, error)
}

type postgresDialect struct{}

func (postgresDialect) quoteIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

func (postgresDialect) placeholder(position int) string {
	return fmt.Sprintf("$%d", position)
}

func (dialect postgresDialect) qualified(reference catalogReference) (string, error) {
	if reference.Schema == "" || reference.Name == "" {
		return "", apperror.NewValidation("invalid PostgreSQL object reference", nil)
	}
	return dialect.quoteIdentifier(reference.Schema) + "." + dialect.quoteIdentifier(reference.Name), nil
}

type mysqlDialect struct{}

func (mysqlDialect) quoteIdentifier(value string) string {
	return "`" + strings.ReplaceAll(value, "`", "``") + "`"
}

func (mysqlDialect) placeholder(int) string {
	return "?"
}

func (dialect mysqlDialect) qualified(reference catalogReference) (string, error) {
	database := reference.Database
	if database == "" {
		database = reference.Schema
	}
	if database == "" || reference.Name == "" {
		return "", apperror.NewValidation("invalid MySQL object reference", nil)
	}
	return dialect.quoteIdentifier(database) + "." + dialect.quoteIdentifier(reference.Name), nil
}

func dialectFor(engine entity.Engine) (engineDialect, error) {
	if engine == entity.EnginePostgreSQL {
		return postgresDialect{}, nil
	}
	if engine == entity.EngineMySQL || engine == entity.EngineMariaDB {
		return mysqlDialect{}, nil
	}
	return nil, apperror.NewUnsupported("database engine is not supported", nil)
}

func encodeCatalogReference(reference catalogReference) (string, error) {
	reference.Version = 1
	if err := validateCatalogReference(reference); err != nil {
		return "", err
	}
	payload, err := json.Marshal(reference)
	if err != nil {
		return "", apperror.NewInternal("catalog reference could not be encoded", err)
	}
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func decodeCatalogReference(value string) (catalogReference, error) {
	if value == "" || len(value) > 4096 || strings.ContainsRune(value, 0) {
		return catalogReference{}, apperror.NewValidation("invalid catalog reference", nil)
	}
	payload, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || len(payload) > 3072 {
		return catalogReference{}, apperror.NewValidation("invalid catalog reference", err)
	}
	var reference catalogReference
	decoder := json.NewDecoder(strings.NewReader(string(payload)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&reference); err != nil {
		return catalogReference{}, apperror.NewValidation("invalid catalog reference", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return catalogReference{}, apperror.NewValidation("invalid catalog reference", err)
	}
	if reference.Version != 1 {
		return catalogReference{}, apperror.NewValidation("unsupported catalog reference version", nil)
	}
	if err := validateCatalogReference(reference); err != nil {
		return catalogReference{}, err
	}
	return reference, nil
}

func encodeCatalogCursor(connectionID, parentReference string, offset int) (string, error) {
	if connectionID == "" || strings.ContainsRune(connectionID, 0) || strings.ContainsRune(parentReference, 0) || offset < 0 {
		return "", apperror.NewValidation("invalid catalog cursor", nil)
	}
	payload := make([]byte, 25)
	payload[0] = 1
	binary.BigEndian.PutUint64(payload[1:9], uint64(offset))
	scope := catalogCursorScope(connectionID, parentReference)
	copy(payload[9:], scope[:16])
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func decodeCatalogCursor(value, connectionID, parentReference string) (int, error) {
	if value == "" {
		return 0, nil
	}
	if connectionID == "" || len(value) > 128 || strings.ContainsRune(value, 0) || strings.ContainsRune(connectionID, 0) || strings.ContainsRune(parentReference, 0) {
		return 0, apperror.NewValidation("invalid catalog cursor", nil)
	}
	payload, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || len(payload) != 25 || payload[0] != 1 {
		return 0, apperror.NewValidation("invalid catalog cursor", err)
	}
	scope := catalogCursorScope(connectionID, parentReference)
	if subtle.ConstantTimeCompare(payload[9:], scope[:16]) != 1 {
		return 0, apperror.NewValidation("invalid catalog cursor", nil)
	}
	offset := binary.BigEndian.Uint64(payload[1:9])
	maximum := uint64(^uint(0) >> 1)
	if offset > maximum {
		return 0, apperror.NewValidation("invalid catalog cursor", nil)
	}
	return int(offset), nil
}

func catalogCursorScope(connectionID, parentReference string) [32]byte {
	scope := parentReference
	if scope == "" {
		scope = "root"
	}
	return sha256.Sum256([]byte("catalog-cursor:v1\x00" + connectionID + "\x00" + scope))
}

func validateCatalogReference(reference catalogReference) error {
	validKind := reference.Kind == dto.CatalogObjectDatabase || reference.Kind == dto.CatalogObjectSchema || reference.Kind == dto.CatalogObjectGroup || reference.Kind == dto.CatalogObjectTable || reference.Kind == dto.CatalogObjectView || reference.Kind == dto.CatalogObjectMaterializedView || reference.Kind == dto.CatalogObjectFunction || reference.Kind == dto.CatalogObjectProcedure || reference.Kind == dto.CatalogObjectSequence || reference.Kind == dto.CatalogObjectExtension || reference.Kind == dto.CatalogObjectTrigger || reference.Kind == dto.CatalogObjectColumn
	if !validKind {
		return apperror.NewValidation("invalid catalog object kind", nil)
	}
	for _, value := range []string{reference.Database, reference.Schema, reference.Name, reference.Signature, reference.Owner} {
		if len(value) > 512 || strings.ContainsRune(value, 0) {
			return apperror.NewValidation("invalid catalog reference value", nil)
		}
	}
	if reference.Kind != dto.CatalogObjectDatabase && reference.Kind != dto.CatalogObjectSchema && reference.Kind != dto.CatalogObjectGroup && reference.Name == "" {
		return apperror.NewValidation("catalog object name is required", nil)
	}
	return nil
}

func catalogObjectID(connectionID string, reference catalogReference) string {
	encoded, err := encodeCatalogReference(reference)
	if err != nil {
		encoded = strings.Join([]string{string(reference.Kind), reference.Database, reference.Schema, reference.Name, reference.Signature, reference.Owner}, "\x00")
	}
	sum := sha256.Sum256([]byte(connectionID + "\x00" + encoded))
	return connectionID + ":" + hex.EncodeToString(sum[:12])
}

func catalogCapabilities(engine entity.Engine) []string {
	if engine == entity.EnginePostgreSQL {
		return []string{"table", "view", "materialized-view", "function", "procedure", "sequence", "extension", "trigger"}
	}
	if engine == entity.EngineMySQL || engine == entity.EngineMariaDB {
		return []string{"table", "view", "function", "procedure", "trigger"}
	}
	return []string{}
}

func catalogChildrenState(children []dto.CatalogObject) dto.CatalogChildrenState {
	if len(children) == 0 {
		return dto.CatalogChildrenEmpty
	}
	return dto.CatalogChildrenLoaded
}

func ensureBrowsableReference(reference catalogReference) error {
	if reference.Kind != dto.CatalogObjectTable && reference.Kind != dto.CatalogObjectView && reference.Kind != dto.CatalogObjectMaterializedView {
		return apperror.NewValidation("catalog object is not browsable", nil)
	}
	return nil
}

func logicalTypeForDatabaseType(databaseType string) dto.LogicalType {
	typeName := strings.ToLower(strings.TrimSpace(databaseType))
	switch {
	case strings.Contains(typeName, "bigint"), typeName == "int8", strings.Contains(typeName, "bigserial"):
		return dto.LogicalTypeBigInt
	case strings.Contains(typeName, "decimal"), strings.Contains(typeName, "numeric"), strings.Contains(typeName, "money"):
		return dto.LogicalTypeDecimal
	case strings.Contains(typeName, "smallint"), strings.Contains(typeName, "integer"), typeName == "int", typeName == "int2", typeName == "int4", strings.Contains(typeName, "serial"):
		return dto.LogicalTypeInteger
	case strings.Contains(typeName, "double"), strings.Contains(typeName, "float"), strings.Contains(typeName, "real"):
		return dto.LogicalTypeFloat
	case strings.Contains(typeName, "bool"), strings.Contains(typeName, "tinyint(1)"):
		return dto.LogicalTypeBoolean
	case strings.Contains(typeName, "json"):
		return dto.LogicalTypeJSON
	case strings.Contains(typeName, "bytea"), strings.Contains(typeName, "blob"), strings.Contains(typeName, "binary"), strings.Contains(typeName, "bit varying"):
		return dto.LogicalTypeBinary
	case strings.Contains(typeName, "timestamp"), strings.Contains(typeName, "datetime"):
		return dto.LogicalTypeDateTime
	case typeName == "date":
		return dto.LogicalTypeDate
	case strings.HasPrefix(typeName, "time"):
		return dto.LogicalTypeTime
	case strings.Contains(typeName, "uuid"):
		return dto.LogicalTypeUUID
	case strings.HasPrefix(typeName, "enum") || strings.HasPrefix(typeName, "set"):
		return dto.LogicalTypeEnum
	case strings.Contains(typeName, "char"), strings.Contains(typeName, "text"), strings.Contains(typeName, "xml"), strings.Contains(typeName, "inet"):
		return dto.LogicalTypeString
	default:
		return dto.LogicalTypeUnknown
	}
}

func valueEncodingForLogicalType(logicalType dto.LogicalType) dto.ValueEncoding {
	switch logicalType {
	case dto.LogicalTypeBigInt, dto.LogicalTypeDecimal:
		return dto.ValueEncodingDecimal
	case dto.LogicalTypeJSON:
		return dto.ValueEncodingJSON
	case dto.LogicalTypeBinary:
		return dto.ValueEncodingBase64
	case dto.LogicalTypeDate, dto.LogicalTypeTime, dto.LogicalTypeDateTime:
		return dto.ValueEncodingTemporal
	default:
		return dto.ValueEncodingNative
	}
}

func unwrapCatalogError(err error) error {
	if err == nil {
		return nil
	}
	var appErr *apperror.Error
	if errors.As(err, &appErr) {
		return err
	}
	return apperror.NewInternal("database catalog operation failed", err)
}
