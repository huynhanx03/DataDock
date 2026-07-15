package engines

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/huynhanx03/datadock/internal/core/dto"
)

const maxSafeJSONInteger = int64(1<<53 - 1)

func scanTypedRows(rows *sql.Rows, hints []dto.DataColumn, maxRows int, maxBytes int64) ([]dto.DataColumn, [][]any, bool, int64, error) {
	if rows == nil {
		return nil, nil, false, 0, errors.New("rows are required")
	}
	columnTypes, err := rows.ColumnTypes()
	if err != nil {
		return nil, nil, false, 0, err
	}
	columns := typedResultColumns(columnTypes, hints)
	result := make([][]any, 0, resultCapacity(maxRows))
	var encodedBytes int64
	truncated := false
	for rows.Next() {
		if maxRows > 0 && len(result) >= maxRows {
			truncated = true
			break
		}
		values := make([]any, len(columnTypes))
		references := make([]any, len(columnTypes))
		for index := range values {
			references[index] = &values[index]
		}
		if err := rows.Scan(references...); err != nil {
			return nil, nil, false, encodedBytes, err
		}
		for index := range values {
			values[index] = typedResultValue(values[index], columns[index])
		}
		encoded, err := json.Marshal(values)
		if err != nil {
			return nil, nil, false, encodedBytes, err
		}
		rowBytes := int64(len(encoded))
		if len(result) > 0 {
			rowBytes++
		}
		if maxBytes > 0 && (encodedBytes >= maxBytes || rowBytes > maxBytes-encodedBytes) {
			truncated = true
			break
		}
		result = append(result, values)
		encodedBytes += rowBytes
	}
	if err := rows.Err(); err != nil {
		return nil, nil, false, encodedBytes, err
	}
	return columns, result, truncated, encodedBytes, nil
}

func typedResultColumns(columnTypes []*sql.ColumnType, hints []dto.DataColumn) []dto.DataColumn {
	columns := make([]dto.DataColumn, len(columnTypes))
	for index, columnType := range columnTypes {
		column := dto.DataColumn{Nullable: true}
		hasHint := index < len(hints)
		if hasHint {
			column = hints[index]
			column.EnumValues = append([]string(nil), column.EnumValues...)
		}
		name := columnType.Name()
		if name == "" {
			name = column.Name
		}
		if name == "" {
			name = "column_" + strconv.Itoa(index+1)
		}
		column.Name = name
		driverType := normalizeDatabaseType(columnType.DatabaseTypeName())
		if column.DatabaseType == "" {
			column.DatabaseType = driverType
		} else {
			column.DatabaseType = normalizeDatabaseType(column.DatabaseType)
		}
		if column.DatabaseType == "" && column.Type != "" {
			column.DatabaseType = normalizeDatabaseType(column.Type)
		}
		if !hasHint {
			if nullable, ok := columnType.Nullable(); ok {
				column.Nullable = nullable
			}
		}
		if column.Precision == nil || column.Scale == nil {
			if precision, scale, ok := columnType.DecimalSize(); ok {
				if column.Precision == nil {
					column.Precision = int64Pointer(precision)
				}
				if column.Scale == nil {
					column.Scale = int64Pointer(scale)
				}
			}
		}
		if column.Length == nil {
			if length, ok := columnType.Length(); ok {
				column.Length = int64Pointer(length)
			}
		}
		if column.LogicalType == "" || column.LogicalType == dto.LogicalTypeUnknown {
			column.LogicalType = inferResultLogicalType(column.DatabaseType, columnType.ScanType(), column.Length)
		}
		if column.LogicalType == "" {
			column.LogicalType = dto.LogicalTypeUnknown
		}
		column.ValueEncoding = resultValueEncoding(column.LogicalType)
		if column.Type == "" {
			column.Type = column.DatabaseType
		}
		if column.Type == "" {
			column.Type = string(column.LogicalType)
		}
		columns[index] = column
	}
	assignStableColumnKeys(columns)
	return columns
}

func assignStableColumnKeys(columns []dto.DataColumn) {
	used := make(map[string]struct{}, len(columns))
	nextSuffix := make(map[string]int, len(columns))
	for index := range columns {
		base := columns[index].Name
		key := base
		if _, exists := used[key]; exists {
			suffix := nextSuffix[base]
			if suffix < 2 {
				suffix = 2
			}
			for {
				key = base + "__" + strconv.Itoa(suffix)
				suffix++
				if _, exists := used[key]; !exists {
					break
				}
			}
			nextSuffix[base] = suffix
		} else {
			nextSuffix[base] = 2
		}
		used[key] = struct{}{}
		columns[index].Key = key
	}
}

func inferResultLogicalType(databaseType string, scanType reflect.Type, length *int64) dto.LogicalType {
	normalized := normalizeDatabaseType(databaseType)
	switch {
	case strings.Contains(normalized, "json"):
		return dto.LogicalTypeJSON
	case strings.Contains(normalized, "bytea"), strings.Contains(normalized, "blob"), strings.Contains(normalized, "binary"), normalized == "image":
		return dto.LogicalTypeBinary
	case normalized == "bit" || strings.HasPrefix(normalized, "bit("):
		if length != nil && *length == 1 {
			return dto.LogicalTypeBoolean
		}
		return dto.LogicalTypeBinary
	case strings.Contains(normalized, "bigint"), strings.Contains(normalized, "bigserial"), normalized == "int8", normalized == "serial8":
		return dto.LogicalTypeBigInt
	case strings.Contains(normalized, "numeric"), strings.Contains(normalized, "decimal"), normalized == "dec", normalized == "fixed", normalized == "money":
		return dto.LogicalTypeDecimal
	case normalized == "bool", normalized == "boolean", strings.HasPrefix(normalized, "tinyint(1"):
		return dto.LogicalTypeBoolean
	case strings.Contains(normalized, "timestamptz"), strings.Contains(normalized, "timestamp"), strings.Contains(normalized, "datetime"), normalized == "smalldatetime":
		return dto.LogicalTypeDateTime
	case normalized == "date":
		return dto.LogicalTypeDate
	case normalized == "time", strings.HasPrefix(normalized, "time("), normalized == "timetz", strings.Contains(normalized, "time with time zone"), strings.Contains(normalized, "time without time zone"):
		return dto.LogicalTypeTime
	case normalized == "uuid", normalized == "uniqueidentifier":
		return dto.LogicalTypeUUID
	case normalized == "enum", strings.HasPrefix(normalized, "enum("), normalized == "set", strings.HasPrefix(normalized, "set("):
		return dto.LogicalTypeEnum
	case normalized == "float", normalized == "float4", normalized == "float8", normalized == "real", normalized == "double", normalized == "double precision":
		return dto.LogicalTypeFloat
	case normalized == "int", normalized == "integer", normalized == "int2", normalized == "int4", normalized == "smallint", normalized == "mediumint", normalized == "tinyint", normalized == "serial", normalized == "serial2", normalized == "serial4", normalized == "oid", normalized == "year", strings.HasPrefix(normalized, "int("), strings.HasPrefix(normalized, "integer("), strings.HasPrefix(normalized, "smallint("), strings.HasPrefix(normalized, "mediumint("), strings.HasPrefix(normalized, "tinyint("):
		return dto.LogicalTypeInteger
	case isResultStringType(normalized), strings.HasPrefix(normalized, "_"):
		return dto.LogicalTypeString
	}
	if scanType == nil {
		return dto.LogicalTypeUnknown
	}
	for scanType.Kind() == reflect.Pointer {
		scanType = scanType.Elem()
	}
	if scanType == reflect.TypeFor[time.Time]() {
		return dto.LogicalTypeDateTime
	}
	switch scanType.Kind() {
	case reflect.Bool:
		return dto.LogicalTypeBoolean
	case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Uint8, reflect.Uint16, reflect.Uint32:
		return dto.LogicalTypeInteger
	case reflect.Int, reflect.Int64, reflect.Uint, reflect.Uint64:
		return dto.LogicalTypeBigInt
	case reflect.Float32, reflect.Float64:
		return dto.LogicalTypeFloat
	case reflect.String:
		return dto.LogicalTypeString
	default:
		return dto.LogicalTypeUnknown
	}
}

func isResultStringType(databaseType string) bool {
	return strings.Contains(databaseType, "char") ||
		strings.Contains(databaseType, "text") ||
		strings.Contains(databaseType, "clob") ||
		databaseType == "string" ||
		databaseType == "name" ||
		databaseType == "xml" ||
		databaseType == "inet" ||
		databaseType == "cidr" ||
		databaseType == "macaddr" ||
		databaseType == "interval"
}

func resultValueEncoding(logicalType dto.LogicalType) dto.ValueEncoding {
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

func typedResultValue(value any, column dto.DataColumn) any {
	if value == nil {
		return nil
	}
	switch column.LogicalType {
	case dto.LogicalTypeBoolean:
		return resultBoolean(value)
	case dto.LogicalTypeInteger:
		return resultInteger(value)
	case dto.LogicalTypeBigInt, dto.LogicalTypeDecimal:
		return resultDecimalString(value)
	case dto.LogicalTypeFloat:
		return resultFloat(value)
	case dto.LogicalTypeJSON:
		return resultJSONString(value)
	case dto.LogicalTypeBinary:
		return resultBase64(value)
	case dto.LogicalTypeDate, dto.LogicalTypeTime, dto.LogicalTypeDateTime:
		return resultTemporalString(value, column.LogicalType, column.DatabaseType)
	case dto.LogicalTypeString, dto.LogicalTypeUUID, dto.LogicalTypeEnum:
		return resultString(value)
	default:
		return resultNative(value)
	}
}

func resultBoolean(value any) any {
	switch typed := value.(type) {
	case bool:
		return typed
	case int64:
		if typed == 0 || typed == 1 {
			return typed == 1
		}
	case uint64:
		if typed == 0 || typed == 1 {
			return typed == 1
		}
	}
	raw := strings.ToLower(strings.TrimSpace(resultString(value)))
	switch raw {
	case "1", "t", "true", "yes", "on":
		return true
	case "0", "f", "false", "no", "off":
		return false
	default:
		return resultNative(value)
	}
}

func resultInteger(value any) any {
	switch typed := value.(type) {
	case int64:
		if typed < -maxSafeJSONInteger || typed > maxSafeJSONInteger {
			return strconv.FormatInt(typed, 10)
		}
		return typed
	case int32:
		return int64(typed)
	case int16:
		return int64(typed)
	case int8:
		return int64(typed)
	case int:
		converted := int64(typed)
		if converted < -maxSafeJSONInteger || converted > maxSafeJSONInteger {
			return strconv.FormatInt(converted, 10)
		}
		return converted
	case uint32:
		return int64(typed)
	case uint16:
		return int64(typed)
	case uint8:
		return int64(typed)
	case uint:
		if uint64(typed) <= uint64(maxSafeJSONInteger) {
			return int64(typed)
		}
		return strconv.FormatUint(uint64(typed), 10)
	case uint64:
		if typed <= uint64(maxSafeJSONInteger) {
			return int64(typed)
		}
		return strconv.FormatUint(typed, 10)
	}
	raw := strings.TrimSpace(resultString(value))
	if parsed, err := strconv.ParseInt(raw, 10, 64); err == nil && parsed >= -maxSafeJSONInteger && parsed <= maxSafeJSONInteger {
		return parsed
	}
	if parsed, err := strconv.ParseUint(raw, 10, 64); err == nil && parsed <= uint64(maxSafeJSONInteger) {
		return int64(parsed)
	}
	return raw
}

func resultDecimalString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case []byte:
		return string(typed)
	case int:
		return strconv.FormatInt(int64(typed), 10)
	case int8:
		return strconv.FormatInt(int64(typed), 10)
	case int16:
		return strconv.FormatInt(int64(typed), 10)
	case int32:
		return strconv.FormatInt(int64(typed), 10)
	case int64:
		return strconv.FormatInt(typed, 10)
	case uint:
		return strconv.FormatUint(uint64(typed), 10)
	case uint8:
		return strconv.FormatUint(uint64(typed), 10)
	case uint16:
		return strconv.FormatUint(uint64(typed), 10)
	case uint32:
		return strconv.FormatUint(uint64(typed), 10)
	case uint64:
		return strconv.FormatUint(typed, 10)
	case float32:
		return strconv.FormatFloat(float64(typed), 'g', -1, 32)
	case float64:
		return strconv.FormatFloat(typed, 'g', -1, 64)
	case json.Number:
		return typed.String()
	default:
		return fmt.Sprint(typed)
	}
}

func resultFloat(value any) any {
	switch typed := value.(type) {
	case float64:
		if math.IsNaN(typed) || math.IsInf(typed, 0) {
			return strconv.FormatFloat(typed, 'g', -1, 64)
		}
		return typed
	case float32:
		converted := float64(typed)
		if math.IsNaN(converted) || math.IsInf(converted, 0) {
			return strconv.FormatFloat(converted, 'g', -1, 32)
		}
		return converted
	}
	raw := strings.TrimSpace(resultString(value))
	if parsed, err := strconv.ParseFloat(raw, 64); err == nil {
		if math.IsNaN(parsed) || math.IsInf(parsed, 0) {
			return raw
		}
		return parsed
	}
	return raw
}

func resultJSONString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case []byte:
		return string(typed)
	case json.RawMessage:
		return string(typed)
	default:
		encoded, err := json.Marshal(typed)
		if err != nil {
			return fmt.Sprint(typed)
		}
		return string(encoded)
	}
}

func resultBase64(value any) string {
	switch typed := value.(type) {
	case []byte:
		return base64.StdEncoding.EncodeToString(typed)
	case string:
		return base64.StdEncoding.EncodeToString([]byte(typed))
	default:
		return base64.StdEncoding.EncodeToString([]byte(fmt.Sprint(typed)))
	}
}

func resultTemporalString(value any, logicalType dto.LogicalType, databaseType string) string {
	switch typed := value.(type) {
	case time.Time:
		return formatResultTime(typed, logicalType, databaseType)
	case []byte:
		return canonicalResultTime(string(typed), logicalType, databaseType)
	case string:
		return canonicalResultTime(typed, logicalType, databaseType)
	default:
		return canonicalResultTime(fmt.Sprint(typed), logicalType, databaseType)
	}
}

func canonicalResultTime(raw string, logicalType dto.LogicalType, databaseType string) string {
	value := strings.TrimSpace(raw)
	for _, layout := range resultTimeLayouts(logicalType) {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return formatResultTime(parsed, logicalType, databaseType, resultLayoutHasTimeZone(layout))
		}
	}
	return raw
}

func resultTimeLayouts(logicalType dto.LogicalType) []string {
	switch logicalType {
	case dto.LogicalTypeDate:
		return []string{"2006-01-02", time.RFC3339Nano, "2006-01-02 15:04:05.999999999Z07:00", "2006-01-02 15:04:05.999999999"}
	case dto.LogicalTypeTime:
		return []string{"15:04:05.999999999Z07:00", "15:04:05Z07:00", "15:04:05.999999999-07", "15:04:05-07", "15:04:05.999999999", "15:04:05"}
	default:
		return []string{time.RFC3339Nano, "2006-01-02 15:04:05.999999999Z07:00", "2006-01-02 15:04:05Z07:00", "2006-01-02 15:04:05.999999999-07", "2006-01-02 15:04:05-07", "2006-01-02 15:04:05.999999999", "2006-01-02 15:04:05", "2006-01-02"}
	}
}

func formatResultTime(value time.Time, logicalType dto.LogicalType, databaseType string, forceTimeZone ...bool) string {
	withTimeZone := resultTypeHasTimeZone(databaseType) || len(forceTimeZone) > 0 && forceTimeZone[0]
	switch logicalType {
	case dto.LogicalTypeDate:
		return value.Format("2006-01-02")
	case dto.LogicalTypeTime:
		if withTimeZone {
			return value.Format("15:04:05.999999999Z07:00")
		}
		return value.Format("15:04:05.999999999")
	default:
		if withTimeZone {
			return value.Format(time.RFC3339Nano)
		}
		return value.Format("2006-01-02T15:04:05.999999999")
	}
}

func resultLayoutHasTimeZone(layout string) bool {
	return strings.Contains(layout, "Z07:00") || strings.HasSuffix(layout, "-07")
}

func resultTypeHasTimeZone(databaseType string) bool {
	normalized := normalizeDatabaseType(databaseType)
	return normalized == "timestamptz" || normalized == "timetz" || strings.Contains(normalized, "with time zone")
}

func resultString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case []byte:
		return string(typed)
	case time.Time:
		return typed.Format(time.RFC3339Nano)
	default:
		return fmt.Sprint(typed)
	}
}

func resultNative(value any) any {
	switch typed := value.(type) {
	case []byte:
		return string(typed)
	case time.Time:
		return typed.Format(time.RFC3339Nano)
	case int64:
		if typed < -maxSafeJSONInteger || typed > maxSafeJSONInteger {
			return strconv.FormatInt(typed, 10)
		}
		return typed
	case uint64:
		if typed > uint64(maxSafeJSONInteger) {
			return strconv.FormatUint(typed, 10)
		}
		return int64(typed)
	case float64:
		if math.IsNaN(typed) || math.IsInf(typed, 0) {
			return strconv.FormatFloat(typed, 'g', -1, 64)
		}
		return typed
	default:
		return value
	}
}

func normalizeDatabaseType(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(value)), " "))
}

func int64Pointer(value int64) *int64 {
	return &value
}

func resultCapacity(maxRows int) int {
	if maxRows <= 0 {
		return 0
	}
	if maxRows > 1024 {
		return 1024
	}
	return maxRows
}
