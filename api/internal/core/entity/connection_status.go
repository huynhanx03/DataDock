package entity

import "time"

type ConnectionState string

const (
	ConnectionStateDisconnected ConnectionState = "disconnected"
	ConnectionStateConnecting   ConnectionState = "connecting"
	ConnectionStateConnected    ConnectionState = "connected"
	ConnectionStateError        ConnectionState = "error"
)

type ConnectionRuntimeStatus struct {
	State              ConnectionState `json:"status,omitempty"`
	LatencyMS          int64           `json:"latencyMs,omitempty"`
	LastConnectedAt    *time.Time      `json:"lastConnectedAt,omitempty"`
	LastErrorCode      string          `json:"lastErrorCode,omitempty"`
	ActiveTransactions int             `json:"activeTransactions,omitempty"`
}
