package engines

import (
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"errors"
	"net"
	"os"
	"strconv"

	"github.com/go-sql-driver/mysql"
	"github.com/google/uuid"

	"github.com/huynhanx03/datadock/internal/core/entity"
)

func openMySQLDatabase(connection entity.Connection, password string, transport *transport) (*sql.DB, error) {
	configuration := mysql.NewConfig()
	configuration.User = connection.Username
	configuration.Passwd = password
	configuration.Net = "tcp"
	configuration.Addr = net.JoinHostPort(connection.Host, strconv.Itoa(connection.Port))
	configuration.DBName = connection.Database
	configuration.ParseTime = true
	configuration.ClientFoundRows = true
	configuration.DialFunc = transport.dial
	if connection.SSLMode != entity.SSLModeDisable {
		name, err := registerMySQLTLS(connection)
		if err != nil {
			return nil, err
		}
		configuration.TLSConfig = name
		transport.addCleanup(func() { mysql.DeregisterTLSConfig(name) })
	}
	connector, err := mysql.NewConnector(configuration)
	if err != nil {
		return nil, err
	}
	return sql.OpenDB(connector), nil
}

func registerMySQLTLS(connection entity.Connection) (string, error) {
	roots, err := x509.SystemCertPool()
	if err != nil || roots == nil {
		roots = x509.NewCertPool()
	}
	if connection.SSLCAPath != "" {
		pem, err := os.ReadFile(connection.SSLCAPath)
		if err != nil {
			return "", err
		}
		if !roots.AppendCertsFromPEM(pem) {
			return "", errors.New("SSL CA certificate contains no valid certificate")
		}
	}
	configuration := &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, ServerName: connection.Host}
	if connection.SSLCertPath != "" {
		certificate, err := tls.LoadX509KeyPair(connection.SSLCertPath, connection.SSLKeyPath)
		if err != nil {
			return "", err
		}
		configuration.Certificates = []tls.Certificate{certificate}
	}
	name := "datadock-" + uuid.NewString()
	if err := mysql.RegisterTLSConfig(name, configuration); err != nil {
		return "", err
	}
	return name, nil
}
