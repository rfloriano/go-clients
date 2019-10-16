package mocks

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"sync"

	"github.com/vtex/go-io/ioext"

	"github.com/vtex/go-clients/clients"
	"github.com/vtex/go-clients/vbase"
)

func NewVBaseChronos() vbase.VBaseChronos {
	return &fakeChronosVbase{
		buckets: map[string]*vbaseBucket{},
	}
}

type fakeVbaseChronos struct {
	sync.Mutex
	buckets map[string]*vbaseBucket
}

func (r *fakeVbaseChronos) GetJSON(bucket, path string, data interface{}) (string, error) {
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

func (r *fakeVbaseChronos) SaveJSON(bucket, path string, data interface{}) (string, error) {
	bytes, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	return r.SaveFileB(bucket, path, bytes, vbase.SaveFileOptions{
		ContentType: "application/json",
	})
}

func (r *fakeVbaseChronos) SaveFileB(bucket, path string, bytes []byte, opts vbase.SaveFileOptions) (string, error) {
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

func (r *fakeVbaseChronos) DeleteFile(bucket, path string) error {
	r.Lock()
	defer r.Unlock()

	buck := r.getBucket(bucket)
	if _, exists := buck.entries[path]; !exists {
		return clients.ResponseError{StatusCode: http.StatusNotFound}
	}

	delete(buck.entries, path)
	return nil
}

func (r *fakeVbaseChronos) getBucket(name string) *vbaseBucket {
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

func (r *fakeVbaseChronos) getEntry(bucket, path string) (*vbaseBucket, *vbaseEntry, bool) {
	buck := r.getBucket(bucket)
	entry, ok := buck.entries[path]
	if !ok {
		return buck, nil, false
	}
	return buck, entry, true
}
