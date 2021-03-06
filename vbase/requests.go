package vbase

type PatchRequest []*PatchOperation

type PatchOperation struct {
	Type  OperationType `json:"op"`
	Path  string        `json:"path"`
	Value PatchValue    `json:"value,omitempty"`
}

type PatchValue struct {
	MIMEType string `json:"mimeType"`
	Content  []byte `json:"content"`
}

type OperationType string

const (
	OperationTypeReplace = OperationType("replace")
	OperationTypeRemove  = OperationType("remove")
)
