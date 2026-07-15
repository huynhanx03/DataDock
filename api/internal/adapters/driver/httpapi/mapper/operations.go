package mapper

import "github.com/huynhanx03/datadock/internal/core/dto"

type OperationsRangeQuery struct {
	Window dto.OperationsWindow `form:"window"`
	Points int                  `form:"points"`
}

type OperationsSessionsQuery struct {
	Limit int                        `form:"limit"`
	State dto.OperationsSessionState `form:"state"`
}

type OperationsLocksQuery struct {
	Limit int `form:"limit"`
}

type OperationsPerformanceQuery struct {
	Window dto.OperationsWindow `form:"window"`
	Points int                  `form:"points"`
	Limit  int                  `form:"limit"`
}

type OperationsSessionControlRequest struct {
	Action dto.OperationsSessionAction `json:"action"`
}

type OperationsDashboardResponse dto.OperationsDashboard
type OperationsSessionsResponse dto.OperationsSessions
type OperationsLocksResponse dto.OperationsLocks
type OperationsPerformanceResponse dto.OperationsPerformance
type OperationsSessionControlResponse dto.OperationsSessionControlResult

func ToOperationsRangeInput(query OperationsRangeQuery) dto.OperationsRangeInput {
	return dto.OperationsRangeInput{Window: query.Window, Points: query.Points}
}

func ToOperationsSessionsInput(query OperationsSessionsQuery) dto.OperationsSessionsInput {
	return dto.OperationsSessionsInput{Limit: query.Limit, State: query.State}
}

func ToOperationsLocksInput(query OperationsLocksQuery) dto.OperationsLocksInput {
	return dto.OperationsLocksInput{Limit: query.Limit}
}

func ToOperationsPerformanceInput(query OperationsPerformanceQuery) dto.OperationsPerformanceInput {
	return dto.OperationsPerformanceInput{
		Range: dto.OperationsRangeInput{Window: query.Window, Points: query.Points},
		Limit: query.Limit,
	}
}

func ToOperationsSessionControlInput(sessionID string, request OperationsSessionControlRequest) dto.OperationsSessionControlInput {
	return dto.OperationsSessionControlInput{SessionID: sessionID, Action: request.Action}
}

func ToOperationsDashboardResponse(result dto.OperationsDashboard) OperationsDashboardResponse {
	return OperationsDashboardResponse(result)
}

func ToOperationsSessionsResponse(result dto.OperationsSessions) OperationsSessionsResponse {
	return OperationsSessionsResponse(result)
}

func ToOperationsLocksResponse(result dto.OperationsLocks) OperationsLocksResponse {
	return OperationsLocksResponse(result)
}

func ToOperationsPerformanceResponse(result dto.OperationsPerformance) OperationsPerformanceResponse {
	return OperationsPerformanceResponse(result)
}

func ToOperationsSessionControlResponse(result dto.OperationsSessionControlResult) OperationsSessionControlResponse {
	return OperationsSessionControlResponse(result)
}
