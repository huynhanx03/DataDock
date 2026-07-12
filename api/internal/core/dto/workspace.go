package dto

type CreateWorkspaceInput struct {
	Name  string `json:"name" binding:"required,max=80"`
	Icon  string `json:"icon" binding:"max=32"`
	Color string `json:"color" binding:"max=32"`
}

type UpdateWorkspaceInput struct {
	Name      string `json:"name" binding:"required,max=80"`
	Icon      string `json:"icon" binding:"max=32"`
	Color     string `json:"color" binding:"max=32"`
	Collapsed bool   `json:"collapsed"`
}

type ReorderWorkspaceInput struct {
	IDs []string `json:"ids" binding:"required,min=1"`
}
