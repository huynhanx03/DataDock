package engines

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
	"golang.org/x/net/proxy"

	"github.com/huynhanx03/datadock/internal/core/dto"
	"github.com/huynhanx03/datadock/internal/core/entity"
)

type transactionHandle struct {
	database       *sql.DB
	closeTransport func()
	transaction    *sql.Tx
	connectionID   string
	startedAt      time.Time
	savepoints     []string
}
type Runtime struct {
	transactions  map[string]*transactionHandle
	transactionMu sync.Mutex
}
type transport struct {
	dial      func(context.Context, string, string) (net.Conn, error)
	sshClient *ssh.Client
}

type bufferedConn struct {
	net.Conn
	reader *bufio.Reader
}

func (connection *bufferedConn) Read(buffer []byte) (int, error) {
	return connection.reader.Read(buffer)
}

func NewRuntime() *Runtime {
	return &Runtime{transactions: map[string]*transactionHandle{}}
}

func (runtime *Runtime) BeginTransaction(ctx context.Context, connection entity.Connection, password string) (dto.TransactionState, error) {
	database, closeTransport, err := runtime.open(connection, password)
	if err != nil {
		return dto.TransactionState{}, err
	}
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		database.Close()
		closeTransport()
		return dto.TransactionState{}, err
	}
	id := uuid.NewString()
	handle := &transactionHandle{database: database, closeTransport: closeTransport, transaction: tx, connectionID: connection.ID, startedAt: time.Now(), savepoints: []string{}}
	runtime.transactionMu.Lock()
	runtime.transactions[id] = handle
	runtime.transactionMu.Unlock()
	return dto.TransactionState{ID: id, ConnectionID: connection.ID, State: "active", StartedAt: handle.startedAt.UTC().Format(time.RFC3339), Savepoints: handle.savepoints}, nil
}
func (runtime *Runtime) TransactionAction(ctx context.Context, id, action, name string) (dto.TransactionState, error) {
	runtime.transactionMu.Lock()
	handle := runtime.transactions[id]
	runtime.transactionMu.Unlock()
	if handle == nil {
		return dto.TransactionState{}, errors.New("transaction was not found or has expired")
	}
	state := dto.TransactionState{ID: id, ConnectionID: handle.connectionID, State: "active", StartedAt: handle.startedAt.UTC().Format(time.RFC3339), Savepoints: handle.savepoints}
	switch action {
	case "commit":
		if err := handle.transaction.Commit(); err != nil {
			return state, err
		}
		runtime.closeTransaction(id, handle)
		state.State = "committed"
	case "rollback":
		if err := handle.transaction.Rollback(); err != nil {
			return state, err
		}
		runtime.closeTransaction(id, handle)
		state.State = "rolled_back"
	case "savepoint":
		if !validIdentifier(name) {
			return state, errors.New("invalid savepoint name")
		}
		if _, err := handle.transaction.ExecContext(ctx, "SAVEPOINT "+name); err != nil {
			return state, err
		}
		handle.savepoints = append(handle.savepoints, name)
		state.Savepoints = handle.savepoints
	case "rollback_to":
		if !validIdentifier(name) {
			return state, errors.New("invalid savepoint name")
		}
		if _, err := handle.transaction.ExecContext(ctx, "ROLLBACK TO SAVEPOINT "+name); err != nil {
			return state, err
		}
	default:
		return state, errors.New("invalid transaction action")
	}
	return state, nil
}
func (runtime *Runtime) closeTransaction(id string, handle *transactionHandle) {
	runtime.transactionMu.Lock()
	delete(runtime.transactions, id)
	runtime.transactionMu.Unlock()
	handle.database.Close()
	handle.closeTransport()
}
func (runtime *Runtime) CancelSession(ctx context.Context, connection entity.Connection, password, sessionID string, force bool) error {
	if !validIdentifier(sessionID) && !allDigits(sessionID) {
		return errors.New("invalid session id")
	}
	database, closeTransport, err := runtime.open(connection, password)
	if err != nil {
		return err
	}
	defer database.Close()
	defer closeTransport()
	q := ""
	if connection.Engine == entity.EnginePostgreSQL {
		q = "SELECT pg_cancel_backend(" + sessionID + ")"
		if force {
			q = "SELECT pg_terminate_backend(" + sessionID + ")"
		}
	} else {
		q = "KILL QUERY " + sessionID
		if force {
			q = "KILL " + sessionID
		}
	}
	_, err = database.ExecContext(ctx, q)
	return err
}
func allDigits(v string) bool {
	if v == "" {
		return false
	}
	for _, r := range v {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func (runtime *Runtime) Test(ctx context.Context, connection entity.Connection, password string) error {
	database, closeTransport, err := runtime.open(connection, password)
	if err != nil {
		return err
	}
	defer database.Close()
	defer closeTransport()
	context, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	return database.PingContext(context)
}

func (runtime *Runtime) open(connection entity.Connection, password string) (*sql.DB, func(), error) {
	transport, err := newTransport(connection)
	if err != nil {
		return nil, nil, err
	}
	database, err := openDatabase(connection, password, transport)
	if err != nil {
		transport.Close()
		return nil, nil, err
	}
	database.SetMaxOpenConns(max(1, connection.MaxOpenConns))
	database.SetMaxIdleConns(max(0, connection.MaxIdleConns))
	if connection.ConnMaxLifetime > 0 {
		database.SetConnMaxLifetime(time.Duration(connection.ConnMaxLifetime) * time.Second)
	}
	return database, transport.Close, nil
}

func openDatabase(connection entity.Connection, password string, transport *transport) (*sql.DB, error) {
	if !connection.Engine.Valid() {
		return nil, errors.New("database engine is not supported")
	}
	if connection.Engine == entity.EnginePostgreSQL {
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
	configuration := mysql.NewConfig()
	configuration.User = connection.Username
	configuration.Passwd = password
	configuration.Net = "tcp"
	configuration.Addr = net.JoinHostPort(connection.Host, strconv.Itoa(connection.Port))
	configuration.DBName = connection.Database
	configuration.ParseTime = true
	configuration.DialFunc = transport.dial
	if connection.SSLMode != entity.SSLModeDisable {
		name, err := registerMySQLTLS(connection)
		if err != nil {
			return nil, err
		}
		configuration.TLSConfig = name
	}
	connector, err := mysql.NewConnector(configuration)
	if err != nil {
		return nil, err
	}
	return sql.OpenDB(connector), nil
}

func newTransport(connection entity.Connection) (*transport, error) {
	dial := (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext
	if connection.ProxyURL != "" {
		proxyDial, err := proxyDialer(connection.ProxyURL)
		if err != nil {
			return nil, err
		}
		dial = proxyDial
	}
	result := &transport{dial: dial}
	if !connection.SSHTunnel.Enabled {
		return result, nil
	}
	hostKeyCallback, err := knownhosts.New(connection.SSHTunnel.KnownHostsPath)
	if err != nil {
		return nil, err
	}
	auth, err := sshAuthMethods(connection.SSHTunnel)
	if err != nil {
		return nil, err
	}
	sshAddress := net.JoinHostPort(connection.SSHTunnel.Host, strconv.Itoa(connection.SSHTunnel.Port))
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	connectionToSSH, err := dial(ctx, "tcp", sshAddress)
	if err != nil {
		return nil, err
	}
	_ = connectionToSSH.SetDeadline(time.Now().Add(12 * time.Second))
	clientConnection, channels, requests, err := ssh.NewClientConn(connectionToSSH, sshAddress, &ssh.ClientConfig{User: connection.SSHTunnel.Username, Auth: auth, HostKeyCallback: hostKeyCallback, Timeout: 12 * time.Second})
	if err != nil {
		connectionToSSH.Close()
		return nil, err
	}
	_ = connectionToSSH.SetDeadline(time.Time{})
	result.sshClient = ssh.NewClient(clientConnection, channels, requests)
	result.dial = func(ctx context.Context, network, address string) (net.Conn, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return result.sshClient.Dial(network, address)
	}
	return result, nil
}

func (transport *transport) Dial(network, address string) (net.Conn, error) {
	return transport.dial(context.Background(), network, address)
}

func (transport *transport) DialTimeout(network, address string, timeout time.Duration) (net.Conn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return transport.dial(ctx, network, address)
}

func (transport *transport) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return transport.dial(ctx, network, address)
}

func (transport *transport) Close() {
	if transport != nil && transport.sshClient != nil {
		_ = transport.sshClient.Close()
	}
}

func sshAuthMethods(tunnel entity.SSHTunnel) ([]ssh.AuthMethod, error) {
	methods := make([]ssh.AuthMethod, 0, 2)
	if tunnel.Password != "" {
		methods = append(methods, ssh.Password(tunnel.Password))
	}
	if tunnel.PrivateKeyPath != "" {
		privateKey, err := os.ReadFile(tunnel.PrivateKeyPath)
		if err != nil {
			return nil, err
		}
		signer, err := ssh.ParsePrivateKey(privateKey)
		if err != nil {
			return nil, err
		}
		methods = append(methods, ssh.PublicKeys(signer))
	}
	if len(methods) == 0 {
		return nil, errors.New("SSH tunnel requires a password or private key")
	}
	return methods, nil
}

func proxyDialer(rawURL string) (func(context.Context, string, string) (net.Conn, error), error) {
	proxyURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	proxyAddress := proxyURL.Host
	if _, _, err := net.SplitHostPort(proxyAddress); err != nil {
		defaultPort := "80"
		if proxyURL.Scheme == "https" {
			defaultPort = "443"
		}
		proxyAddress = net.JoinHostPort(proxyURL.Hostname(), defaultPort)
	}
	if proxyURL.Scheme == "socks5" || proxyURL.Scheme == "socks5h" {
		dialer, err := proxy.FromURL(proxyURL, &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second})
		if err != nil {
			return nil, err
		}
		return func(ctx context.Context, network, address string) (net.Conn, error) {
			if contextDialer, ok := dialer.(proxy.ContextDialer); ok {
				return contextDialer.DialContext(ctx, network, address)
			}
			return dialer.Dial(network, address)
		}, nil
	}
	if proxyURL.Scheme != "http" && proxyURL.Scheme != "https" {
		return nil, errors.New("unsupported proxy protocol")
	}
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		dialer := net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
		connection, err := dialer.DialContext(ctx, "tcp", proxyAddress)
		if err != nil {
			return nil, err
		}
		if proxyURL.Scheme == "https" {
			serverName := proxyURL.Hostname()
			secureConnection := tls.Client(connection, &tls.Config{MinVersion: tls.VersionTLS12, ServerName: serverName})
			if err := secureConnection.HandshakeContext(ctx); err != nil {
				connection.Close()
				return nil, err
			}
			connection = secureConnection
		}
		request := &http.Request{Method: http.MethodConnect, URL: &url.URL{Opaque: address}, Host: address, Header: make(http.Header)}
		if proxyURL.User != nil {
			password, _ := proxyURL.User.Password()
			request.SetBasicAuth(proxyURL.User.Username(), password)
		}
		if err := request.Write(connection); err != nil {
			connection.Close()
			return nil, err
		}
		reader := bufio.NewReader(connection)
		response, err := http.ReadResponse(reader, request)
		if err != nil {
			connection.Close()
			return nil, err
		}
		if response.StatusCode != http.StatusOK {
			response.Body.Close()
			connection.Close()
			return nil, fmt.Errorf("proxy CONNECT failed: %s", response.Status)
		}
		response.Body.Close()
		return &bufferedConn{Conn: connection, reader: reader}, nil
	}, nil
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

func (runtime *Runtime) ListTables(ctx context.Context, connection entity.Connection, password string) ([]string, error) {
	database, closeTransport, err := runtime.open(connection, password)
	if err != nil {
		return nil, err
	}
	defer database.Close()
	defer closeTransport()
	query := `SELECT table_schema || '.' || table_name FROM information_schema.tables WHERE table_type = 'BASE TABLE' AND table_schema NOT IN ('pg_catalog', 'information_schema') ORDER BY 1`
	if connection.Engine != entity.EnginePostgreSQL {
		query = `SELECT table_name FROM information_schema.tables WHERE table_schema = DATABASE() AND table_type = 'BASE TABLE' ORDER BY table_name`
	}
	rows, err := database.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tables := make([]string, 0)
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			return nil, err
		}
		tables = append(tables, table)
	}
	return tables, rows.Err()
}

func (runtime *Runtime) BrowseTable(ctx context.Context, connection entity.Connection, password string, input dto.TableRowsInput) (dto.TableRowsResult, error) {
	database, closeTransport, err := runtime.open(connection, password)
	if err != nil {
		return dto.TableRowsResult{}, err
	}
	defer database.Close()
	defer closeTransport()
	schema, table, err := tableReference(connection, input.Table)
	if err != nil {
		return dto.TableRowsResult{}, err
	}
	queryContext, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	columns, err := tableColumns(queryContext, database, connection, schema, table)
	if err != nil {
		return dto.TableRowsResult{}, err
	}
	if len(columns) == 0 {
		return dto.TableRowsResult{}, errors.New("table was not found")
	}
	sortColumn := ""
	if input.Sort != "" {
		for _, column := range columns {
			if column == input.Sort {
				sortColumn = column
				break
			}
		}
		if sortColumn == "" {
			return dto.TableRowsResult{}, errors.New("sort column was not found")
		}
	}
	quotedTable := quoteTable(connection.Engine, schema, table)
	where, args := searchClause(connection.Engine, columns, input.Search)
	countQuery := "SELECT COUNT(*) FROM " + quotedTable + where
	var total int64
	if err := database.QueryRowContext(queryContext, countQuery, args...).Scan(&total); err != nil {
		return dto.TableRowsResult{}, err
	}
	orderBy := ""
	if sortColumn != "" {
		direction := "ASC"
		if strings.EqualFold(input.Order, "desc") {
			direction = "DESC"
		}
		orderBy = " ORDER BY " + quoteIdentifier(connection.Engine, sortColumn) + " " + direction
	}
	limitPlaceholder, offsetPlaceholder := "?", "?"
	if connection.Engine == entity.EnginePostgreSQL {
		limitPlaceholder, offsetPlaceholder = fmt.Sprintf("$%d", len(args)+1), fmt.Sprintf("$%d", len(args)+2)
	}
	query := "SELECT * FROM " + quotedTable + where + orderBy + " LIMIT " + limitPlaceholder + " OFFSET " + offsetPlaceholder
	args = append(args, input.Limit, input.Offset)
	rows, err := database.QueryContext(queryContext, query, args...)
	if err != nil {
		return dto.TableRowsResult{}, err
	}
	defer rows.Close()
	result := dto.TableRowsResult{Columns: columns, Rows: make([][]any, 0), Total: total, Limit: input.Limit, Offset: input.Offset}
	for rows.Next() {
		values := make([]any, len(columns))
		references := make([]any, len(columns))
		for index := range values {
			references[index] = &values[index]
		}
		if err := rows.Scan(references...); err != nil {
			return dto.TableRowsResult{}, err
		}
		for index, value := range values {
			values[index] = queryValue(value)
		}
		result.Rows = append(result.Rows, values)
	}
	return result, rows.Err()
}

func (runtime *Runtime) MutateRows(ctx context.Context, connection entity.Connection, password, reference string, mutations []dto.RowMutation) (dto.TableMutateResult, error) {
	database, closeTransport, err := runtime.open(connection, password)
	if err != nil {
		return dto.TableMutateResult{}, err
	}
	defer database.Close()
	defer closeTransport()
	schema, table, err := tableReference(connection, reference)
	if err != nil {
		return dto.TableMutateResult{}, err
	}
	queryContext, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	columns, err := tableColumns(queryContext, database, connection, schema, table)
	if err != nil {
		return dto.TableMutateResult{}, err
	}
	allowed := make(map[string]bool, len(columns))
	for _, column := range columns {
		allowed[column] = true
	}
	primaryKeys, err := primaryKeyColumns(queryContext, database, connection, schema, table)
	if err != nil {
		return dto.TableMutateResult{}, err
	}
	transaction, err := database.BeginTx(queryContext, nil)
	if err != nil {
		return dto.TableMutateResult{}, err
	}
	for _, mutation := range mutations {
		for key := range mutation.Values {
			if !allowed[key] {
				transaction.Rollback()
				return dto.TableMutateResult{}, errors.New("column was not found")
			}
		}
		for key := range mutation.Keys {
			if !allowed[key] {
				transaction.Rollback()
				return dto.TableMutateResult{}, errors.New("key column was not found")
			}
		}
		if mutation.Kind != "insert" && !hasExactKeys(mutation.Keys, primaryKeys) {
			transaction.Rollback()
			return dto.TableMutateResult{}, errors.New("primary key values are required")
		}
		statement, arguments, err := rowMutationStatement(connection.Engine, schema, table, mutation)
		if err != nil {
			transaction.Rollback()
			return dto.TableMutateResult{}, err
		}
		result, err := transaction.ExecContext(queryContext, statement, arguments...)
		if err != nil {
			transaction.Rollback()
			return dto.TableMutateResult{}, err
		}
		if mutation.Kind != "insert" {
			affected, err := result.RowsAffected()
			if err != nil {
				transaction.Rollback()
				return dto.TableMutateResult{}, err
			}
			if affected != 1 {
				transaction.Rollback()
				return dto.TableMutateResult{}, errors.New("row changed or no longer exists")
			}
		}
	}
	if err := transaction.Commit(); err != nil {
		return dto.TableMutateResult{}, err
	}
	return dto.TableMutateResult{Applied: int64(len(mutations))}, nil
}

func primaryKeyColumns(ctx context.Context, database *sql.DB, connection entity.Connection, schema, table string) ([]string, error) {
	query := "SELECT kcu.column_name FROM information_schema.table_constraints tc JOIN information_schema.key_column_usage kcu ON tc.constraint_catalog = kcu.constraint_catalog AND tc.constraint_schema = kcu.constraint_schema AND tc.constraint_name = kcu.constraint_name WHERE tc.table_schema = ? AND tc.table_name = ? AND tc.constraint_type = 'PRIMARY KEY' ORDER BY kcu.ordinal_position"
	if connection.Engine == entity.EnginePostgreSQL {
		query = "SELECT kcu.column_name FROM information_schema.table_constraints tc JOIN information_schema.key_column_usage kcu ON tc.constraint_catalog = kcu.constraint_catalog AND tc.constraint_schema = kcu.constraint_schema AND tc.constraint_name = kcu.constraint_name WHERE tc.table_schema = $1 AND tc.table_name = $2 AND tc.constraint_type = 'PRIMARY KEY' ORDER BY kcu.ordinal_position"
	}
	rows, err := database.QueryContext(ctx, query, schema, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	keys := []string{}
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(keys) == 0 {
		return nil, errors.New("table has no primary key and cannot be edited safely")
	}
	return keys, nil
}

func hasExactKeys(keys map[string]any, primaryKeys []string) bool {
	if len(keys) != len(primaryKeys) {
		return false
	}
	for _, key := range primaryKeys {
		if _, found := keys[key]; !found {
			return false
		}
	}
	return true
}

func rowMutationStatement(engine entity.Engine, schema, table string, mutation dto.RowMutation) (string, []any, error) {
	quotedTable := quoteTable(engine, schema, table)
	valueColumns := sortedColumns(mutation.Values)
	keyColumns := sortedColumns(mutation.Keys)
	arguments := make([]any, 0, len(valueColumns)+len(keyColumns))
	placeholder := func(index int) string {
		if engine == entity.EnginePostgreSQL {
			return fmt.Sprintf("$%d", index)
		}
		return "?"
	}
	if mutation.Kind == "insert" {
		quoted := make([]string, len(valueColumns))
		placeholders := make([]string, len(valueColumns))
		for index, column := range valueColumns {
			quoted[index] = quoteIdentifier(engine, column)
			placeholders[index] = placeholder(index + 1)
			arguments = append(arguments, mutationValue(mutation.Values[column]))
		}
		return "INSERT INTO " + quotedTable + " (" + strings.Join(quoted, ", ") + ") VALUES (" + strings.Join(placeholders, ", ") + ")", arguments, nil
	}
	conditions := make([]string, len(keyColumns))
	for index, column := range keyColumns {
		conditions[index] = quoteIdentifier(engine, column) + " = " + placeholder(len(arguments)+1)
		arguments = append(arguments, mutationValue(mutation.Keys[column]))
	}
	if mutation.Kind == "delete" {
		return "DELETE FROM " + quotedTable + " WHERE " + strings.Join(conditions, " AND "), arguments, nil
	}
	assignments := make([]string, len(valueColumns))
	arguments = arguments[:0]
	for index, column := range valueColumns {
		assignments[index] = quoteIdentifier(engine, column) + " = " + placeholder(index+1)
		arguments = append(arguments, mutationValue(mutation.Values[column]))
	}
	conditions = make([]string, len(keyColumns))
	for index, column := range keyColumns {
		conditions[index] = quoteIdentifier(engine, column) + " = " + placeholder(len(arguments)+1)
		arguments = append(arguments, mutationValue(mutation.Keys[column]))
	}
	return "UPDATE " + quotedTable + " SET " + strings.Join(assignments, ", ") + " WHERE " + strings.Join(conditions, " AND "), arguments, nil
}

func sortedColumns(values map[string]any) []string {
	columns := make([]string, 0, len(values))
	for column := range values {
		columns = append(columns, column)
	}
	sort.Strings(columns)
	return columns
}

func mutationValue(value any) any {
	if value == nil {
		return nil
	}
	switch typed := value.(type) {
	case map[string]any, []any:
		encoded, err := json.Marshal(typed)
		if err == nil {
			return string(encoded)
		}
	}
	return value
}

func (runtime *Runtime) TableDDL(ctx context.Context, connection entity.Connection, password, reference string) (string, error) {
	database, closeTransport, err := runtime.open(connection, password)
	if err != nil {
		return "", err
	}
	defer database.Close()
	defer closeTransport()
	schema, table, err := tableReference(connection, reference)
	if err != nil {
		return "", err
	}
	if connection.Engine != entity.EnginePostgreSQL {
		var name, ddl string
		if err := database.QueryRowContext(ctx, "SHOW CREATE TABLE "+quoteTable(connection.Engine, schema, table)).Scan(&name, &ddl); err != nil {
			return "", err
		}
		return ddl, nil
	}
	columns, err := tableColumnDefinitions(ctx, database, connection, schema, table)
	if err != nil {
		return "", err
	}
	if len(columns) == 0 {
		return "", errors.New("table was not found")
	}
	return "CREATE TABLE " + quoteTable(connection.Engine, schema, table) + " (\n  " + strings.Join(columns, ",\n  ") + "\n);", nil
}

func tableReference(connection entity.Connection, reference string) (string, string, error) {
	parts := strings.Split(reference, ".")
	if len(parts) > 2 || len(parts) == 0 {
		return "", "", errors.New("invalid table identifier")
	}
	schema, table := "", parts[len(parts)-1]
	if len(parts) == 2 {
		schema = parts[0]
	}
	if schema == "" {
		if connection.Engine == entity.EnginePostgreSQL {
			schema = "public"
		} else {
			schema = connection.Database
		}
	}
	if !validIdentifier(schema) || !validIdentifier(table) {
		return "", "", errors.New("invalid table identifier")
	}
	return schema, table, nil
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

func quoteIdentifier(engine entity.Engine, value string) string {
	if engine == entity.EnginePostgreSQL {
		return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
	}
	return "`" + strings.ReplaceAll(value, "`", "``") + "`"
}

func quoteTable(engine entity.Engine, schema, table string) string {
	return quoteIdentifier(engine, schema) + "." + quoteIdentifier(engine, table)
}

func tableColumns(ctx context.Context, database *sql.DB, connection entity.Connection, schema, table string) ([]string, error) {
	query := "SELECT column_name FROM information_schema.columns WHERE table_schema = ? AND table_name = ? ORDER BY ordinal_position"
	if connection.Engine == entity.EnginePostgreSQL {
		query = "SELECT column_name FROM information_schema.columns WHERE table_schema = $1 AND table_name = $2 ORDER BY ordinal_position"
	}
	rows, err := database.QueryContext(ctx, query, schema, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	columns := make([]string, 0)
	for rows.Next() {
		var column string
		if err := rows.Scan(&column); err != nil {
			return nil, err
		}
		columns = append(columns, column)
	}
	return columns, rows.Err()
}

func tableColumnDefinitions(ctx context.Context, database *sql.DB, connection entity.Connection, schema, table string) ([]string, error) {
	query := "SELECT column_name, data_type, is_nullable, column_default FROM information_schema.columns WHERE table_schema = $1 AND table_name = $2 ORDER BY ordinal_position"
	rows, err := database.QueryContext(ctx, query, schema, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	definitions := make([]string, 0)
	for rows.Next() {
		var name, dataType, nullable string
		var defaultValue sql.NullString
		if err := rows.Scan(&name, &dataType, &nullable, &defaultValue); err != nil {
			return nil, err
		}
		definition := quoteIdentifier(connection.Engine, name) + " " + dataType
		if defaultValue.Valid {
			definition += " DEFAULT " + defaultValue.String
		}
		if nullable == "NO" {
			definition += " NOT NULL"
		}
		definitions = append(definitions, definition)
	}
	return definitions, rows.Err()
}

func searchClause(engine entity.Engine, columns []string, search string) (string, []any) {
	search = strings.TrimSpace(search)
	if search == "" {
		return "", []any{}
	}
	conditions := make([]string, 0, len(columns))
	args := make([]any, 0, len(columns))
	for index, column := range columns {
		if engine == entity.EnginePostgreSQL {
			conditions = append(conditions, "CAST("+quoteIdentifier(engine, column)+" AS TEXT) ILIKE $"+fmt.Sprintf("%d", index+1))
		} else {
			conditions = append(conditions, "CAST("+quoteIdentifier(engine, column)+" AS CHAR) LIKE ?")
		}
		args = append(args, "%"+search+"%")
	}
	return " WHERE (" + strings.Join(conditions, " OR ") + ")", args
}

func (runtime *Runtime) Execute(ctx context.Context, connection entity.Connection, password, sqlText string, timeoutSeconds int) (dto.QueryResult, error) {
	database, closeTransport, err := runtime.open(connection, password)
	if err != nil {
		return dto.QueryResult{}, err
	}
	defer database.Close()
	defer closeTransport()
	if timeoutSeconds <= 0 {
		timeoutSeconds = 30
	}
	queryContext, cancel := context.WithTimeout(ctx, time.Duration(timeoutSeconds)*time.Second)
	defer cancel()
	started := time.Now()
	result := dto.QueryResult{Columns: []string{}, Rows: [][]any{}}
	if IsWriteSQL(sqlText) {
		execution, err := database.ExecContext(queryContext, sqlText)
		if err != nil {
			return dto.QueryResult{}, err
		}
		result.RowsAffected, err = execution.RowsAffected()
		if err != nil {
			return dto.QueryResult{}, err
		}
		result.DurationMs = time.Since(started).Milliseconds()
		return result, nil
	}
	rows, err := database.QueryContext(queryContext, sqlText)
	if err != nil {
		return dto.QueryResult{}, err
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		return dto.QueryResult{}, err
	}
	result.Columns = columns
	for rows.Next() {
		if len(result.Rows) >= 1000 {
			break
		}
		values := make([]any, len(columns))
		references := make([]any, len(columns))
		for index := range values {
			references[index] = &values[index]
		}
		if err := rows.Scan(references...); err != nil {
			return dto.QueryResult{}, err
		}
		for index, value := range values {
			values[index] = queryValue(value)
		}
		result.Rows = append(result.Rows, values)
	}
	if err := rows.Err(); err != nil {
		return dto.QueryResult{}, err
	}
	result.DurationMs = time.Since(started).Milliseconds()
	return result, nil
}

func (runtime *Runtime) ExecuteTransaction(ctx context.Context, id, sqlText string, timeoutSeconds int) (dto.QueryResult, error) {
	runtime.transactionMu.Lock()
	handle := runtime.transactions[id]
	runtime.transactionMu.Unlock()
	if handle == nil {
		return dto.QueryResult{}, errors.New("transaction was not found or has expired")
	}
	if timeoutSeconds <= 0 {
		timeoutSeconds = 30
	}
	queryContext, cancel := context.WithTimeout(ctx, time.Duration(timeoutSeconds)*time.Second)
	defer cancel()
	started := time.Now()
	result := dto.QueryResult{Columns: []string{}, Rows: [][]any{}}
	if IsWriteSQL(sqlText) {
		execution, err := handle.transaction.ExecContext(queryContext, sqlText)
		if err != nil {
			return result, err
		}
		result.RowsAffected, err = execution.RowsAffected()
		result.DurationMs = time.Since(started).Milliseconds()
		return result, err
	}
	rows, err := handle.transaction.QueryContext(queryContext, sqlText)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		return result, err
	}
	result.Columns = columns
	for rows.Next() {
		if len(result.Rows) >= 1000 {
			break
		}
		values := make([]any, len(columns))
		refs := make([]any, len(columns))
		for i := range values {
			refs[i] = &values[i]
		}
		if err := rows.Scan(refs...); err != nil {
			return result, err
		}
		for i, v := range values {
			values[i] = queryValue(v)
		}
		result.Rows = append(result.Rows, values)
	}
	result.DurationMs = time.Since(started).Milliseconds()
	return result, rows.Err()
}

func (runtime *Runtime) IsWriteSQL(sqlText string) bool {
	return IsWriteSQL(sqlText)
}

func (runtime *Runtime) TableSchema(ctx context.Context, connection entity.Connection, password, reference string) (dto.TableSchema, error) {
	database, closeTransport, err := runtime.open(connection, password)
	if err != nil {
		return dto.TableSchema{}, err
	}
	defer database.Close()
	defer closeTransport()
	schema, table, err := tableReference(connection, reference)
	if err != nil {
		return dto.TableSchema{}, err
	}
	queryContext, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	columns, err := inspectColumns(queryContext, database, connection, schema, table)
	if err != nil {
		return dto.TableSchema{}, err
	}
	if len(columns) == 0 {
		return dto.TableSchema{}, errors.New("table was not found")
	}
	indexes, err := inspectIndexes(queryContext, database, connection, schema, table)
	if err != nil {
		return dto.TableSchema{}, err
	}
	constraints, err := inspectConstraints(queryContext, database, connection, schema, table)
	if err != nil {
		return dto.TableSchema{}, err
	}
	return dto.TableSchema{Columns: columns, Indexes: indexes, Constraints: constraints}, nil
}

func (runtime *Runtime) CreateTable(ctx context.Context, connection entity.Connection, password, schema string, columns []dto.ColumnDefinition) error {
	database, closeTransport, err := runtime.open(connection, password)
	if err != nil {
		return err
	}
	defer database.Close()
	defer closeTransport()
	resolvedSchema, table, err := tableReference(connection, schema)
	if err != nil {
		return err
	}
	definitions := make([]string, 0, len(columns))
	for _, column := range columns {
		definition, err := columnDDL(connection.Engine, column)
		if err != nil {
			return err
		}
		definitions = append(definitions, definition)
	}
	statement := "CREATE TABLE " + quoteTable(connection.Engine, resolvedSchema, table) + " (" + strings.Join(definitions, ", ") + ")"
	if _, err := database.ExecContext(ctx, statement); err != nil {
		return err
	}
	for _, column := range columns {
		if column.Comment == "" {
			continue
		}
		if err := setColumnComment(ctx, database, connection, resolvedSchema, table, column.Name, column.Comment); err != nil {
			return err
		}
	}
	return nil
}

func (runtime *Runtime) AlterTable(ctx context.Context, connection entity.Connection, password, reference string, actions []dto.TableAlteration) error {
	database, closeTransport, err := runtime.open(connection, password)
	if err != nil {
		return err
	}
	defer database.Close()
	defer closeTransport()
	schema, table, err := tableReference(connection, reference)
	if err != nil {
		return err
	}
	transaction, err := database.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	for _, action := range actions {
		statement, err := alterTableDDL(connection.Engine, schema, table, action)
		if err != nil {
			transaction.Rollback()
			return err
		}
		if _, err := transaction.ExecContext(ctx, statement); err != nil {
			transaction.Rollback()
			return err
		}
		if action.Kind == "rename_table" {
			table = action.NewName
		}
	}
	return transaction.Commit()
}

func (runtime *Runtime) CreateIndex(ctx context.Context, connection entity.Connection, password, reference string, input dto.CreateIndexInput) error {
	database, closeTransport, err := runtime.open(connection, password)
	if err != nil {
		return err
	}
	defer database.Close()
	defer closeTransport()
	schema, table, err := tableReference(connection, reference)
	if err != nil {
		return err
	}
	if !validIdentifier(input.Name) || len(input.Columns) == 0 {
		return errors.New("invalid index definition")
	}
	columns := make([]string, 0, len(input.Columns))
	for _, column := range input.Columns {
		if !validIdentifier(column) {
			return errors.New("invalid index column")
		}
		columns = append(columns, quoteIdentifier(connection.Engine, column))
	}
	unique := ""
	if input.Unique {
		unique = "UNIQUE "
	}
	statement := "CREATE " + unique + "INDEX " + quoteIdentifier(connection.Engine, input.Name) + " ON " + quoteTable(connection.Engine, schema, table) + " (" + strings.Join(columns, ", ") + ")"
	_, err = database.ExecContext(ctx, statement)
	return err
}

func (runtime *Runtime) DropIndex(ctx context.Context, connection entity.Connection, password, reference, index string) error {
	database, closeTransport, err := runtime.open(connection, password)
	if err != nil {
		return err
	}
	defer database.Close()
	defer closeTransport()
	schema, table, err := tableReference(connection, reference)
	if err != nil {
		return err
	}
	if !validIdentifier(index) {
		return errors.New("invalid index name")
	}
	statement := "DROP INDEX " + quoteTable(connection.Engine, schema, index)
	if connection.Engine != entity.EnginePostgreSQL {
		statement = "DROP INDEX " + quoteIdentifier(connection.Engine, index) + " ON " + quoteTable(connection.Engine, schema, table)
	}
	_, err = database.ExecContext(ctx, statement)
	return err
}

func inspectColumns(ctx context.Context, database *sql.DB, connection entity.Connection, schema, table string) ([]dto.TableColumn, error) {
	query := "SELECT column_name, column_type, is_nullable, column_default, column_comment FROM information_schema.columns WHERE table_schema = ? AND table_name = ? ORDER BY ordinal_position"
	if connection.Engine == entity.EnginePostgreSQL {
		query = "SELECT c.column_name, c.data_type, c.is_nullable, c.column_default, COALESCE(d.description, '') FROM information_schema.columns c LEFT JOIN pg_catalog.pg_class cls ON cls.relname = c.table_name LEFT JOIN pg_catalog.pg_namespace ns ON ns.oid = cls.relnamespace AND ns.nspname = c.table_schema LEFT JOIN pg_catalog.pg_attribute a ON a.attrelid = cls.oid AND a.attname = c.column_name LEFT JOIN pg_catalog.pg_description d ON d.objoid = cls.oid AND d.objsubid = a.attnum WHERE c.table_schema = $1 AND c.table_name = $2 ORDER BY c.ordinal_position"
	}
	rows, err := database.QueryContext(ctx, query, schema, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]dto.TableColumn, 0)
	for rows.Next() {
		var column dto.TableColumn
		var nullable string
		var defaultValue sql.NullString
		if err := rows.Scan(&column.Name, &column.DataType, &nullable, &defaultValue, &column.Comment); err != nil {
			return nil, err
		}
		column.Nullable = nullable == "YES"
		if defaultValue.Valid {
			column.DefaultValue = &defaultValue.String
		}
		result = append(result, column)
	}
	return result, rows.Err()
}

func inspectIndexes(ctx context.Context, database *sql.DB, connection entity.Connection, schema, table string) ([]dto.TableIndex, error) {
	if connection.Engine == entity.EnginePostgreSQL {
		rows, err := database.QueryContext(ctx, "SELECT idx.relname, i.indisunique, i.indisprimary, am.amname, pg_get_indexdef(i.indexrelid) FROM pg_catalog.pg_class tbl JOIN pg_catalog.pg_namespace ns ON ns.oid = tbl.relnamespace JOIN pg_catalog.pg_index i ON i.indrelid = tbl.oid JOIN pg_catalog.pg_class idx ON idx.oid = i.indexrelid JOIN pg_catalog.pg_am am ON am.oid = idx.relam WHERE ns.nspname = $1 AND tbl.relname = $2 ORDER BY idx.relname", schema, table)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		result := make([]dto.TableIndex, 0)
		for rows.Next() {
			var item dto.TableIndex
			if err := rows.Scan(&item.Name, &item.Unique, &item.Primary, &item.Type, &item.Definition); err != nil {
				return nil, err
			}
			result = append(result, item)
		}
		return result, rows.Err()
	}
	rows, err := database.QueryContext(ctx, "SELECT index_name, non_unique, index_type, GROUP_CONCAT(column_name ORDER BY seq_in_index SEPARATOR ', ') FROM information_schema.statistics WHERE table_schema = ? AND table_name = ? GROUP BY index_name, non_unique, index_type ORDER BY index_name", schema, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]dto.TableIndex, 0)
	for rows.Next() {
		var item dto.TableIndex
		var nonUnique int
		var columns string
		if err := rows.Scan(&item.Name, &nonUnique, &item.Type, &columns); err != nil {
			return nil, err
		}
		item.Unique = nonUnique == 0
		item.Primary = item.Name == "PRIMARY"
		item.Definition = columns
		result = append(result, item)
	}
	return result, rows.Err()
}

func inspectConstraints(ctx context.Context, database *sql.DB, connection entity.Connection, schema, table string) ([]dto.TableConstraint, error) {
	placeholder := "?"
	if connection.Engine == entity.EnginePostgreSQL {
		placeholder = "$1"
	}
	second := "?"
	if connection.Engine == entity.EnginePostgreSQL {
		second = "$2"
	}
	separator := "GROUP_CONCAT(kcu.column_name ORDER BY kcu.ordinal_position SEPARATOR ',')"
	if connection.Engine == entity.EnginePostgreSQL {
		separator = "string_agg(kcu.column_name, ',' ORDER BY kcu.ordinal_position)"
	}
	query := "SELECT tc.constraint_name, tc.constraint_type, COALESCE(" + separator + ", '') FROM information_schema.table_constraints tc LEFT JOIN information_schema.key_column_usage kcu ON tc.constraint_catalog = kcu.constraint_catalog AND tc.constraint_schema = kcu.constraint_schema AND tc.constraint_name = kcu.constraint_name AND tc.table_name = kcu.table_name WHERE tc.table_schema = " + placeholder + " AND tc.table_name = " + second + " GROUP BY tc.constraint_name, tc.constraint_type ORDER BY tc.constraint_name"
	rows, err := database.QueryContext(ctx, query, schema, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]dto.TableConstraint, 0)
	for rows.Next() {
		var item dto.TableConstraint
		var columns string
		if err := rows.Scan(&item.Name, &item.Type, &columns); err != nil {
			return nil, err
		}
		if columns != "" {
			item.Columns = strings.Split(columns, ",")
		} else {
			item.Columns = []string{}
		}
		item.Definition = item.Type
		result = append(result, item)
	}
	return result, rows.Err()
}

func columnDDL(engine entity.Engine, column dto.ColumnDefinition) (string, error) {
	if !validIdentifier(column.Name) || !validDataType(column.DataType) || !validDefault(column.DefaultValue) {
		return "", errors.New("invalid column definition")
	}
	definition := quoteIdentifier(engine, column.Name) + " " + strings.TrimSpace(column.DataType)
	if column.DefaultValue != nil && strings.TrimSpace(*column.DefaultValue) != "" {
		definition += " DEFAULT " + strings.TrimSpace(*column.DefaultValue)
	}
	if !column.Nullable {
		definition += " NOT NULL"
	}
	return definition, nil
}

func alterTableDDL(engine entity.Engine, schema, table string, action dto.TableAlteration) (string, error) {
	quotedTable := quoteTable(engine, schema, table)
	if !validIdentifier(action.Column) && action.Kind != "add_column" && action.Kind != "rename_table" {
		return "", errors.New("invalid column name")
	}
	switch action.Kind {
	case "add_column":
		if action.Definition == nil {
			return "", errors.New("column definition is required")
		}
		definition, err := columnDDL(engine, *action.Definition)
		if err != nil {
			return "", err
		}
		return "ALTER TABLE " + quotedTable + " ADD COLUMN " + definition, nil
	case "drop_column":
		return "ALTER TABLE " + quotedTable + " DROP COLUMN " + quoteIdentifier(engine, action.Column), nil
	case "rename_table":
		if !validIdentifier(action.NewName) {
			return "", errors.New("invalid table name")
		}
		return "ALTER TABLE " + quotedTable + " RENAME TO " + quoteIdentifier(engine, action.NewName), nil
	case "rename_column":
		if !validIdentifier(action.NewName) {
			return "", errors.New("invalid column name")
		}
		return "ALTER TABLE " + quotedTable + " RENAME COLUMN " + quoteIdentifier(engine, action.Column) + " TO " + quoteIdentifier(engine, action.NewName), nil
	case "set_type":
		if action.Definition == nil || !validDataType(action.Definition.DataType) {
			return "", errors.New("valid data type is required")
		}
		if engine == entity.EnginePostgreSQL {
			return "ALTER TABLE " + quotedTable + " ALTER COLUMN " + quoteIdentifier(engine, action.Column) + " TYPE " + strings.TrimSpace(action.Definition.DataType), nil
		}
		return "ALTER TABLE " + quotedTable + " MODIFY COLUMN " + quoteIdentifier(engine, action.Column) + " " + strings.TrimSpace(action.Definition.DataType), nil
	case "set_nullable":
		if action.Definition == nil {
			return "", errors.New("column definition is required")
		}
		if engine == entity.EnginePostgreSQL {
			if action.Definition.Nullable {
				return "ALTER TABLE " + quotedTable + " ALTER COLUMN " + quoteIdentifier(engine, action.Column) + " DROP NOT NULL", nil
			}
			return "ALTER TABLE " + quotedTable + " ALTER COLUMN " + quoteIdentifier(engine, action.Column) + " SET NOT NULL", nil
		}
		nullable := "NOT NULL"
		if action.Definition.Nullable {
			nullable = "NULL"
		}
		return "ALTER TABLE " + quotedTable + " MODIFY COLUMN " + quoteIdentifier(engine, action.Column) + " " + strings.TrimSpace(action.Definition.DataType) + " " + nullable, nil
	case "set_default":
		if action.Definition == nil || !validDefault(action.Definition.DefaultValue) {
			return "", errors.New("valid default value is required")
		}
		if action.Definition.DefaultValue == nil || strings.TrimSpace(*action.Definition.DefaultValue) == "" {
			return "ALTER TABLE " + quotedTable + " ALTER COLUMN " + quoteIdentifier(engine, action.Column) + " DROP DEFAULT", nil
		}
		if engine == entity.EnginePostgreSQL {
			return "ALTER TABLE " + quotedTable + " ALTER COLUMN " + quoteIdentifier(engine, action.Column) + " SET DEFAULT " + strings.TrimSpace(*action.Definition.DefaultValue), nil
		}
		return "ALTER TABLE " + quotedTable + " ALTER COLUMN " + quoteIdentifier(engine, action.Column) + " SET DEFAULT " + strings.TrimSpace(*action.Definition.DefaultValue), nil
	default:
		return "", errors.New("unsupported table alteration")
	}
}

func validDataType(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 128 || strings.ContainsAny(value, "';`\\\"/-") {
		return false
	}
	for _, character := range value {
		if !(character == ' ' || character == ',' || character == '(' || character == ')' || character == '_' || character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9') {
			return false
		}
	}
	return true
}

func validDefault(value *string) bool {
	if value == nil {
		return true
	}
	trimmed := strings.TrimSpace(*value)
	return len(trimmed) <= 500 && !strings.Contains(trimmed, ";") && !strings.Contains(trimmed, "--") && !strings.Contains(trimmed, "/*")
}

func setColumnComment(ctx context.Context, database *sql.DB, connection entity.Connection, schema, table, column, comment string) error {
	if len(comment) > 500 {
		return errors.New("column comment is too long")
	}
	if connection.Engine == entity.EnginePostgreSQL {
		_, err := database.ExecContext(ctx, "COMMENT ON COLUMN "+quoteTable(connection.Engine, schema, table)+"."+quoteIdentifier(connection.Engine, column)+" IS $1", comment)
		return err
	}
	return nil
}

func queryValue(value any) any {
	switch converted := value.(type) {
	case []byte:
		return string(converted)
	case time.Time:
		return converted.UTC().Format(time.RFC3339Nano)
	default:
		return value
	}
}

func (runtime *Runtime) Dashboard(ctx context.Context, connection entity.Connection, password string) (dto.DatabaseDashboard, error) {
	database, closeTransport, err := runtime.open(connection, password)
	if err != nil {
		return dto.DatabaseDashboard{}, err
	}
	defer database.Close()
	defer closeTransport()
	queryContext, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	result := dto.DatabaseDashboard{Available: true, Engine: string(connection.Engine), Metrics: []dto.Metric{}}
	if connection.Engine == entity.EnginePostgreSQL {
		var version string
		var connections, tables int64
		var size int64
		err = database.QueryRowContext(queryContext, "SELECT current_setting('server_version'), (SELECT COUNT(*) FROM pg_stat_activity), pg_database_size(current_database()), (SELECT COUNT(*) FROM information_schema.tables WHERE table_type = 'BASE TABLE' AND table_schema NOT IN ('pg_catalog', 'information_schema'))").Scan(&version, &connections, &size, &tables)
		if err == nil {
			result.Version = version
			result.Metrics = []dto.Metric{{Key: "connections", Label: "Connections", Value: connections}, {Key: "storage", Label: "Storage", Value: size, Unit: "bytes"}, {Key: "tables", Label: "Tables", Value: tables}}
		}
	} else {
		var version string
		err = database.QueryRowContext(queryContext, "SELECT VERSION()").Scan(&version)
		if err == nil {
			result.Version = version
			var tables, size int64
			err = database.QueryRowContext(queryContext, "SELECT COUNT(*), COALESCE(SUM(data_length + index_length), 0) FROM information_schema.tables WHERE table_schema = DATABASE()").Scan(&tables, &size)
			if err == nil {
				result.Metrics = []dto.Metric{{Key: "storage", Label: "Storage", Value: size, Unit: "bytes"}, {Key: "tables", Label: "Tables", Value: tables}}
			}
		}
	}
	if err != nil {
		return dto.DatabaseDashboard{Available: false, Engine: string(connection.Engine), Message: "Dashboard metrics are unavailable for this database user", Metrics: []dto.Metric{}}, nil
	}
	return result, nil
}

func (runtime *Runtime) Sessions(ctx context.Context, connection entity.Connection, password string) (dto.DatabaseSessions, error) {
	database, closeTransport, err := runtime.open(connection, password)
	if err != nil {
		return dto.DatabaseSessions{}, err
	}
	defer database.Close()
	defer closeTransport()
	queryContext, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	result := dto.DatabaseSessions{Available: true, Items: []dto.DatabaseSession{}}
	if connection.Engine == entity.EnginePostgreSQL {
		rows, queryErr := database.QueryContext(queryContext, "SELECT pid::text, usename, datname, state, COALESCE(query, ''), COALESCE(EXTRACT(EPOCH FROM (clock_timestamp() - query_start)) * 1000, 0)::bigint, COALESCE(to_char(query_start AT TIME ZONE 'UTC', 'YYYY-MM-DD\"T\"HH24:MI:SS\"Z\"'), ''), COALESCE(client_addr::text, ''), COALESCE(wait_event_type || ':' || wait_event, '') FROM pg_stat_activity WHERE pid <> pg_backend_pid() ORDER BY query_start DESC NULLS LAST LIMIT 200")
		if queryErr != nil {
			return unavailableSessions(), nil
		}
		defer rows.Close()
		for rows.Next() {
			var item dto.DatabaseSession
			if err := rows.Scan(&item.ID, &item.User, &item.Database, &item.State, &item.Query, &item.DurationMs, &item.StartedAt, &item.Client, &item.WaitEvent); err != nil {
				return unavailableSessions(), nil
			}
			item.Query = truncateText(item.Query, 2000)
			result.Items = append(result.Items, item)
		}
		if rows.Err() != nil {
			return unavailableSessions(), nil
		}
		return result, nil
	}
	rows, queryErr := database.QueryContext(queryContext, "SELECT ID, USER, COALESCE(DB, ''), COMMAND, COALESCE(INFO, ''), TIME, COALESCE(HOST, '') FROM information_schema.PROCESSLIST ORDER BY TIME DESC LIMIT 200")
	if queryErr != nil {
		return unavailableSessions(), nil
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var item dto.DatabaseSession
		var seconds int64
		if err := rows.Scan(&id, &item.User, &item.Database, &item.State, &item.Query, &seconds, &item.Client); err != nil {
			return unavailableSessions(), nil
		}
		item.ID = fmt.Sprint(id)
		item.DurationMs = seconds * 1000
		item.Query = truncateText(item.Query, 2000)
		result.Items = append(result.Items, item)
	}
	if rows.Err() != nil {
		return unavailableSessions(), nil
	}
	return result, nil
}

func (runtime *Runtime) Locks(ctx context.Context, connection entity.Connection, password string) (dto.DatabaseLocks, error) {
	database, closeTransport, err := runtime.open(connection, password)
	if err != nil {
		return dto.DatabaseLocks{}, err
	}
	defer database.Close()
	defer closeTransport()
	queryContext, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	result := dto.DatabaseLocks{Available: true, Items: []dto.DatabaseLock{}}
	if connection.Engine == entity.EnginePostgreSQL {
		rows, queryErr := database.QueryContext(queryContext, "SELECT l.pid::text, l.locktype, COALESCE(c.relname, ''), l.mode, l.granted, COALESCE(a.query, '') FROM pg_locks l LEFT JOIN pg_stat_activity a ON a.pid = l.pid LEFT JOIN pg_class c ON c.oid = l.relation WHERE l.pid <> pg_backend_pid() ORDER BY l.granted, l.pid LIMIT 300")
		if queryErr != nil {
			return unavailableLocks(), nil
		}
		defer rows.Close()
		for rows.Next() {
			var item dto.DatabaseLock
			if err := rows.Scan(&item.ID, &item.Type, &item.Object, &item.Mode, &item.Granted, &item.Query); err != nil {
				return unavailableLocks(), nil
			}
			item.Query = truncateText(item.Query, 1000)
			result.Items = append(result.Items, item)
		}
		if rows.Err() != nil {
			return unavailableLocks(), nil
		}
		return result, nil
	}
	rows, queryErr := database.QueryContext(queryContext, "SELECT CONCAT(w.REQUESTING_ENGINE_LOCK_ID, ':', w.REQUESTING_THREAD_ID), COALESCE(r.OBJECT_TYPE, ''), CONCAT(COALESCE(r.OBJECT_SCHEMA, ''), '.', COALESCE(r.OBJECT_NAME, '')), COALESCE(r.LOCK_MODE, ''), w.REQUESTING_THREAD_ID, w.BLOCKING_THREAD_ID FROM performance_schema.data_lock_waits w LEFT JOIN performance_schema.data_locks r ON r.ENGINE_LOCK_ID = w.REQUESTING_ENGINE_LOCK_ID AND r.ENGINE = w.ENGINE LIMIT 300")
	if queryErr != nil {
		return unavailableLocks(), nil
	}
	defer rows.Close()
	for rows.Next() {
		var item dto.DatabaseLock
		if err := rows.Scan(&item.ID, &item.Type, &item.Object, &item.Mode, &item.WaitingPID, &item.BlockingPID); err != nil {
			return unavailableLocks(), nil
		}
		item.Granted = false
		result.Items = append(result.Items, item)
	}
	if rows.Err() != nil {
		return unavailableLocks(), nil
	}
	return result, nil
}

func (runtime *Runtime) Performance(ctx context.Context, connection entity.Connection, password string) (dto.DatabasePerformance, error) {
	database, closeTransport, err := runtime.open(connection, password)
	if err != nil {
		return dto.DatabasePerformance{}, err
	}
	defer database.Close()
	defer closeTransport()
	queryContext, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	result := dto.DatabasePerformance{Available: true, SlowQueries: []dto.SlowQuery{}}
	if connection.Engine == entity.EnginePostgreSQL {
		var enabled bool
		if err := database.QueryRowContext(queryContext, "SELECT EXISTS(SELECT 1 FROM pg_extension WHERE extname = 'pg_stat_statements')").Scan(&enabled); err != nil || !enabled {
			return unavailablePerformance("pg_stat_statements is not enabled"), nil
		}
		rows, queryErr := database.QueryContext(queryContext, "SELECT query, calls, total_exec_time, mean_exec_time, rows FROM pg_stat_statements ORDER BY total_exec_time DESC LIMIT 50")
		if queryErr != nil {
			return unavailablePerformance("Query statistics are unavailable for this database user"), nil
		}
		defer rows.Close()
		for rows.Next() {
			var item dto.SlowQuery
			if err := rows.Scan(&item.Query, &item.Calls, &item.TotalMs, &item.MeanMs, &item.Rows); err != nil {
				return unavailablePerformance("Query statistics are unavailable for this database user"), nil
			}
			item.Query = truncateText(item.Query, 2000)
			result.SlowQueries = append(result.SlowQueries, item)
		}
		if rows.Err() != nil {
			return unavailablePerformance("Query statistics are unavailable for this database user"), nil
		}
		return result, nil
	}
	rows, queryErr := database.QueryContext(queryContext, "SELECT DIGEST_TEXT, COUNT_STAR, SUM_TIMER_WAIT / 1000000000, AVG_TIMER_WAIT / 1000000000, SUM_ROWS_SENT FROM performance_schema.events_statements_summary_by_digest WHERE SCHEMA_NAME = DATABASE() ORDER BY SUM_TIMER_WAIT DESC LIMIT 50")
	if queryErr != nil {
		return unavailablePerformance("Performance Schema statement summaries are unavailable"), nil
	}
	defer rows.Close()
	for rows.Next() {
		var item dto.SlowQuery
		if err := rows.Scan(&item.Query, &item.Calls, &item.TotalMs, &item.MeanMs, &item.Rows); err != nil {
			return unavailablePerformance("Performance Schema statement summaries are unavailable"), nil
		}
		item.Query = truncateText(item.Query, 2000)
		result.SlowQueries = append(result.SlowQueries, item)
	}
	if rows.Err() != nil {
		return unavailablePerformance("Performance Schema statement summaries are unavailable"), nil
	}
	return result, nil
}

func unavailableSessions() dto.DatabaseSessions {
	return dto.DatabaseSessions{Available: false, Message: "Session inspection is unavailable for this database user", Items: []dto.DatabaseSession{}}
}
func unavailableLocks() dto.DatabaseLocks {
	return dto.DatabaseLocks{Available: false, Message: "Lock inspection is unavailable for this engine or database user", Items: []dto.DatabaseLock{}}
}
func unavailablePerformance(message string) dto.DatabasePerformance {
	return dto.DatabasePerformance{Available: false, Message: message, SlowQueries: []dto.SlowQuery{}}
}
func truncateText(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit]
}

func max(first, second int) int {
	if first > second {
		return first
	}
	return second
}

func IsWriteSQL(sqlText string) bool {
	for _, token := range sqlTokens(sqlText) {
		switch token {
		case "insert", "update", "delete", "create", "alter", "drop", "truncate", "grant", "revoke", "replace", "merge", "call", "do", "set", "load", "copy":
			return true
		}
	}
	return false
}

func sqlTokens(sqlText string) []string {
	tokens := make([]string, 0)
	var token strings.Builder
	flush := func() {
		if token.Len() > 0 {
			tokens = append(tokens, strings.ToLower(token.String()))
			token.Reset()
		}
	}
	for index := 0; index < len(sqlText); index++ {
		character := sqlText[index]
		if character == '-' && index+1 < len(sqlText) && sqlText[index+1] == '-' {
			flush()
			for index < len(sqlText) && sqlText[index] != '\n' {
				index++
			}
			continue
		}
		if character == '/' && index+1 < len(sqlText) && sqlText[index+1] == '*' {
			flush()
			index += 2
			for index+1 < len(sqlText) && (sqlText[index] != '*' || sqlText[index+1] != '/') {
				index++
			}
			index++
			continue
		}
		if character == '\'' || character == '"' || character == '`' {
			flush()
			quote := character
			for index++; index < len(sqlText); index++ {
				if sqlText[index] == quote {
					if index+1 < len(sqlText) && sqlText[index+1] == quote {
						index++
						continue
					}
					break
				}
			}
			continue
		}
		if (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') || character == '_' {
			token.WriteByte(character)
			continue
		}
		flush()
	}
	flush()
	return tokens
}
