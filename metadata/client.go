package metadata

import (
	"fmt"
	"net/http"

	"strconv"

	"sync"

	"strings"

	"github.com/vtex/go-clients/clients"
	"gopkg.in/h2non/gentleman.v1"
)

const detectConflictsHeader = "X-Vtex-Detect-Conflicts"

type Options struct {
	IncludeValue bool
	Limit        int
	Marker       string
}

type Metadata interface {
	GetBucket(bucket string) (*BucketResponse, string, error)
	SetBucketState(bucket, state string) error
	List(bucket string, options *Options) (*MetadataListResponse, string, error)
	ListAll(bucket string, includeValue bool) (*MetadataListResponse, string, error)
	Get(bucket, key string, data interface{}) (string, error)
	Save(bucket, key string, data interface{}) (string, error)
	SaveAll(bucket string, data map[string]interface{}) (string, error)
	DoAll(bucket string, patch MetadataPatchRequest) error
	Delete(bucket, key string) (bool, error)
	DeleteAll(bucket string) error
	ListAllConflicts(bucket string) ([]*MetadataConflict, error)
	ResolveConflicts(bucket string, patch MetadataPatchRequest) error
}

type ConflictResolver interface {
	Resolve(client Metadata, bucketDetected string) (resolved bool, err error)
}

type Client struct {
	http             *gentleman.Client
	conflictResolver ConflictResolver
}

// NewClient creates a Metadata client with specified configuration. Conflict
// resolver is optional but if set, will be called for each detected conflict in
// metadata access methods to attempt a resolution logic.
func NewClient(config *clients.Config, resolver ConflictResolver) Metadata {
	cl := clients.CreateClient("kube-router", config, true)
	return &Client{cl, resolver}
}

const (
	bucketPath      = "/buckets/%v"
	bucketStatePath = "/buckets/%v/state"
	conflictsPath   = "/buckets/%v/conflicts"
	metadataPath    = "/buckets/%v/metadata"
	metadataKeyPath = "/buckets/%v/metadata/%v"
)

func (cl *Client) GetBucket(bucket string) (*BucketResponse, string, error) {
	res, err := cl.http.Get().
		AddPath(fmt.Sprintf(bucketPath, bucket)).Send()
	if err != nil {
		return nil, "", err
	}

	var bucketResponse BucketResponse
	if err := res.JSON(&bucketResponse); err != nil {
		return nil, "", err
	}

	return &bucketResponse, res.Header.Get("ETag"), nil
}

func (cl *Client) SetBucketState(bucket, state string) error {
	_, err := cl.http.Put().
		AddPath(fmt.Sprintf(bucketStatePath, bucket)).
		JSON(state).Send()
	if err != nil {
		return err
	}
	return nil
}

func (cl *Client) List(bucket string, options *Options) (*MetadataListResponse, string, error) {
	if options.Limit <= 0 {
		options.Limit = 10
	}

	req := cl.http.Get().
		AddPath(fmt.Sprintf(metadataPath, bucket)).
		SetQueryParams(map[string]string{
			"value":   strconv.FormatBool(options.IncludeValue),
			"_limit":  strconv.Itoa(options.Limit),
			"_marker": options.Marker,
		})
	res, err := cl.performConflictResolved(bucket, req)

	if err != nil {
		return nil, "", err
	}

	var metadata MetadataListResponse
	if err := res.JSON(&metadata); err != nil {
		return nil, "", err
	}

	return &metadata, res.Header.Get(clients.HeaderETag), nil
}

func (cl *Client) ListAll(bucket string, includeValue bool) (*MetadataListResponse, string, error) {
	options := &Options{
		Limit:        100,
		IncludeValue: includeValue,
	}

	list, eTag, err := cl.List(bucket, options)
	if err != nil {
		return nil, "", err
	}

	for {
		if list.NextMarker == "" {
			break
		}
		options.Marker = list.NextMarker

		partialList, newETag, err := cl.List(bucket, options)
		if err != nil {
			return nil, "", err
		}

		list.Data = append(list.Data, partialList.Data...)
		list.NextMarker = partialList.NextMarker
		eTag = newETag
	}
	return list, eTag, nil
}

func (cl *Client) Get(bucket, key string, data interface{}) (string, error) {
	req := cl.http.Get().
		AddPath(fmt.Sprintf(metadataKeyPath, bucket, key))
	res, err := cl.performConflictResolved(bucket, req)
	if err != nil {
		return "", err
	}

	if err := res.JSON(data); err != nil {
		return "", err
	}

	return res.Header.Get("ETag"), nil
}

func (cl *Client) Save(bucket, key string, data interface{}) (string, error) {
	req := cl.http.Put().
		AddPath(fmt.Sprintf(metadataKeyPath, bucket, key)).
		JSON(data)
	res, err := cl.performConflictResolved(bucket, req)

	if err != nil {
		return "", err
	}

	return res.Header.Get("ETag"), nil
}

func (cl *Client) SaveAll(bucket string, data map[string]interface{}) (string, error) {
	req := cl.http.Put().
		AddPath(fmt.Sprintf(metadataPath, bucket)).
		JSON(data)
	res, err := cl.performConflictResolved(bucket, req)

	if err != nil {
		return "", err
	}

	return res.Header.Get("ETag"), nil
}

func (cl *Client) Delete(bucket, key string) (bool, error) {
	req := cl.http.Delete().
		AddPath(fmt.Sprintf(metadataKeyPath, bucket, key))
	_, err := cl.performConflictResolved(bucket, req)

	if err != nil {
		if err, ok := err.(clients.ResponseError); ok && err.StatusCode == http.StatusNotFound {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func (cl *Client) DeleteAll(bucket string) error {
	_, err := cl.http.Delete().
		AddPath(fmt.Sprintf(metadataPath, bucket)).
		Send()

	return err
}

func (cl *Client) DoAll(bucket string, patch MetadataPatchRequest) error {
	toSave := map[string]interface{}{}
	// not to block goroutines, assume at most one error per operation
	errs := make(chan error, len(patch))

	wg := sync.WaitGroup{}
	for _, op := range patch {
		switch op.Type {
		case OperationTypeAdd, OperationTypeReplace:
			toSave[op.Key] = op.Value
		case OperationTypeRemove:
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := cl.Delete(bucket, op.Key)
				if err != nil {
					errs <- fmt.Errorf("Delete %s: %v", op.Key, err)
				}
			}()
		}
	}

	if len(toSave) > 0 {
		_, err := cl.SaveAll(bucket, toSave)
		if err != nil {
			errs <- fmt.Errorf("Save keys %v: %v", mapKeys(toSave), err)
		}
	}

	wg.Wait()
	close(errs)

	errCount := len(errs)
	if errCount > 0 {
		errMsgs := make([]string, 0, errCount)
		for err := range errs {
			errMsgs = append(errMsgs, err.Error())
		}

		return fmt.Errorf("Error(s) in metadata patch in bucket %s: %s", bucket, strings.Join(errMsgs, "; "))
	}

	return nil
}

func (cl *Client) ListAllConflicts(bucket string) ([]*MetadataConflict, error) {
	res, err := cl.http.Get().
		AddPath(fmt.Sprintf(conflictsPath, bucket)).
		Send()
	if err != nil {
		return nil, err
	}

	var response MetadataConflictListResponse
	if err := res.JSON(&response); err != nil {
		return nil, fmt.Errorf("Error unmarshaling metadata conflicts: %v", err)
	}

	return response.Data, nil
}

func (cl *Client) ResolveConflicts(bucket string, patch MetadataPatchRequest) error {
	_, err := cl.http.Post().
		AddPath(fmt.Sprintf(conflictsPath, bucket)).
		JSON(patch).
		Send()
	return err
}

func (cl *Client) performConflictResolved(bucket string, req *gentleman.Request) (*gentleman.Response, error) {
	if cl.conflictResolver == nil {
		return req.Send()
	}
	req.SetHeader(detectConflictsHeader, "true")

	for {
		// Clone request before sending or we won't be able to retry.
		res, err := req.Clone().Send()

		if isConflict(err) {
			resolved, resolveErr := cl.conflictResolver.Resolve(cl, bucket)
			if resolveErr != nil {
				return nil, fmt.Errorf("Error resolving conflicts: %v", resolveErr)
			} else if !resolved {
				return nil, err
			}

			// Retry the request after conflicts resolved
			continue
		}

		return res, err
	}
}

func isConflict(err error) bool {
	if respErr, ok := err.(clients.ResponseError); ok && respErr.StatusCode == http.StatusConflict {
		return true
	}
	return false
}

func mapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
