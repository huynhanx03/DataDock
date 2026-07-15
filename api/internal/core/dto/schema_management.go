package dto

import (
	"time"

	"github.com/huynhanx03/datadock/internal/core/entity"
)

type SchemaActionKind string

const (
	SchemaActionCreateTable       SchemaActionKind = "create_table"
	SchemaActionRenameTable       SchemaActionKind = "rename_table"
	SchemaActionDropTable         SchemaActionKind = "drop_table"
	SchemaActionAddColumn         SchemaActionKind = "add_column"
	SchemaActionRenameColumn      SchemaActionKind = "rename_column"
	SchemaActionAlterColumnType   SchemaActionKind = "alter_column_type"
	SchemaActionSetColumnNullable SchemaActionKind = "set_column_nullable"
	SchemaActionSetColumnDefault  SchemaActionKind = "set_column_default"
	SchemaActionSetColumnComment  SchemaActionKind = "set_column_comment"
	SchemaActionDropColumn        SchemaActionKind = "drop_column"
	SchemaActionAddConstraint     SchemaActionKind = "add_constraint"
	SchemaActionDropConstraint    SchemaActionKind = "drop_constraint"
	SchemaActionCreateIndex       SchemaActionKind = "create_index"
	SchemaActionDropIndex         SchemaActionKind = "drop_index"
	SchemaActionRebuildIndex      SchemaActionKind = "rebuild_index"
	SchemaActionAnalyzeIndex      SchemaActionKind = "analyze_index"
)

type SchemaConstraintType string

const (
	SchemaConstraintPrimaryKey SchemaConstraintType = "primary_key"
	SchemaConstraintForeignKey SchemaConstraintType = "foreign_key"
	SchemaConstraintUnique     SchemaConstraintType = "unique"
	SchemaConstraintCheck      SchemaConstraintType = "check"
)

type SchemaTarget struct {
	Schema string `json:"schema,omitempty"`
	Table  string `json:"table"`
}

type SchemaColumnDefinition struct {
	Name                string  `json:"name"`
	DataType            string  `json:"dataType"`
	Nullable            bool    `json:"nullable"`
	DefaultValue        *string `json:"defaultValue,omitempty"`
	Comment             string  `json:"comment,omitempty"`
	Identity            bool    `json:"identity"`
	GeneratedExpression *string `json:"generatedExpression,omitempty"`
}

type SchemaConstraintDefinition struct {
	Name              string               `json:"name"`
	Type              SchemaConstraintType `json:"type"`
	Columns           []string             `json:"columns,omitempty"`
	ReferencedTarget  *SchemaTarget        `json:"referencedTarget,omitempty"`
	ReferencedColumns []string             `json:"referencedColumns,omitempty"`
	Expression        string               `json:"expression,omitempty"`
	OnUpdate          string               `json:"onUpdate,omitempty"`
	OnDelete          string               `json:"onDelete,omitempty"`
}

type SchemaIndexDefinition struct {
	Name      string   `json:"name"`
	Columns   []string `json:"columns"`
	Unique    bool     `json:"unique"`
	Method    string   `json:"method,omitempty"`
	Predicate string   `json:"predicate,omitempty"`
}

type SchemaAction struct {
	ID           string                      `json:"id,omitempty"`
	Kind         SchemaActionKind            `json:"kind"`
	Target       SchemaTarget                `json:"target"`
	Name         string                      `json:"name,omitempty"`
	NewName      string                      `json:"newName,omitempty"`
	Columns      []SchemaColumnDefinition    `json:"columns,omitempty"`
	Column       *SchemaColumnDefinition     `json:"column,omitempty"`
	Constraint   *SchemaConstraintDefinition `json:"constraint,omitempty"`
	Index        *SchemaIndexDefinition      `json:"index,omitempty"`
	DataType     string                      `json:"dataType,omitempty"`
	Nullable     *bool                       `json:"nullable,omitempty"`
	DefaultValue *string                     `json:"defaultValue,omitempty"`
	Comment      *string                     `json:"comment,omitempty"`
	Cascade      bool                        `json:"cascade"`
}

type SchemaPreviewInput struct {
	Actions []SchemaAction `json:"actions"`
}

type SchemaStep struct {
	Position    int              `json:"position"`
	ActionID    string           `json:"actionId"`
	Kind        SchemaActionKind `json:"kind"`
	SQL         string           `json:"sql"`
	Destructive bool             `json:"destructive"`
}

type SchemaPreview struct {
	ConnectionID string         `json:"connectionId"`
	Engine       entity.Engine  `json:"engine"`
	Actions      []SchemaAction `json:"actions"`
	Steps        []SchemaStep   `json:"steps"`
	SQL          string         `json:"sql"`
	Hash         string         `json:"hash"`
	Destructive  bool           `json:"destructive"`
	GeneratedAt  time.Time      `json:"generatedAt"`
}

type SchemaApplyInput struct {
	Actions            []SchemaAction `json:"actions"`
	PreviewHash        string         `json:"previewHash"`
	ConfirmDestructive bool           `json:"confirmDestructive"`
}

type SchemaApplyResult struct {
	PreviewHash  string    `json:"previewHash"`
	AppliedSteps int       `json:"appliedSteps"`
	AppliedAt    time.Time `json:"appliedAt"`
}
