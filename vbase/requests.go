package vbase

type PatchRequest []*PatchOperation

type PatchOperation struct {
	Type  OperationType `json:"op"`
	Path  string        `json:"path"`
	Value interface{}   `json:"value,omitempty"`
}

type OperationType string

const (
	OperationTypeReplace = OperationType("replace")
	OperationTypeRemove  = OperationType("remove")
)
