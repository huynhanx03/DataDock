package sqlite

import (
	"context"
	"database/sql"
	"errors"

	"github.com/huynhanx03/datadock/internal/core/entity"
	"github.com/huynhanx03/datadock/internal/ports"
)

type ConnectionRepository struct {
	database *sql.DB
}

func NewConnectionRepository(database *sql.DB) *ConnectionRepository {
	return &ConnectionRepository{database: database}
}

func (repository *ConnectionRepository) Create(ctx context.Context, connection entity.Connection) (entity.Connection, error) {
	_, err := repository.database.ExecContext(ctx, `INSERT INTO connections (id, workspace_id, name, engine, host, port, database_name, username, password_cipher, ssl_mode, ssl_ca_path, ssl_cert_path, ssl_key_path, proxy_url, ssh_enabled, ssh_host, ssh_port, ssh_username, ssh_password_cipher, ssh_private_key_path, ssh_known_hosts_path, read_only, auto_reconnect, max_open_conns, max_idle_conns, conn_max_lifetime, conn_max_idle_time, favorite, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, connectionValues(connection)...)
	return connection, err
}

func (repository *ConnectionRepository) List(ctx context.Context, workspaceID string) ([]entity.Connection, error) {
	query := `SELECT ` + connectionColumns + ` FROM connections`
	args := []any{}
	if workspaceID != "" {
		query += ` WHERE workspace_id = ?`
		args = append(args, workspaceID)
	}
	query += ` ORDER BY favorite DESC, name`
	rows, err := repository.database.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	connections := make([]entity.Connection, 0)
	for rows.Next() {
		connection, err := scanConnection(rows)
		if err != nil {
			return nil, err
		}
		connections = append(connections, connection)
	}
	return connections, rows.Err()
}

func (repository *ConnectionRepository) Get(ctx context.Context, id string) (entity.Connection, error) {
	return scanConnection(repository.database.QueryRowContext(ctx, `SELECT `+connectionColumns+` FROM connections WHERE id = ?`, id))
}

func (repository *ConnectionRepository) Update(ctx context.Context, connection entity.Connection) (entity.Connection, error) {
	result, err := repository.database.ExecContext(ctx, `UPDATE connections SET workspace_id = ?, name = ?, engine = ?, host = ?, port = ?, database_name = ?, username = ?, password_cipher = ?, ssl_mode = ?, ssl_ca_path = ?, ssl_cert_path = ?, ssl_key_path = ?, proxy_url = ?, ssh_enabled = ?, ssh_host = ?, ssh_port = ?, ssh_username = ?, ssh_password_cipher = ?, ssh_private_key_path = ?, ssh_known_hosts_path = ?, read_only = ?, auto_reconnect = ?, max_open_conns = ?, max_idle_conns = ?, conn_max_lifetime = ?, conn_max_idle_time = ?, favorite = ?, updated_at = ? WHERE id = ?`, connectionUpdateValues(connection)...)
	if err != nil {
		return entity.Connection{}, err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return entity.Connection{}, err
	}
	if changed == 0 {
		return entity.Connection{}, ErrNotFound
	}
	return connection, nil
}

const connectionColumns = `id, workspace_id, name, engine, host, port, database_name, username, password_cipher, ssl_mode, ssl_ca_path, ssl_cert_path, ssl_key_path, proxy_url, ssh_enabled, ssh_host, ssh_port, ssh_username, ssh_password_cipher, ssh_private_key_path, ssh_known_hosts_path, read_only, auto_reconnect, max_open_conns, max_idle_conns, conn_max_lifetime, conn_max_idle_time, favorite, created_at, updated_at`

func connectionValues(connection entity.Connection) []any {
	return []any{connection.ID, connection.WorkspaceID, connection.Name, connection.Engine, connection.Host, connection.Port, connection.Database, connection.Username, connection.PasswordCipher, connection.SSLMode, connection.SSLCAPath, connection.SSLCertPath, connection.SSLKeyPath, connection.ProxyURL, connection.SSHTunnel.Enabled, connection.SSHTunnel.Host, connection.SSHTunnel.Port, connection.SSHTunnel.Username, connection.SSHTunnel.PasswordCipher, connection.SSHTunnel.PrivateKeyPath, connection.SSHTunnel.KnownHostsPath, connection.ReadOnly, connection.AutoReconnect, connection.MaxOpenConns, connection.MaxIdleConns, connection.ConnMaxLifetime, connection.ConnMaxIdleTime, connection.Favorite, connection.CreatedAt.Format(timeFormat), connection.UpdatedAt.Format(timeFormat)}
}

func connectionUpdateValues(connection entity.Connection) []any {
	return []any{connection.WorkspaceID, connection.Name, connection.Engine, connection.Host, connection.Port, connection.Database, connection.Username, connection.PasswordCipher, connection.SSLMode, connection.SSLCAPath, connection.SSLCertPath, connection.SSLKeyPath, connection.ProxyURL, connection.SSHTunnel.Enabled, connection.SSHTunnel.Host, connection.SSHTunnel.Port, connection.SSHTunnel.Username, connection.SSHTunnel.PasswordCipher, connection.SSHTunnel.PrivateKeyPath, connection.SSHTunnel.KnownHostsPath, connection.ReadOnly, connection.AutoReconnect, connection.MaxOpenConns, connection.MaxIdleConns, connection.ConnMaxLifetime, connection.ConnMaxIdleTime, connection.Favorite, connection.UpdatedAt.Format(timeFormat), connection.ID}
}

func (repository *ConnectionRepository) Delete(ctx context.Context, id string) error {
	result, err := repository.database.ExecContext(ctx, `DELETE FROM connections WHERE id = ?`, id)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed == 0 {
		return ErrNotFound
	}
	return nil
}

type connectionScanner interface {
	Scan(...any) error
}

func scanConnection(scanner connectionScanner) (entity.Connection, error) {
	var connection entity.Connection
	var createdAt string
	var updatedAt string
	err := scanner.Scan(&connection.ID, &connection.WorkspaceID, &connection.Name, &connection.Engine, &connection.Host, &connection.Port, &connection.Database, &connection.Username, &connection.PasswordCipher, &connection.SSLMode, &connection.SSLCAPath, &connection.SSLCertPath, &connection.SSLKeyPath, &connection.ProxyURL, &connection.SSHTunnel.Enabled, &connection.SSHTunnel.Host, &connection.SSHTunnel.Port, &connection.SSHTunnel.Username, &connection.SSHTunnel.PasswordCipher, &connection.SSHTunnel.PrivateKeyPath, &connection.SSHTunnel.KnownHostsPath, &connection.ReadOnly, &connection.AutoReconnect, &connection.MaxOpenConns, &connection.MaxIdleConns, &connection.ConnMaxLifetime, &connection.ConnMaxIdleTime, &connection.Favorite, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return entity.Connection{}, ErrNotFound
	}
	if err != nil {
		return entity.Connection{}, err
	}
	connection.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return entity.Connection{}, err
	}
	connection.UpdatedAt, err = parseTime(updatedAt)
	return connection, err
}

func (repository *ConnectionRepository) GetMetadata(ctx context.Context, id string) (ports.ConnectionMetadata, error) {
	var metadata ports.ConnectionMetadata
	var lastConnectedAt sql.NullString
	var lastDisconnectedAt sql.NullString
	err := repository.database.QueryRowContext(ctx, `SELECT id, proxy_username_cipher, proxy_password_cipher, connection_status, last_connected_at, last_disconnected_at, last_error_code FROM connections WHERE id = ?`, id).Scan(&metadata.ConnectionID, &metadata.ProxyUsernameCipher, &metadata.ProxyPasswordCipher, &metadata.Status, &lastConnectedAt, &lastDisconnectedAt, &metadata.LastErrorCode)
	if errors.Is(err, sql.ErrNoRows) {
		return ports.ConnectionMetadata{}, ports.ErrNotFound
	}
	if err != nil {
		return ports.ConnectionMetadata{}, err
	}
	if lastConnectedAt.Valid {
		value, parseErr := parseTime(lastConnectedAt.String)
		if parseErr != nil {
			return ports.ConnectionMetadata{}, parseErr
		}
		metadata.LastConnectedAt = &value
	}
	if lastDisconnectedAt.Valid {
		value, parseErr := parseTime(lastDisconnectedAt.String)
		if parseErr != nil {
			return ports.ConnectionMetadata{}, parseErr
		}
		metadata.LastDisconnectedAt = &value
	}
	return metadata, nil
}

func (repository *ConnectionRepository) UpdateMetadata(ctx context.Context, metadata ports.ConnectionMetadata) error {
	result, err := repository.database.ExecContext(ctx, `UPDATE connections SET proxy_username_cipher = ?, proxy_password_cipher = ?, connection_status = ?, last_connected_at = ?, last_disconnected_at = ?, last_error_code = ? WHERE id = ?`, metadata.ProxyUsernameCipher, metadata.ProxyPasswordCipher, metadata.Status, nullableTime(metadata.LastConnectedAt), nullableTime(metadata.LastDisconnectedAt), metadata.LastErrorCode, metadata.ConnectionID)
	if err != nil {
		return err
	}
	return requireChanged(result)
}

func (repository *ConnectionRepository) CreateRecord(ctx context.Context, record ports.ConnectionRecord) (ports.ConnectionRecord, error) {
	metadata := record.Metadata
	metadata.ConnectionID = record.Connection.ID
	values := append(connectionValues(record.Connection), metadata.ProxyUsernameCipher, metadata.ProxyPasswordCipher, metadata.Status, nullableTime(metadata.LastConnectedAt), nullableTime(metadata.LastDisconnectedAt), metadata.LastErrorCode)
	_, err := repository.database.ExecContext(ctx, `INSERT INTO connections (id, workspace_id, name, engine, host, port, database_name, username, password_cipher, ssl_mode, ssl_ca_path, ssl_cert_path, ssl_key_path, proxy_url, ssh_enabled, ssh_host, ssh_port, ssh_username, ssh_password_cipher, ssh_private_key_path, ssh_known_hosts_path, read_only, auto_reconnect, max_open_conns, max_idle_conns, conn_max_lifetime, conn_max_idle_time, favorite, created_at, updated_at, proxy_username_cipher, proxy_password_cipher, connection_status, last_connected_at, last_disconnected_at, last_error_code) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, values...)
	record.Metadata = metadata
	return record, err
}

func (repository *ConnectionRepository) ListRecords(ctx context.Context, workspaceID string) ([]ports.ConnectionRecord, error) {
	query := `SELECT ` + connectionRecordColumns + ` FROM connections`
	args := []any{}
	if workspaceID != "" {
		query += ` WHERE workspace_id = ?`
		args = append(args, workspaceID)
	}
	query += ` ORDER BY favorite DESC, name`
	rows, err := repository.database.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	records := make([]ports.ConnectionRecord, 0)
	for rows.Next() {
		record, err := scanConnectionRecord(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, rows.Err()
}

func (repository *ConnectionRepository) GetRecord(ctx context.Context, id string) (ports.ConnectionRecord, error) {
	return scanConnectionRecord(repository.database.QueryRowContext(ctx, `SELECT `+connectionRecordColumns+` FROM connections WHERE id = ?`, id))
}

func (repository *ConnectionRepository) UpdateRecord(ctx context.Context, record ports.ConnectionRecord) (ports.ConnectionRecord, error) {
	metadata := record.Metadata
	metadata.ConnectionID = record.Connection.ID
	values := connectionUpdateValues(record.Connection)
	id := values[len(values)-1]
	values = append(values[:len(values)-1], metadata.ProxyUsernameCipher, metadata.ProxyPasswordCipher, metadata.Status, nullableTime(metadata.LastConnectedAt), nullableTime(metadata.LastDisconnectedAt), metadata.LastErrorCode, id)
	result, err := repository.database.ExecContext(ctx, `UPDATE connections SET workspace_id = ?, name = ?, engine = ?, host = ?, port = ?, database_name = ?, username = ?, password_cipher = ?, ssl_mode = ?, ssl_ca_path = ?, ssl_cert_path = ?, ssl_key_path = ?, proxy_url = ?, ssh_enabled = ?, ssh_host = ?, ssh_port = ?, ssh_username = ?, ssh_password_cipher = ?, ssh_private_key_path = ?, ssh_known_hosts_path = ?, read_only = ?, auto_reconnect = ?, max_open_conns = ?, max_idle_conns = ?, conn_max_lifetime = ?, conn_max_idle_time = ?, favorite = ?, updated_at = ?, proxy_username_cipher = ?, proxy_password_cipher = ?, connection_status = ?, last_connected_at = ?, last_disconnected_at = ?, last_error_code = ? WHERE id = ?`, values...)
	if err != nil {
		return ports.ConnectionRecord{}, err
	}
	if err := requireChanged(result); err != nil {
		return ports.ConnectionRecord{}, err
	}
	record.Metadata = metadata
	return record, nil
}

const connectionRecordColumns = connectionColumns + `, proxy_username_cipher, proxy_password_cipher, connection_status, last_connected_at, last_disconnected_at, last_error_code`

func scanConnectionRecord(scanner connectionScanner) (ports.ConnectionRecord, error) {
	var record ports.ConnectionRecord
	var createdAt string
	var updatedAt string
	var lastConnectedAt sql.NullString
	var lastDisconnectedAt sql.NullString
	err := scanner.Scan(
		&record.Connection.ID,
		&record.Connection.WorkspaceID,
		&record.Connection.Name,
		&record.Connection.Engine,
		&record.Connection.Host,
		&record.Connection.Port,
		&record.Connection.Database,
		&record.Connection.Username,
		&record.Connection.PasswordCipher,
		&record.Connection.SSLMode,
		&record.Connection.SSLCAPath,
		&record.Connection.SSLCertPath,
		&record.Connection.SSLKeyPath,
		&record.Connection.ProxyURL,
		&record.Connection.SSHTunnel.Enabled,
		&record.Connection.SSHTunnel.Host,
		&record.Connection.SSHTunnel.Port,
		&record.Connection.SSHTunnel.Username,
		&record.Connection.SSHTunnel.PasswordCipher,
		&record.Connection.SSHTunnel.PrivateKeyPath,
		&record.Connection.SSHTunnel.KnownHostsPath,
		&record.Connection.ReadOnly,
		&record.Connection.AutoReconnect,
		&record.Connection.MaxOpenConns,
		&record.Connection.MaxIdleConns,
		&record.Connection.ConnMaxLifetime,
		&record.Connection.ConnMaxIdleTime,
		&record.Connection.Favorite,
		&createdAt,
		&updatedAt,
		&record.Metadata.ProxyUsernameCipher,
		&record.Metadata.ProxyPasswordCipher,
		&record.Metadata.Status,
		&lastConnectedAt,
		&lastDisconnectedAt,
		&record.Metadata.LastErrorCode,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return ports.ConnectionRecord{}, ports.ErrNotFound
	}
	if err != nil {
		return ports.ConnectionRecord{}, err
	}
	record.Connection.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return ports.ConnectionRecord{}, err
	}
	record.Connection.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return ports.ConnectionRecord{}, err
	}
	record.Metadata.ConnectionID = record.Connection.ID
	if lastConnectedAt.Valid {
		value, parseErr := parseTime(lastConnectedAt.String)
		if parseErr != nil {
			return ports.ConnectionRecord{}, parseErr
		}
		record.Metadata.LastConnectedAt = &value
	}
	if lastDisconnectedAt.Valid {
		value, parseErr := parseTime(lastDisconnectedAt.String)
		if parseErr != nil {
			return ports.ConnectionRecord{}, parseErr
		}
		record.Metadata.LastDisconnectedAt = &value
	}
	return record, nil
}
