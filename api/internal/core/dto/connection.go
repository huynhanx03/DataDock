package dto

import "github.com/huynhanx03/datadock/internal/core/entity"

type ConnectionInput struct {
	WorkspaceID     string         `json:"workspaceId" binding:"required"`
	Name            string         `json:"name" binding:"required,max=100"`
	Engine          entity.Engine  `json:"engine" binding:"required"`
	Host            string         `json:"host" binding:"required,max=255"`
	Port            int            `json:"port" binding:"gte=0,lte=65535"`
	Database        string         `json:"database" binding:"max=255"`
	Username        string         `json:"username" binding:"max=255"`
	Password        string         `json:"password"`
	ClearPassword   bool           `json:"clearPassword"`
	SSLMode         entity.SSLMode `json:"sslMode"`
	SSLCAPath       string         `json:"sslCaPath" binding:"max=4096"`
	SSLCertPath     string         `json:"sslCertPath" binding:"max=4096"`
	SSLKeyPath      string         `json:"sslKeyPath" binding:"max=4096"`
	ProxyURL        string         `json:"proxyUrl" binding:"max=2048"`
	ClearProxyAuth  bool           `json:"clearProxyCredentials"`
	SSHTunnel       SSHTunnelInput `json:"sshTunnel"`
	ReadOnly        bool           `json:"readOnly"`
	AutoReconnect   bool           `json:"autoReconnect"`
	MaxOpenConns    int            `json:"maxOpenConns" binding:"gte=0,lte=100"`
	MaxIdleConns    int            `json:"maxIdleConns" binding:"gte=0,lte=100"`
	ConnMaxLifetime int            `json:"connMaxLifetimeSeconds" binding:"gte=0,lte=86400"`
	ConnMaxIdleTime int            `json:"connMaxIdleTimeSeconds" binding:"gte=0,lte=86400"`
}

type SSHTunnelInput struct {
	Enabled        bool   `json:"enabled"`
	Host           string `json:"host" binding:"max=255"`
	Port           int    `json:"port" binding:"gte=0,lte=65535"`
	Username       string `json:"username" binding:"max=255"`
	Password       string `json:"password"`
	ClearPassword  bool   `json:"clearPassword"`
	PrivateKeyPath string `json:"privateKeyPath" binding:"max=4096"`
	KnownHostsPath string `json:"knownHostsPath" binding:"max=4096"`
}

type ConnectionView struct {
	entity.ConnectionRuntimeStatus
	ID              string         `json:"id"`
	WorkspaceID     string         `json:"workspaceId"`
	Name            string         `json:"name"`
	Engine          entity.Engine  `json:"engine"`
	Host            string         `json:"host"`
	Port            int            `json:"port"`
	Database        string         `json:"database"`
	Username        string         `json:"username"`
	SSLMode         entity.SSLMode `json:"sslMode"`
	SSLCAPath       string         `json:"sslCaPath"`
	SSLCertPath     string         `json:"sslCertPath"`
	SSLKeyPath      string         `json:"sslKeyPath"`
	ProxyURL        string         `json:"proxyUrl"`
	SSHTunnel       SSHTunnelView  `json:"sshTunnel"`
	ReadOnly        bool           `json:"readOnly"`
	AutoReconnect   bool           `json:"autoReconnect"`
	MaxOpenConns    int            `json:"maxOpenConns"`
	MaxIdleConns    int            `json:"maxIdleConns"`
	ConnMaxLifetime int            `json:"connMaxLifetimeSeconds"`
	ConnMaxIdleTime int            `json:"connMaxIdleTimeSeconds"`
	Favorite        bool           `json:"favorite"`
	HasPassword     bool           `json:"hasPassword"`
	HasProxyAuth    bool           `json:"hasProxyCredentials"`
	CreatedAt       string         `json:"createdAt"`
	UpdatedAt       string         `json:"updatedAt"`
}

type ConnectionPatchInput struct {
	WorkspaceID     *string              `json:"workspaceId"`
	Name            *string              `json:"name"`
	Engine          *entity.Engine       `json:"engine"`
	Host            *string              `json:"host"`
	Port            *int                 `json:"port"`
	Database        *string              `json:"database"`
	Username        *string              `json:"username"`
	Password        *string              `json:"password"`
	ClearPassword   *bool                `json:"clearPassword"`
	SSLMode         *entity.SSLMode      `json:"sslMode"`
	SSLCAPath       *string              `json:"sslCaPath"`
	SSLCertPath     *string              `json:"sslCertPath"`
	SSLKeyPath      *string              `json:"sslKeyPath"`
	ProxyURL        *string              `json:"proxyUrl"`
	ClearProxyAuth  *bool                `json:"clearProxyCredentials"`
	SSHTunnel       *SSHTunnelPatchInput `json:"sshTunnel"`
	ReadOnly        *bool                `json:"readOnly"`
	AutoReconnect   *bool                `json:"autoReconnect"`
	MaxOpenConns    *int                 `json:"maxOpenConns"`
	MaxIdleConns    *int                 `json:"maxIdleConns"`
	ConnMaxLifetime *int                 `json:"connMaxLifetimeSeconds"`
	ConnMaxIdleTime *int                 `json:"connMaxIdleTimeSeconds"`
}

type SSHTunnelPatchInput struct {
	Enabled        *bool   `json:"enabled"`
	Host           *string `json:"host"`
	Port           *int    `json:"port"`
	Username       *string `json:"username"`
	Password       *string `json:"password"`
	ClearPassword  *bool   `json:"clearPassword"`
	PrivateKeyPath *string `json:"privateKeyPath"`
	KnownHostsPath *string `json:"knownHostsPath"`
}

type ConnectionTestResult struct {
	OK        bool   `json:"ok"`
	LatencyMS int64  `json:"latencyMs"`
	Message   string `json:"message"`
}

type ConnectionFavoriteInput struct {
	Favorite *bool `json:"favorite"`
}

type SSHTunnelView struct {
	Enabled        bool   `json:"enabled"`
	Host           string `json:"host"`
	Port           int    `json:"port"`
	Username       string `json:"username"`
	PrivateKeyPath string `json:"privateKeyPath"`
	KnownHostsPath string `json:"knownHostsPath"`
	HasPassword    bool   `json:"hasPassword"`
}
