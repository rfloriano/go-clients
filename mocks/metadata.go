package mocks

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/vtex/go-clients/clients"
	"github.com/vtex/go-clients/metadata"
)

func NewMetadata() metadata.Metadata {
	return &fakeMetadata{
		buckets: map[string]*bucket{},
	}
}

type bucket struct {
	entries []*metadata.MetadataResponseEntry
	eTag    string
}

type fakeMetadata struct {
	sync.Mutex
	buckets map[string]*bucket
}

func (r *fakeMetadata) GetBucket(bucket string) (*metadata.BucketResponse, string, error) {
	panic("not implemented")
}

func (r *fakeMetadata) SetBucketState(bucket, state string) error {
	panic("not implemented")
}

func (r *fakeMetadata) List(bucket string, options *metadata.Options) (*metadata.MetadataListResponse, string, error) {
	panic("not implemented")
}

func (r *fakeMetadata) ListAll(bucketName string, includeValue bool) (*metadata.MetadataListResponse, string, error) {
	r.Lock()
	defer r.Unlock()
	bucket := r.getBucket(bucketName)
	return &metadata.MetadataListResponse{Data: bucket.entries}, bucket.eTag, nil
}

func (r *fakeMetadata) Get(bucket, key string, data interface{}) (string, error) {
	r.Lock()
	defer r.Unlock()
	_, entry, ok := r.getEntry(bucket, key)
	if !ok {
		return "", clients.ResponseError{StatusCode: http.StatusNotFound}
	}
	if err := json.Unmarshal(entry.Value, data); err != nil {
		return "", err
	}
	return entry.Hash, nil
}

func (r *fakeMetadata) Save(bucketName, key string, data interface{}) (string, error) {
	r.Lock()
	defer r.Unlock()
	return r.saveNoLock(bucketName, key, data)
}

func (r *fakeMetadata) saveNoLock(bucketName, key string, data interface{}) (string, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	bucket, entry, ok := r.getEntry(bucketName, key)
	if ok {
		entry.Value = raw
		entry.Hash = genEtag()
	} else {
		entry = &metadata.MetadataResponseEntry{
			Key:   key,
			Value: raw,
			Hash:  genEtag(),
		}
		bucket.entries = append(bucket.entries, entry)
	}
	bucket.eTag = genEtag()
	return entry.Hash, nil
}

func (r *fakeMetadata) SaveAll(bucket string, data map[string]interface{}) (string, error) {
	r.Lock()
	defer r.Unlock()

	for k, v := range data {
		_, err := r.saveNoLock(bucket, k, v)
		if err != nil {
			return "", err
		}
	}
	return r.getBucket(bucket).eTag, nil
}

func (r *fakeMetadata) DoAll(bucket string, patch metadata.MetadataPatchRequest) error {
	for _, p := range patch {
		var err error
		switch p.Type {
		case metadata.OperationTypeAdd:
		case metadata.OperationTypeReplace:
			_, err = r.Save(bucket, p.Key, p.Value)
		case metadata.OperationTypeRemove:
			_, err = r.Delete(bucket, p.Key)
		default:
			panic("not implemented")
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (r *fakeMetadata) Delete(bucketName, key string) (bool, error) {
	r.Lock()
	defer r.Unlock()

	bucket := r.getBucket(bucketName)
	idx, _, ok := findEntry(bucket.entries, key)
	if !ok {
		return false, nil
	}
	bucket.entries = append(bucket.entries[:idx], bucket.entries[idx+1:]...)
	bucket.eTag = genEtag()
	return true, nil
}

func (r *fakeMetadata) DeleteAll(bucketName string) error {
	r.Lock()
	defer r.Unlock()

	bucket := r.getBucket(bucketName)
	bucket.entries = nil
	bucket.eTag = genEtag()
	return nil
}

func (r *fakeMetadata) ListAllConflicts(bucket string) ([]*metadata.MetadataConflict, error) {
	panic("not implemented")
}

func (r *fakeMetadata) ResolveConflicts(bucket string, patch metadata.MetadataPatchRequest) error {
	panic("not implemented")
}

func (r *fakeMetadata) getBucket(name string) *bucket {
	buck, ok := r.buckets[name]
	if !ok {
		buck = &bucket{
			eTag: genEtag(),
		}
		r.buckets[name] = buck
	}
	return buck
}

func (r *fakeMetadata) getEntry(bucketName, key string) (*bucket, *metadata.MetadataResponseEntry, bool) {
	bucket := r.getBucket(bucketName)
	_, entry, ok := findEntry(bucket.entries, key)
	return bucket, entry, ok
}

var etagNum int32

func genEtag() string {
	return fmt.Sprintf("random-etag-%d", atomic.AddInt32(&etagNum, 1))
}

func findEntry(entries []*metadata.MetadataResponseEntry, key string) (int, *metadata.MetadataResponseEntry, bool) {
	for i, entry := range entries {
		if entry.Key == key {
			return i, entry, true
		}
	}
	return -1, nil, false
}
