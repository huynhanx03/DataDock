package dto

import (
	"time"

	"github.com/huynhanx03/datadock/internal/core/entity"
)

type OperationsWindow string

const (
	OperationsWindow5Minutes  OperationsWindow = "5m"
	OperationsWindow15Minutes OperationsWindow = "15m"
	OperationsWindow1Hour     OperationsWindow = "1h"
	OperationsWindow6Hours    OperationsWindow = "6h"
	OperationsWindow24Hours   OperationsWindow = "24h"
)

type OperationsSessionState string

const (
	OperationsSessionStateAll               OperationsSessionState = "all"
	OperationsSessionStateActive            OperationsSessionState = "active"
	OperationsSessionStateIdle              OperationsSessionState = "idle"
	OperationsSessionStateIdleInTransaction OperationsSessionState = "idle_in_transaction"
)

type OperationsSessionAction string

const (
	OperationsSessionCancel    OperationsSessionAction = "cancel"
	OperationsSessionTerminate OperationsSessionAction = "terminate"
)

type OperationsRangeInput struct {
	Window OperationsWindow `json:"window"`
	Points int              `json:"points"`
}

type OperationsSessionsInput struct {
	Limit int                    `json:"limit"`
	State OperationsSessionState `json:"state"`
}

type OperationsLocksInput struct {
	Limit int `json:"limit"`
}

type OperationsPerformanceInput struct {
	Range OperationsRangeInput `json:"range"`
	Limit int                  `json:"limit"`
}

type OperationsSessionControlInput struct {
	SessionID string                  `json:"sessionId"`
	Action    OperationsSessionAction `json:"action"`
}

type OperationsMetricPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
}

type OperationsMetric struct {
	Key    string                  `json:"key"`
	Label  string                  `json:"label"`
	Value  float64                 `json:"value"`
	Unit   string                  `json:"unit,omitempty"`
	Series []OperationsMetricPoint `json:"series,omitempty"`
}

type OperationsDashboard struct {
	Available    bool               `json:"available"`
	Message      string             `json:"message,omitempty"`
	ConnectionID string             `json:"connectionId"`
	Engine       entity.Engine      `json:"engine"`
	Version      string             `json:"version,omitempty"`
	Window       OperationsWindow   `json:"window"`
	CollectedAt  time.Time          `json:"collectedAt"`
	Metrics      []OperationsMetric `json:"metrics"`
}

type OperationsSession struct {
	ID         string     `json:"id"`
	User       string     `json:"user"`
	Database   string     `json:"database"`
	State      string     `json:"state"`
	Query      string     `json:"query,omitempty"`
	DurationMS int64      `json:"durationMs,omitempty"`
	StartedAt  *time.Time `json:"startedAt,omitempty"`
	Client     string     `json:"client,omitempty"`
	WaitEvent  string     `json:"waitEvent,omitempty"`
}

type OperationsSessions struct {
	Available    bool                `json:"available"`
	Message      string              `json:"message,omitempty"`
	ConnectionID string              `json:"connectionId"`
	Engine       entity.Engine       `json:"engine"`
	CollectedAt  time.Time           `json:"collectedAt"`
	Items        []OperationsSession `json:"items"`
}

type OperationsLock struct {
	ID                string `json:"id"`
	Type              string `json:"type"`
	Object            string `json:"object,omitempty"`
	Mode              string `json:"mode,omitempty"`
	Granted           bool   `json:"granted"`
	WaitingSessionID  string `json:"waitingSessionId,omitempty"`
	BlockingSessionID string `json:"blockingSessionId,omitempty"`
	Query             string `json:"query,omitempty"`
}

type OperationsBlockingChain struct {
	WaitingSessionID   string   `json:"waitingSessionId"`
	BlockingSessionIDs []string `json:"blockingSessionIds"`
}

type OperationsLocks struct {
	Available      bool                      `json:"available"`
	Message        string                    `json:"message,omitempty"`
	ConnectionID   string                    `json:"connectionId"`
	Engine         entity.Engine             `json:"engine"`
	CollectedAt    time.Time                 `json:"collectedAt"`
	Items          []OperationsLock          `json:"items"`
	BlockingChains []OperationsBlockingChain `json:"blockingChains"`
}

type OperationsSlowQuery struct {
	Fingerprint string  `json:"fingerprint"`
	Query       string  `json:"query"`
	Calls       int64   `json:"calls"`
	TotalMS     float64 `json:"totalMs"`
	MeanMS      float64 `json:"meanMs"`
	Rows        int64   `json:"rows"`
}

type OperationsPerformance struct {
	Available    bool                  `json:"available"`
	Message      string                `json:"message,omitempty"`
	ConnectionID string                `json:"connectionId"`
	Engine       entity.Engine         `json:"engine"`
	Window       OperationsWindow      `json:"window"`
	CollectedAt  time.Time             `json:"collectedAt"`
	Metrics      []OperationsMetric    `json:"metrics"`
	SlowQueries  []OperationsSlowQuery `json:"slowQueries"`
}

type OperationsSessionControlResult struct {
	SessionID  string                  `json:"sessionId"`
	Action     OperationsSessionAction `json:"action"`
	AcceptedAt time.Time               `json:"acceptedAt"`
}
