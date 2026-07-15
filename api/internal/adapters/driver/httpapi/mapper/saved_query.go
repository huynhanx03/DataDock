package mapper

import "github.com/huynhanx03/datadock/internal/core/dto"

type SavedQueryCreateRequest dto.SavedQueryCreateInput
type SavedQueryUpdateRequest dto.SavedQueryUpdateInput
type SavedQueryFavoriteRequest struct {
	Favorite *bool `json:"favorite"`
}
type SavedQueryShareRequest dto.SavedQueryShareInput
type SavedQueryFolderCreateRequest dto.SavedQueryFolderCreateInput
type SavedQueryFolderUpdateRequest dto.SavedQueryFolderUpdateInput
type SavedQueryFolderDeleteRequest dto.SavedQueryFolderDeleteInput
type SavedQueryResponse dto.SavedQueryDetail
type SavedQueryFolderResponse dto.SavedQueryFolderDetail
type SavedQueryShareResponse dto.SavedQueryShareDetail
type PublicSavedQueryResponse dto.PublicSavedQuery

func ToSavedQueryCreateInput(request SavedQueryCreateRequest) dto.SavedQueryCreateInput {
	return dto.SavedQueryCreateInput(request)
}

func ToSavedQueryUpdateInput(request SavedQueryUpdateRequest) dto.SavedQueryUpdateInput {
	return dto.SavedQueryUpdateInput(request)
}

func ToSavedQueryShareInput(request SavedQueryShareRequest) dto.SavedQueryShareInput {
	return dto.SavedQueryShareInput(request)
}

func ToSavedQueryFolderCreateInput(request SavedQueryFolderCreateRequest) dto.SavedQueryFolderCreateInput {
	return dto.SavedQueryFolderCreateInput(request)
}

func ToSavedQueryFolderUpdateInput(request SavedQueryFolderUpdateRequest) dto.SavedQueryFolderUpdateInput {
	return dto.SavedQueryFolderUpdateInput(request)
}

func ToSavedQueryFolderDeleteInput(request SavedQueryFolderDeleteRequest) dto.SavedQueryFolderDeleteInput {
	return dto.SavedQueryFolderDeleteInput(request)
}

func ToSavedQueryResponse(query dto.SavedQueryDetail) SavedQueryResponse {
	return SavedQueryResponse(query)
}

func ToSavedQueryResponses(queries []dto.SavedQueryDetail) []SavedQueryResponse {
	result := make([]SavedQueryResponse, len(queries))
	for index, query := range queries {
		result[index] = ToSavedQueryResponse(query)
	}
	return result
}

func ToSavedQueryFolderResponse(folder dto.SavedQueryFolderDetail) SavedQueryFolderResponse {
	return SavedQueryFolderResponse(folder)
}

func ToSavedQueryFolderResponses(folders []dto.SavedQueryFolderDetail) []SavedQueryFolderResponse {
	result := make([]SavedQueryFolderResponse, len(folders))
	for index, folder := range folders {
		result[index] = ToSavedQueryFolderResponse(folder)
	}
	return result
}

func ToSavedQueryShareResponse(share dto.SavedQueryShareDetail) SavedQueryShareResponse {
	return SavedQueryShareResponse(share)
}

func ToPublicSavedQueryResponse(query dto.PublicSavedQuery) PublicSavedQueryResponse {
	return PublicSavedQueryResponse(query)
}
