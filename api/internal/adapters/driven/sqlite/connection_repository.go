package sqlite

import (
	"context"
	"database/sql"
	"errors"

	"github.com/huynhanx03/datadock/internal/core/entity"
)

type ConnectionRepository struct {
	database *sql.DB
}

func NewConnectionRepository(database *sql.DB) *ConnectionRepository {
	return &ConnectionRepository{database: database}
}

func (repository *ConnectionRepository) Create(ctx context.Context, connection entity.Connection) (entity.Connection, error) {
	_, err := repository.database.ExecContext(ctx, `INSERT INTO connections (id, workspace_id, name, engine, host, port, database_name, username, password_cipher, ssl_mode, ssl_ca_path, ssl_cert_path, ssl_key_path, proxy_url, ssh_enabled, ssh_host, ssh_port, ssh_username, ssh_password_cipher, ssh_private_key_path, ssh_known_hosts_path, read_only, auto_reconnect, max_open_conns, max_idle_conns, conn_max_lifetime, favorite, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, connectionValues(connection)...)
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
	result, err := repository.database.ExecContext(ctx, `UPDATE connections SET workspace_id = ?, name = ?, engine = ?, host = ?, port = ?, database_name = ?, username = ?, password_cipher = ?, ssl_mode = ?, ssl_ca_path = ?, ssl_cert_path = ?, ssl_key_path = ?, proxy_url = ?, ssh_enabled = ?, ssh_host = ?, ssh_port = ?, ssh_username = ?, ssh_password_cipher = ?, ssh_private_key_path = ?, ssh_known_hosts_path = ?, read_only = ?, auto_reconnect = ?, max_open_conns = ?, max_idle_conns = ?, conn_max_lifetime = ?, favorite = ?, updated_at = ? WHERE id = ?`, connectionUpdateValues(connection)...)
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

const connectionColumns = `id, workspace_id, name, engine, host, port, database_name, username, password_cipher, ssl_mode, ssl_ca_path, ssl_cert_path, ssl_key_path, proxy_url, ssh_enabled, ssh_host, ssh_port, ssh_username, ssh_password_cipher, ssh_private_key_path, ssh_known_hosts_path, read_only, auto_reconnect, max_open_conns, max_idle_conns, conn_max_lifetime, favorite, created_at, updated_at`

func connectionValues(connection entity.Connection) []any {
	return []any{connection.ID, connection.WorkspaceID, connection.Name, connection.Engine, connection.Host, connection.Port, connection.Database, connection.Username, connection.PasswordCipher, connection.SSLMode, connection.SSLCAPath, connection.SSLCertPath, connection.SSLKeyPath, connection.ProxyURL, connection.SSHTunnel.Enabled, connection.SSHTunnel.Host, connection.SSHTunnel.Port, connection.SSHTunnel.Username, connection.SSHTunnel.PasswordCipher, connection.SSHTunnel.PrivateKeyPath, connection.SSHTunnel.KnownHostsPath, connection.ReadOnly, connection.AutoReconnect, connection.MaxOpenConns, connection.MaxIdleConns, connection.ConnMaxLifetime, connection.Favorite, connection.CreatedAt.Format(timeFormat), connection.UpdatedAt.Format(timeFormat)}
}

func connectionUpdateValues(connection entity.Connection) []any {
	values := connectionValues(connection)
	return append(values[1:27], connection.UpdatedAt.Format(timeFormat), connection.ID)
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
	err := scanner.Scan(&connection.ID, &connection.WorkspaceID, &connection.Name, &connection.Engine, &connection.Host, &connection.Port, &connection.Database, &connection.Username, &connection.PasswordCipher, &connection.SSLMode, &connection.SSLCAPath, &connection.SSLCertPath, &connection.SSLKeyPath, &connection.ProxyURL, &connection.SSHTunnel.Enabled, &connection.SSHTunnel.Host, &connection.SSHTunnel.Port, &connection.SSHTunnel.Username, &connection.SSHTunnel.PasswordCipher, &connection.SSHTunnel.PrivateKeyPath, &connection.SSHTunnel.KnownHostsPath, &connection.ReadOnly, &connection.AutoReconnect, &connection.MaxOpenConns, &connection.MaxIdleConns, &connection.ConnMaxLifetime, &connection.Favorite, &createdAt, &updatedAt)
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
