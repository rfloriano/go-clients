package mocks

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"sync"

	"github.com/vtex/go-io/ioext"

	"github.com/vtex/go-clients/clients"
	"github.com/vtex/go-clients/vbase"
)

func NewVBase() vbase.VBase {
	return &fakeVbase{
		buckets: map[string]*vbaseBucket{},
	}
}

type vbaseEntry struct {
	value       []byte
	contentType string
	eTag        string
}

type vbaseBucket struct {
	entries map[string]*vbaseEntry
	eTag    string
}

type fakeVbase struct {
	sync.Mutex
	buckets map[string]*vbaseBucket
}

func (r *fakeVbase) GetBucket(bucket string) (*vbase.BucketResponse, string, error) {
	panic("not implemented")
}

func (r *fakeVbase) ListAllConflicts(bucket string) ([]*vbase.Conflict, error) {
	panic("not implemented")
}

func (r *fakeVbase) ResolveConflicts(bucket string, patch vbase.PatchRequest) error {
	panic("not implemented")
}

func (r *fakeVbase) ListFiles(bucket string, options *vbase.Options) (*vbase.FileListResponse, string, error) {
	panic("deprecated api")
}

func (r *fakeVbase) ListAllFiles(bucket, prefix string) (*vbase.FileListResponse, string, error) {
	panic("deprecated api")
}

func (r *fakeVbase) DeleteAllFiles(bucket string) error {
	panic("deprecated api")
}

func (r *fakeVbase) GetFile(bucket, path string) (file io.ReadCloser, contentType string, err error) {
	r.Lock()
	defer r.Unlock()

	_, entry, ok := r.getEntry(bucket, path)
	if !ok {
		return nil, "", clients.ResponseError{StatusCode: http.StatusNotFound}
	}
	return ioutil.NopCloser(bytes.NewReader(entry.value)), entry.contentType, nil
}

func (r *fakeVbase) GetJSON(bucket, path string, data interface{}) (string, error) {
	r.Lock()
	defer r.Unlock()

	_, entry, ok := r.getEntry(bucket, path)
	if !ok {
		return "", clients.ResponseError{StatusCode: http.StatusNotFound}
	}

	if err := json.Unmarshal(entry.value, data); err != nil {
		return "", err
	}
	return entry.eTag, nil
}

func (r *fakeVbase) SaveFile(bucket, path string, body io.Reader, opts vbase.SaveFileOptions) (string, error) {
	bytes, err := ioutil.ReadAll(body)
	if err != nil {
		return "", err
	}
	return r.SaveFileB(bucket, path, bytes, opts)
}

func (r *fakeVbase) SaveJSON(bucket, path string, data interface{}) (string, error) {
	bytes, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	return r.SaveFileB(bucket, path, bytes, vbase.SaveFileOptions{
		ContentType: "application/json",
	})
}

func (r *fakeVbase) SaveFileB(bucket, path string, bytes []byte, opts vbase.SaveFileOptions) (string, error) {
	r.Lock()
	defer r.Unlock()

	buck := r.getBucket(bucket)
	buck.eTag = genEtag()

	if !opts.Unzip {
		if opts.ContentType == "" {
			opts.ContentType = "text/plain"
		}
		entry := &vbaseEntry{
			value:       bytes,
			eTag:        genEtag(),
			contentType: opts.ContentType,
		}
		buck.entries[path] = entry
		return entry.eTag, nil
	}

	files, err := ioext.ZipExtract(bytes)
	if err != nil {
		return "", err
	}
	for filePath, content := range files {
		fullPath := filepath.Join(path, filePath)
		buck.entries[fullPath] = &vbaseEntry{
			value:       content,
			eTag:        genEtag(),
			contentType: "text/plain",
		}
	}
	return buck.eTag, nil
}

func (r *fakeVbase) DeleteFile(bucket, path string) error {
	r.Lock()
	defer r.Unlock()

	buck := r.getBucket(bucket)
	if _, exists := buck.entries[path]; !exists {
		return clients.ResponseError{StatusCode: http.StatusNotFound}
	}

	delete(buck.entries, path)
	return nil
}

func (r *fakeVbase) getBucket(name string) *vbaseBucket {
	buck, ok := r.buckets[name]
	if !ok {
		buck = &vbaseBucket{
			entries: map[string]*vbaseEntry{},
			eTag:    genEtag(),
		}
		r.buckets[name] = buck
	}
	return buck
}

func (r *fakeVbase) getEntry(bucket, path string) (*vbaseBucket, *vbaseEntry, bool) {
	buck := r.getBucket(bucket)
	entry, ok := buck.entries[path]
	if !ok {
		return buck, nil, false
	}
	return buck, entry, true
}
