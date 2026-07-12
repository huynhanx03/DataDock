package entity

import "time"

type Engine string

const (
	EnginePostgreSQL Engine = "postgresql"
	EngineMySQL      Engine = "mysql"
	EngineMariaDB    Engine = "mariadb"
	EngineSQLite     Engine = "sqlite"
	EngineSQLServer  Engine = "sqlserver"
	EngineOracle     Engine = "oracle"
	EngineClickHouse Engine = "clickhouse"
	EngineRedis      Engine = "redis"
	EngineMongoDB    Engine = "mongodb"
)

func (engine Engine) Valid() bool {
	return engine == EnginePostgreSQL || engine == EngineMySQL || engine == EngineMariaDB
}

type SSLMode string

const (
	SSLModeDisable    SSLMode = "disable"
	SSLModeRequire    SSLMode = "require"
	SSLModeVerifyCA   SSLMode = "verify-ca"
	SSLModeVerifyFull SSLMode = "verify-full"
)

type Connection struct {
	ID              string    `json:"id"`
	WorkspaceID     string    `json:"workspaceId"`
	Name            string    `json:"name"`
	Engine          Engine    `json:"engine"`
	Host            string    `json:"host"`
	Port            int       `json:"port"`
	Database        string    `json:"database"`
	Username        string    `json:"username"`
	PasswordCipher  string    `json:"-"`
	SSLMode         SSLMode   `json:"sslMode"`
	SSLCAPath       string    `json:"sslCaPath"`
	SSLCertPath     string    `json:"sslCertPath"`
	SSLKeyPath      string    `json:"sslKeyPath"`
	ProxyURL        string    `json:"proxyUrl"`
	SSHTunnel       SSHTunnel `json:"sshTunnel"`
	ReadOnly        bool      `json:"readOnly"`
	AutoReconnect   bool      `json:"autoReconnect"`
	MaxOpenConns    int       `json:"maxOpenConns"`
	MaxIdleConns    int       `json:"maxIdleConns"`
	ConnMaxLifetime int       `json:"connMaxLifetimeSeconds"`
	Favorite        bool      `json:"favorite"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

type SSHTunnel struct {
	Enabled        bool   `json:"enabled"`
	Host           string `json:"host"`
	Port           int    `json:"port"`
	Username       string `json:"username"`
	PasswordCipher string `json:"-"`
	Password       string `json:"-"`
	PrivateKeyPath string `json:"privateKeyPath"`
	KnownHostsPath string `json:"knownHostsPath"`
}

type EngineCapability struct {
	Engine       Engine   `json:"engine"`
	Label        string   `json:"label"`
	Available    bool     `json:"available"`
	Capabilities []string `json:"capabilities"`
}

func EngineCapabilities() []EngineCapability {
	return []EngineCapability{
		{Engine: EnginePostgreSQL, Label: "PostgreSQL", Available: true, Capabilities: []string{"connection", "explorer", "table-browser", "sql-editor", "schema", "operations"}},
		{Engine: EngineMySQL, Label: "MySQL", Available: true, Capabilities: []string{"connection", "explorer", "table-browser", "sql-editor", "schema", "operations"}},
		{Engine: EngineMariaDB, Label: "MariaDB", Available: true, Capabilities: []string{"connection", "explorer", "table-browser", "sql-editor", "schema", "operations"}},
		{Engine: EngineSQLite, Label: "SQLite", Available: false, Capabilities: []string{}},
		{Engine: EngineSQLServer, Label: "SQL Server", Available: false, Capabilities: []string{}},
		{Engine: EngineOracle, Label: "Oracle", Available: false, Capabilities: []string{}},
		{Engine: EngineClickHouse, Label: "ClickHouse", Available: false, Capabilities: []string{}},
		{Engine: EngineRedis, Label: "Redis", Available: false, Capabilities: []string{}},
		{Engine: EngineMongoDB, Label: "MongoDB", Available: false, Capabilities: []string{}},
	}
}

func (connection Connection) DefaultPort() int {
	if connection.Engine == EnginePostgreSQL {
		return 5432
	}
	return 3306
}
