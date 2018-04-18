package vbase

import (
	"bytes"
	"fmt"
	"io"
	"net/http"

	"strconv"

	"github.com/vtex/go-clients/clients"
	"github.com/vtex/go-clients/metadata"
	"gopkg.in/h2non/gentleman.v1"
	"gopkg.in/h2non/gentleman.v1/plugins/headers"
)

type Options struct {
	Prefix string
	Marker string
	Limit  int
}

type SaveFileOptions struct {
	ContentType string
	Unzip       bool
}

// VBase is an interface for interacting with VBase
type VBase interface {
	GetFile(bucket, path string) (*gentleman.Response, string, error)
	GetJSON(bucket, key string, data interface{}) (string, error)
	ListFiles(bucket string, options *Options) (*FileListResponse, string, error)
	ListAllFiles(bucket, prefix string) (*FileListResponse, string, error)

	SaveFile(bucket, path string, body io.Reader, opts SaveFileOptions) (string, error)
	SaveFileB(bucket, path string, content []byte, opts SaveFileOptions) (string, error)
	SaveJSON(bucket, key string, data interface{}) (string, error)
	DeleteFile(bucket, path string) error
	DeleteAllFiles(bucket string) error

	GetBucket(bucket string) (*BucketResponse, string, error)
	SetBucketState(bucket, state string) (string, error)
	GetFileConflict(bucket, path string) (*gentleman.Response, *Conflict, string, error)
}

// VBaseWithFallback is an interface for interacting with VBase with fallback to Metadata
type VBaseWithFallback interface {
	VBase
	GetJSONWithFallback(vbaseBucket, metadataBucket, key string, data interface{}) (string, error)
}

type client struct {
	http    *gentleman.Client
	appName string
}

type clientWithFallback struct {
	*client
	metadata metadata.Metadata
}

// NewClient creates a new Workspaces client
func NewClient(config *clients.Config) (VBase, error) {
	cl := clients.CreateClient("vbase", config, true)
	appName := clients.UserAgentName(config)
	if appName == "" {
		return nil, clients.NewNoUserAgentError("User-Agent is missing to create a VBase client.")
	}
	return &client{cl, appName}, nil
}

// NewClientFallback creates a new Workspaces client with fallback to Metadata
func NewClientFallback(vbaseConfig, metadataConfig *clients.Config) (VBaseWithFallback, error) {
	cl, err := NewClient(vbaseConfig)
	if err != nil {
		return nil, err
	}
	clMetadata, err := metadata.NewClient(metadataConfig, nil)
	if err != nil {
		return nil, err
	}
	return &clientWithFallback{cl.(*client), clMetadata}, nil
}

const (
	pathToBucket      = "/buckets/%v/%v"
	pathToBucketState = "/buckets/%v/%v/state"
	pathToFileList    = "/buckets/%v/%v/files"
	pathToFile        = "/buckets/%v/%v/files/%v"
)

// GetBucket describes the current state of a bucket
func (cl *client) GetBucket(bucket string) (*BucketResponse, string, error) {
	res, err := cl.http.Get().
		AddPath(fmt.Sprintf(pathToBucket, cl.appName, bucket)).Send()
	if err != nil {
		return nil, "", err
	}

	var bucketResponse BucketResponse
	if err := res.JSON(&bucketResponse); err != nil {
		return nil, "", err
	}

	return &bucketResponse, res.Header.Get(clients.HeaderETag), nil
}

// SetBucketState sets the current state of a bucket
func (cl *client) SetBucketState(bucket, state string) (string, error) {
	_, err := cl.http.Put().
		AddPath(fmt.Sprintf(pathToBucketState, cl.appName, bucket)).
		JSON(state).Send()
	if err != nil {
		return "", err
	}

	return "", nil
}

// GetJSON populates data with the content of the specified file, assuming it is serialized as JSON
func (cl *client) GetJSON(bucket, path string, data interface{}) (string, error) {
	res, etag, err := cl.GetFile(bucket, path)
	if err != nil {
		return "", err
	}

	if err := res.JSON(data); err != nil {
		return "", err
	}

	return etag, nil
}

func (cl *clientWithFallback) GetJSONWithFallback(vbaseBucket, metadataBucket, path string, data interface{}) (string, error) {
	etag, err := cl.GetJSON(vbaseBucket, path, data)
	if rerr, ok := err.(clients.ResponseError); ok && rerr.StatusCode == http.StatusNotFound {
		etag, err = cl.metadata.Get(metadataBucket, path, data)
		if err == nil {
			etag, err = cl.SaveJSON(vbaseBucket, path, data)
		}
	}
	return etag, err
}

// GetFile gets a file's content as a read closer
func (cl *client) GetFile(bucket, path string) (*gentleman.Response, string, error) {
	res, err := cl.http.Get().AddPath(fmt.Sprintf(pathToFile, cl.appName, bucket, path)).Send()
	if err != nil {
		return nil, res.Header.Get(clients.HeaderETag), err
	}

	return res, res.Header.Get(clients.HeaderETag), nil
}

// GetFileConflict gets a file's content as a byte slice, or conflict
func (cl *client) GetFileConflict(bucket, path string) (*gentleman.Response, *Conflict, string, error) {
	req := cl.http.Get().
		AddPath(fmt.Sprintf(pathToFile, cl.appName, bucket, path)).
		Use(headers.Set("x-conflict-resolution", "merge"))

	res, err := req.Send()
	if err != nil {
		if err, ok := err.(clients.ResponseError); ok && err.StatusCode == 409 {
			var conflict Conflict
			if err := res.JSON(&conflict); err != nil {
				return nil, nil, "", err
			}
			return nil, &conflict, res.Header.Get(clients.HeaderETag), nil
		}
		return nil, nil, "", err
	}

	return res, nil, res.Header.Get(clients.HeaderETag), nil
}

// SaveJSON saves generic data serializing it to JSON
func (cl *client) SaveJSON(bucket, path string, data interface{}) (string, error) {
	res, err := cl.http.Put().
		AddPath(fmt.Sprintf(pathToFile, cl.appName, bucket, path)).
		JSON(data).Send()

	if err != nil {
		return "", err
	}

	return res.Header.Get(clients.HeaderETag), nil
}

// SaveFile saves a file to a workspace
func (cl *client) SaveFile(bucket, path string, body io.Reader, opts SaveFileOptions) (string, error) {
	req := cl.http.Put().
		AddPath(fmt.Sprintf(pathToFile, cl.appName, bucket, path)).
		SetQuery("unzip", fmt.Sprintf("%v", opts.Unzip)).
		Body(body)
	if opts.ContentType != "" {
		req = req.SetHeader("Content-Type", opts.ContentType)
	}

	res, err := req.Send()
	if err != nil {
		return "", err
	}
	return res.Header.Get(clients.HeaderETag), nil
}

// SaveFileB saves a file to a workspace
func (cl *client) SaveFileB(bucket, path string, body []byte, opts SaveFileOptions) (string, error) {
	return cl.SaveFile(bucket, path, bytes.NewReader(body), opts)
}

// ListFiles returns a list of files, given a prefix
func (cl *client) ListFiles(bucket string, options *Options) (*FileListResponse, string, error) {
	if options.Limit <= 0 {
		options.Limit = 10
	}

	res, err := cl.http.Get().
		AddPath(fmt.Sprintf(pathToFileList, cl.appName, bucket)).
		SetQueryParams(map[string]string{
			"prefix": options.Prefix,
			"_next":  options.Marker,
			"_limit": strconv.Itoa(options.Limit),
		}).Send()

	if err != nil {
		return nil, "", err
	}

	var fileListResponse FileListResponse
	if err := res.JSON(&fileListResponse); err != nil {
		return nil, "", err
	}

	return &fileListResponse, res.Header.Get(clients.HeaderETag), nil
}

// ListAllFiles returns a complete list of files, given a prefix
func (cl *client) ListAllFiles(bucket, prefix string) (*FileListResponse, string, error) {
	options := &Options{
		Limit:  100,
		Prefix: prefix,
	}

	list, eTag, err := cl.ListFiles(bucket, options)
	if err != nil {
		return nil, "", err
	}

	for {
		if list.NextMarker == "" {
			break
		}
		options.Marker = list.NextMarker

		partialList, newETag, err := cl.ListFiles(bucket, options)
		if err != nil {
			return nil, "", err
		}

		list.Files = append(list.Files, partialList.Files...)
		list.NextMarker = partialList.NextMarker
		eTag = newETag
	}
	return list, eTag, nil
}

// DeleteFile deletes a file from the workspace
func (cl *client) DeleteFile(bucket, path string) error {
	_, err := cl.http.Delete().
		AddPath(fmt.Sprintf(pathToFile, cl.appName, bucket, path)).
		Send()

	return err
}

// DeleteAllFiles deletes all files from the specificed bucket
func (cl *client) DeleteAllFiles(bucket string) error {
	_, err := cl.http.Delete().
		AddPath(fmt.Sprintf(pathToFileList, cl.appName, bucket)).
		Send()

	return err
}
