package workspaces

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/vtex/go-clients/errors"
)

var hcli = &http.Client{
	Timeout: time.Second * 10,
}

// Workspaces is an interface for interacting with workspaces
type Workspaces interface {
	GetBucket(account, workspace, bucket string) (*BucketResponse, error)
	GetFile(account, workspace, bucket, path string) (io.ReadCloser, error)
	GetFileB(account, workspace, bucket, path string) ([]byte, error)
	SaveFile(account, workspace, bucket, path string, body io.Reader) error
	SaveFileB(account, workspace, bucket, path string, content []byte) error
}

// Client is a struct that provides interaction with workspaces
type Client struct {
	Endpoint  string
	AuthToken string
	UserAgent string
}

// NewClient creates a new Workspaces client
func NewClient(endpoint, authToken, userAgent string) Workspaces {
	return &Client{Endpoint: endpoint, AuthToken: authToken, UserAgent: userAgent}
}

const (
	pathToBucket = "%v/%v/buckets/%v"
	pathToFile   = "%v/%v/buckets/%v/files/%v"
)

func (cl *Client) createRequestB(method string, content []byte, pathFormat string, a ...interface{}) *http.Request {
	var body io.Reader
	if content != nil {
		body = bytes.NewBuffer(content)
	}
	return cl.createRequest(method, body, pathFormat, a...)
}

func (cl *Client) createRequest(method string, body io.Reader, pathFormat string, a ...interface{}) *http.Request {
	req, err := http.NewRequest("GET", fmt.Sprintf(cl.Endpoint+pathFormat, a...), body)
	if err != nil {
		panic(err)
	}

	req.Header.Set("Authorization", "token "+cl.AuthToken)
	req.Header.Set("User-Agent", cl.UserAgent)
	return req
}

// GetBucket describes the current state of a bucket
func (cl *Client) GetBucket(account, workspace, bucket string) (*BucketResponse, error) {
	req := cl.createRequest("GET", nil, pathToBucket, account, bucket, workspace)
	res, reserr := hcli.Do(req)
	if reserr != nil {
		return nil, reserr
	}
	if err := errors.StatusCode(res); err != nil {
		return nil, err
	}

	var bucketResponse BucketResponse
	buf, buferr := ioutil.ReadAll(res.Body)
	if buferr != nil {
		return nil, buferr
	}
	json.Unmarshal(buf, &bucketResponse)
	return &bucketResponse, nil
}

// GetFile gets a file's content as a read closer
func (cl *Client) GetFile(account, workspace, bucket, path string) (io.ReadCloser, error) {
	req := cl.createRequest("GET", nil, pathToFile, account, workspace, bucket, path)
	res, reserr := hcli.Do(req)
	if reserr != nil {
		return nil, reserr
	}
	if err := errors.StatusCode(res); err != nil {
		return nil, err
	}

	return res.Body, nil
}

// GetFileB gets a file's content as bytes
func (cl *Client) GetFileB(account, workspace, bucket, path string) ([]byte, error) {
	body, err := cl.GetFile(account, workspace, bucket, path)
	if err != nil {
		return nil, err
	}

	buf, buferr := ioutil.ReadAll(body)
	if buferr != nil {
		return nil, buferr
	}
	return buf, nil
}

// SaveFile saves a file to a workspace
func (cl *Client) SaveFile(account, workspace, bucket, path string, body io.Reader) error {
	req := cl.createRequest("PUT", body, pathToFile, account, workspace, bucket, path)
	res, reserr := hcli.Do(req)
	if reserr != nil {
		return reserr
	}
	if err := errors.StatusCode(res); err != nil {
		return err
	}
	return nil
}

// SaveFileB saves a file to a workspace
func (cl *Client) SaveFileB(account, workspace, bucket, path string, body []byte) error {
	req := cl.createRequestB("PUT", body, pathToFile, account, workspace, bucket, path)
	res, reserr := hcli.Do(req)
	if reserr != nil {
		return reserr
	}
	if err := errors.StatusCode(res); err != nil {
		return err
	}
	return nil
}
