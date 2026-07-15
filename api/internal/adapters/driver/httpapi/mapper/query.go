package mapper

import "github.com/huynhanx03/datadock/internal/core/dto"

type QueryExecuteRequest dto.QueryExecutionInput
type QueryExplainRequest dto.QueryExplainInput
type QueryHistoryDeleteRequest dto.QueryHistoryDeleteInput
type QueryExecutionResponse dto.QueryExecutionResult
type QueryHistoryResponse dto.QueryHistoryItem

func ToQueryExecutionInput(request QueryExecuteRequest) dto.QueryExecutionInput {
	return dto.QueryExecutionInput(request)
}

func ToQueryExplainInput(request QueryExplainRequest) dto.QueryExplainInput {
	return dto.QueryExplainInput(request)
}

func ToQueryHistoryDeleteInput(request QueryHistoryDeleteRequest) dto.QueryHistoryDeleteInput {
	return dto.QueryHistoryDeleteInput(request)
}

func ToQueryExecutionResponse(result dto.QueryExecutionResult) QueryExecutionResponse {
	return QueryExecutionResponse(result)
}

func ToQueryHistoryResponses(items []dto.QueryHistoryItem) []QueryHistoryResponse {
	result := make([]QueryHistoryResponse, len(items))
	for index, item := range items {
		result[index] = QueryHistoryResponse(item)
	}
	return result
}
