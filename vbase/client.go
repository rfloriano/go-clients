package vbase

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"strconv"

	"github.com/pkg/errors"
	"github.com/vtex/go-clients/clients"
	gentleman "gopkg.in/h2non/gentleman.v1"
	"gopkg.in/h2non/gentleman.v1/context"
	"gopkg.in/h2non/gentleman.v1/plugin"
)

type Options struct {
	Prefix string
	Marker string
	Limit  int
}

type SaveFileOptions struct {
	ContentType     string
	Unzip           bool
	IgnoreConflicts bool
}

// VBase is an interface for interacting with VBase
type VBase interface {
	GetFile(bucket, path string) (*gentleman.Response, string, error)
	GetJSON(bucket, path string, data interface{}) (string, error)
	ListFiles(bucket string, options *Options) (*FileListResponse, string, error)
	ListAllFiles(bucket, prefix string) (*FileListResponse, string, error)

	SaveFile(bucket, path string, body io.Reader, opts SaveFileOptions) (string, error)
	SaveFileB(bucket, path string, content []byte, opts SaveFileOptions) (string, error)
	SaveJSON(bucket, path string, data interface{}) (string, error)
	DeleteFile(bucket, path string) error
	DeleteAllFiles(bucket string) error

	GetBucket(bucket string) (*BucketResponse, string, error)

	ListAllConflicts(bucket string) ([]*Conflict, error)
	ResolveConflicts(bucket string, patch PatchRequest) error
}

type ConflictResolver interface {
	Resolve(client VBase, bucket string) (resolved bool, err error)
}

type client struct {
	http               *gentleman.Client
	appName            string
	workspace          string
	conflictResolver   ConflictResolver
	resolvingConflicts bool
}

// NewClient creates a new Workspaces client
func NewClient(config *clients.Config, cResolver ConflictResolver) (VBase, error) {
	cl := clients.CreateClient("vbase", config, true)
	appName := clients.UserAgentName(config)
	if appName == "" {
		return nil, clients.NewNoUserAgentError("User-Agent is missing to create a VBase client.")
	}
	return &client{cl, appName, config.Workspace, cResolver, false}, nil
}

const (
	pathToBucket    = "/buckets/%v/%v"
	pathToFileList  = "/buckets/%v/%v/files"
	pathToFile      = "/buckets/%v/%v/files/%v"
	pathToConflicts = "/buckets/%v/%v/conflicts"
)

// GetBucket describes the current state of a bucket
func (cl *client) GetBucket(bucket string) (*BucketResponse, string, error) {
	res, err := cl.http.Get().
		AddPath(fmt.Sprintf(pathToBucket, cl.appName, bucket)).
		Use(cl.conflictHandler(bucket)).
		Send()
	if err != nil {
		return nil, "", err
	}

	var bucketResponse BucketResponse
	if err := res.JSON(&bucketResponse); err != nil {
		return nil, "", err
	}

	return &bucketResponse, res.Header.Get(clients.HeaderETag), nil
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

// GetFile gets a file's content as a read closer
func (cl *client) GetFile(bucket, path string) (*gentleman.Response, string, error) {
	res, err := cl.http.Get().
		AddPath(fmt.Sprintf(pathToFile, cl.appName, bucket, path)).
		Use(cl.conflictHandler(bucket)).
		Send()
	if err != nil {
		return nil, res.Header.Get(clients.HeaderETag), err
	}

	return res, res.Header.Get(clients.HeaderETag), nil
}

// SaveJSON saves generic data serializing it to JSON
func (cl *client) SaveJSON(bucket, path string, data interface{}) (string, error) {
	res, err := cl.http.Put().
		AddPath(fmt.Sprintf(pathToFile, cl.appName, bucket, path)).
		JSON(data).
		Use(cl.conflictHandler(bucket)).
		Send()

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

	if !opts.IgnoreConflicts {
		req = req.Use(cl.conflictHandler(bucket))
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
		}).
		Use(cl.conflictHandler(bucket)).
		Send()

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

func (cl *client) ListAllConflicts(bucket string) ([]*Conflict, error) {
	res, err := cl.http.Get().
		AddPath(fmt.Sprintf(pathToConflicts, cl.appName, bucket)).
		Send() // No conflict handler plugin for getting conflicts
	if err != nil {
		return nil, err
	}

	var response ConflictListResponse
	if err := res.JSON(&response); err != nil {
		return nil, fmt.Errorf("Error unmarshaling conflicts list: %v", err)
	}

	return response.Data, nil
}

func (cl *client) ResolveConflicts(bucket string, patch PatchRequest) error {
	_, err := cl.http.Patch().
		AddPath(fmt.Sprintf(pathToConflicts, cl.appName, bucket)).
		JSON(patch).
		Send() // No conflict handler plugin for resolving conflicts
	return err
}

// DeleteFile deletes a file from the workspace
func (cl *client) DeleteFile(bucket, path string) error {
	_, err := cl.http.Delete().
		AddPath(fmt.Sprintf(pathToFile, cl.appName, bucket, path)).
		Use(cl.conflictHandler(bucket)).
		Send()

	return err
}

// DeleteAllFiles deletes all files from the specificed bucket
func (cl *client) DeleteAllFiles(bucket string) error {
	_, err := cl.http.Delete().
		AddPath(fmt.Sprintf(pathToFileList, cl.appName, bucket)).
		Use(cl.conflictHandler(bucket)).
		Send()

	return err
}

func (cl *client) conflictHandler(bucket string) plugin.Plugin {
	p := plugin.New()
	if cl.conflictResolver == nil || cl.resolvingConflicts || cl.workspace == clients.MasterWorkspace {
		return p
	}

	var reqCopy *http.Request
	p.SetHandlers(plugin.Handlers{
		"request": func(c *context.Context, h context.Handler) {
			c.Request.Header.Set("X-Vtex-Detect-Conflicts", "true")

			var err error
			reqCopy, err = copyRequest(c.Request)
			if err != nil {
				h.Error(c, err)
				return
			}

			h.Next(c)
		},
		"after dial": func(c *context.Context, h context.Handler) {
			if c.Response.StatusCode == http.StatusConflict {
				if err := cl.resolveConflicts(bucket); err != nil {
					h.Error(c, err)
					return
				}

				// Retry
				res, err := retryRequest(c.Client, reqCopy, bucket)
				if res != nil {
					c.Response = res
				}
				if err != nil {
					h.Error(c, err)
					return
				}
				res.Header.Set("X-Vtex-Solved-Conflicts", bucket)
			}

			h.Next(c)
		},
	})
	return p
}

func copyRequest(req *http.Request) (*http.Request, error) {
	reqCopy := &http.Request{}
	*reqCopy = *req

	if req.Body == nil {
		return reqCopy, nil
	}

	buf, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	req.Body = ioutil.NopCloser(bytes.NewReader(buf))
	reqCopy.Body = ioutil.NopCloser(bytes.NewReader(buf))

	return reqCopy, nil
}

func (cl *client) resolveConflicts(bucket string) error {
	clCopy := *cl
	clCopy.resolvingConflicts = true

	resolved, err := cl.conflictResolver.Resolve(&clCopy, bucket)
	if err != nil {
		return errors.Wrapf(err, "Error resolving conflicts in bucket %s", bucket)
	} else if !resolved {
		return fmt.Errorf("Conflicts could not be solved in bucket %s", bucket)
	}
	return nil
}

func retryRequest(client *http.Client, req *http.Request, bucket string) (*http.Response, error) {
	res, err := client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "Error retrying request after conflicts resolution")
	}
	if res == nil {
		return nil, errors.Errorf("Nil response retrying request after conflicts resolution")
	}

	if res.StatusCode == http.StatusConflict {
		return res, errors.Errorf("Bucket %s still has conflicts after resolution", bucket)
	}
	return res, nil
}
