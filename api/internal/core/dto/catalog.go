package dto

import (
	"time"

	"github.com/huynhanx03/datadock/internal/core/entity"
)

type CatalogObjectKind string

const (
	CatalogObjectDatabase         CatalogObjectKind = "database"
	CatalogObjectSchema           CatalogObjectKind = "schema"
	CatalogObjectGroup            CatalogObjectKind = "group"
	CatalogObjectTable            CatalogObjectKind = "table"
	CatalogObjectView             CatalogObjectKind = "view"
	CatalogObjectMaterializedView CatalogObjectKind = "materialized-view"
	CatalogObjectFunction         CatalogObjectKind = "function"
	CatalogObjectProcedure        CatalogObjectKind = "procedure"
	CatalogObjectSequence         CatalogObjectKind = "sequence"
	CatalogObjectExtension        CatalogObjectKind = "extension"
	CatalogObjectTrigger          CatalogObjectKind = "trigger"
	CatalogObjectColumn           CatalogObjectKind = "column"
)

type CatalogChildrenState string

const (
	CatalogChildrenUnloaded    CatalogChildrenState = "unloaded"
	CatalogChildrenLoaded      CatalogChildrenState = "loaded"
	CatalogChildrenEmpty       CatalogChildrenState = "empty"
	CatalogChildrenUnsupported CatalogChildrenState = "unsupported"
)

type CatalogDepth string

const (
	CatalogDepthSummary CatalogDepth = "summary"
	CatalogDepthAll     CatalogDepth = "all"
)

type CatalogInput struct {
	ParentReference string       `json:"parentReference"`
	Cursor          string       `json:"cursor"`
	Limit           int          `json:"limit"`
	Depth           CatalogDepth `json:"depth"`
}

type CatalogTree struct {
	ConnectionID string          `json:"connectionId"`
	Engine       entity.Engine   `json:"engine"`
	Capabilities []string        `json:"capabilities"`
	Databases    []CatalogObject `json:"databases"`
	NextCursor   string          `json:"nextCursor,omitempty"`
	LoadedAt     time.Time       `json:"loadedAt"`
}

type CatalogObject struct {
	ID            string               `json:"id"`
	Reference     string               `json:"reference"`
	ConnectionID  string               `json:"connectionId"`
	ParentID      string               `json:"parentId,omitempty"`
	Name          string               `json:"name"`
	QualifiedName string               `json:"qualifiedName"`
	Kind          CatalogObjectKind    `json:"kind"`
	Database      string               `json:"database,omitempty"`
	Schema        string               `json:"schema,omitempty"`
	DataType      string               `json:"dataType,omitempty"`
	Count         *int64               `json:"count,omitempty"`
	ChildrenState CatalogChildrenState `json:"childrenState"`
	Capabilities  []string             `json:"capabilities,omitempty"`
	Children      []CatalogObject      `json:"children,omitempty"`
}
