package vbase

import "encoding/json"

// BucketResponse is the description of a bucket's state
type BucketResponse struct {
	Hash  string `json:"hash"`
	State string `json:"state"`
}

// FileListEntryResponse is the description of an entry in a FileListResponse
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

// Conflict a 409 response's payload
type Conflict struct {
	Key    string         `json:"key"`
	Master *ConflictEntry `json:"master"`
	Base   *ConflictEntry `json:"base"`
	Mine   *ConflictEntry `json:"mine"`
}

// ConflictEntry is a Conflict's item
type ConflictEntry struct {
	Value json.RawMessage `json:"value"`
}
