package mapper

import (
	"github.com/huynhanx03/datadock/internal/core/dto"
	"github.com/huynhanx03/datadock/internal/core/entity"
)

type ConnectionCreateRequest dto.ConnectionInput
type ConnectionPatchRequest dto.ConnectionPatchInput
type ConnectionFavoriteRequest dto.ConnectionFavoriteInput
type ConnectionResponse dto.ConnectionView
type ConnectionTestResponse dto.ConnectionTestResult
type ConnectionStatusResponse entity.ConnectionRuntimeStatus

func ToConnectionCreateInput(request ConnectionCreateRequest) dto.ConnectionInput {
	return dto.ConnectionInput(request)
}

func ToConnectionPatchInput(request ConnectionPatchRequest) dto.ConnectionPatchInput {
	return dto.ConnectionPatchInput(request)
}

func ToConnectionFavoriteInput(request ConnectionFavoriteRequest) dto.ConnectionFavoriteInput {
	return dto.ConnectionFavoriteInput(request)
}

func ToConnectionResponse(view dto.ConnectionView) ConnectionResponse {
	return ConnectionResponse(view)
}

func ToConnectionResponses(views []dto.ConnectionView) []ConnectionResponse {
	result := make([]ConnectionResponse, len(views))
	for index, view := range views {
		result[index] = ToConnectionResponse(view)
	}
	return result
}

func ToConnectionTestResponse(result dto.ConnectionTestResult) ConnectionTestResponse {
	return ConnectionTestResponse(result)
}

func ToConnectionStatusResponse(result entity.ConnectionRuntimeStatus) ConnectionStatusResponse {
	return ConnectionStatusResponse(result)
}
