package vbase

import (
	"bytes"
	"fmt"
	"io"

	"strconv"

	"github.com/vtex/go-clients/clients"
	"gopkg.in/h2non/gentleman.v1"
	"gopkg.in/h2non/gentleman.v1/plugins/headers"
)

type Options struct {
	Prefix string
	Marker string
	Limit  int
}

// VBase is an interface for interacting with VBase
type VBase interface {
	GetJSON(bucket, key string, data interface{}) (string, error)
	SaveJSON(bucket, key string, data interface{}) (string, error)
	GetBucket(bucket string) (*BucketResponse, string, error)
	SetBucketState(bucket, state string) (string, error)
	GetFile(bucket, path string) (*gentleman.Response, string, error)
	GetFileConflict(bucket, path string) (*gentleman.Response, *Conflict, string, error)
	SaveFile(bucket, path string, body io.Reader) (string, error)
	SaveFileB(bucket, path string, content []byte, contentType string, unzip bool) (string, error)
	ListFiles(bucket string, options *Options) (*FileListResponse, string, error)
	ListAllFiles(bucket, prefix string) (*FileListResponse, string, error)
	DeleteFile(bucket, path string) error
	DeleteAllFiles(bucket string) error
}

// Client is a struct that provides interaction with workspaces
type Client struct {
	http    *gentleman.Client
	appName string
}

// NewClient creates a new Workspaces client
func NewClient(config *clients.Config) (VBase, error) {
	cl := clients.CreateClient("vbase", config, true)
	appName := clients.UserAgentName(config)
	if appName == "" {
		return nil, clients.NewNoUserAgentError("User-Agent is missing to create a Metadata cient.")
	}
	return &Client{cl, appName}, nil
}

const (
	pathToBucket      = "/buckets/%v/%v"
	pathToBucketState = "/buckets/%v/%v/state"
	pathToFileList    = "/buckets/%v/%v/files"
	pathToFile        = "/buckets/%v/%v/files/%v"
)

// GetBucket describes the current state of a bucket
func (cl *Client) GetBucket(bucket string) (*BucketResponse, string, error) {
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
func (cl *Client) SetBucketState(bucket, state string) (string, error) {
	_, err := cl.http.Put().
		AddPath(fmt.Sprintf(pathToBucketState, cl.appName, bucket)).
		JSON(state).Send()
	if err != nil {
		return "", err
	}

	return "", nil
}

// GetJSON populates data with the content of the specified file, assuming it is serialized as JSON
func (cl *Client) GetJSON(bucket, path string, data interface{}) (string, error) {
	res, etag, err := cl.GetFile(bucket, path)
	if err != nil {
		return "", err
	}

	if err := res.JSON(data); err != nil {
		return "", err
	}

	return etag, nil
}

// GetFile gets a file's content as a read closer
func (cl *Client) GetFile(bucket, path string) (*gentleman.Response, string, error) {
	res, err := cl.http.Get().AddPath(fmt.Sprintf(pathToFile, cl.appName, bucket, path)).Send()
	if err != nil {
		return nil, res.Header.Get(clients.HeaderETag), err
	}

	return res, res.Header.Get(clients.HeaderETag), nil
}

// GetFileConflict gets a file's content as a byte slice, or conflict
func (cl *Client) GetFileConflict(bucket, path string) (*gentleman.Response, *Conflict, string, error) {
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
func (cl *Client) SaveJSON(bucket, path string, data interface{}) (string, error) {
	res, err := cl.http.Put().
		AddPath(fmt.Sprintf(pathToFile, cl.appName, bucket, path)).
		JSON(data).Send()

	if err != nil {
		return "", err
	}

	return res.Header.Get(clients.HeaderETag), nil
}

// SaveFile saves a file to a workspace
func (cl *Client) SaveFile(bucket, path string, body io.Reader) (string, error) {
	_, err := cl.http.Put().
		AddPath(fmt.Sprintf(pathToFile, cl.appName, bucket, path)).
		Body(body).Send()

	return "", err
}

// SaveFileB saves a file to a workspace
func (cl *Client) SaveFileB(bucket, path string, body []byte, contentType string, unzip bool) (string, error) {
	res, err := cl.http.Put().
		AddPath(fmt.Sprintf(pathToFile, cl.appName, bucket, path)).
		SetQuery("unzip", fmt.Sprintf("%v", unzip)).
		Body(bytes.NewReader(body)).Send()

	if err != nil {
		return "", err
	}

	return res.Header.Get(clients.HeaderETag), nil
}

// ListFiles returns a list of files, given a prefix
func (cl *Client) ListFiles(bucket string, options *Options) (*FileListResponse, string, error) {
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
func (cl *Client) ListAllFiles(bucket, prefix string) (*FileListResponse, string, error) {
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
func (cl *Client) DeleteFile(bucket, path string) error {
	_, err := cl.http.Delete().
		AddPath(fmt.Sprintf(pathToFile, cl.appName, bucket, path)).
		Send()

	return err
}

// DeleteAllFiles deletes all files from the specificed bucket
func (cl *Client) DeleteAllFiles(bucket string) error {
	_, err := cl.http.Delete().
		AddPath(fmt.Sprintf(pathToFileList, cl.appName, bucket)).
		Send()

	return err
}
