package engines

import (
	"database/sql"
	"fmt"

	"github.com/lib/pq"

	"github.com/huynhanx03/datadock/internal/core/entity"
)

func openPostgresDatabase(connection entity.Connection, password string, transport *transport) (*sql.DB, error) {
	configuration := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s", connection.Host, connection.Port, connection.Username, password, connection.Database, postgresSSLMode(connection.SSLMode))
	if connection.SSLCAPath != "" {
		configuration += " sslrootcert=" + connection.SSLCAPath
	}
	if connection.SSLCertPath != "" {
		configuration += " sslcert=" + connection.SSLCertPath + " sslkey=" + connection.SSLKeyPath
	}
	connector, err := pq.NewConnector(configuration)
	if err != nil {
		return nil, err
	}
	connector.Dialer(transport)
	return sql.OpenDB(connector), nil
}

func postgresSSLMode(mode entity.SSLMode) string {
	switch mode {
	case entity.SSLModeRequire:
		return "require"
	case entity.SSLModeVerifyCA:
		return "verify-ca"
	case entity.SSLModeVerifyFull:
		return "verify-full"
	default:
		return "disable"
	}
}
