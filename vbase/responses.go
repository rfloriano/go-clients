package vbase

import "encoding/json"

// BucketResponse is the description of a bucket's state
type BucketResponse struct {
	Hash  string `json:"hash"`
	State string `json:"state"`
}

// FileEntryResponse is the description of an entry in a FileListResponse
type FileEntryResponse struct {
	Path  string          `json:"path"`
	Hash  string          `json:"hash"`
	Value json.RawMessage `json:"value"`
}

// FileListResponse is the description of file list
type FileListResponse struct {
	Files      []*FileEntryResponse `json:"data"`
	NextMarker string               `json:"next"`
}

// ConflictListResponse is a list of Conflicts
type ConflictListResponse struct {
	Data []*Conflict `json:"data"`
}

// Conflict contains multiple versions of a file
type Conflict struct {
	Path   string         `json:"path"`
	Base   *ConflictEntry `json:"base"`
	Mine   *ConflictEntry `json:"mine"`
	Master *ConflictEntry `json:"master"`
}

// ConflictEntry holds a version (base, mine or master) of the conflicted file
type ConflictEntry struct {
	Deleted        bool   `json:"deleted"`
	MIMEType       string `json:"mimeType"`
	Content        []byte `json:"content"`
	ContentOmitted bool   `json:"contentOmitted"`
}
