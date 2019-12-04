package vbase

import (
	"fmt"

	"time"

	"github.com/vtex/go-clients/clients"
	gentleman "gopkg.in/h2non/gentleman.v1"
)

// VBaseChronos is an interface for interacting with VBase using Chronos as storage
type VBaseChronos interface {
	GetJSON(bucket, path string, date *time.Time, data interface{}) (eTag string, err error)

	SaveJSON(bucket, path string, data interface{}) (string, error)
	DeleteFile(bucket, path string) error
}

type clientChronos struct {
	http               *gentleman.Client
	appName            string
	workspace          string
	resolvingConflicts bool
}

func NewCustomAppClientChronos(appName string, config *clients.Config) VBaseChronos {
	cl := clients.CreateClient("vbase", config, true)
	return &clientChronos{cl, appName, config.Workspace, false}
}

const (
	pathToFileChronos = "/buckets/%v/%v/config/files/%v"
)

// GetJSON populates data with the content of the specified file, assuming it is serialized as JSON
func (cl *clientChronos) GetJSON(bucket, path string, date *time.Time, data interface{}) (string, error) {
	res, _, err := cl.getFileInternal(bucket, path, date)
	if err != nil {
		return "", err
	}
	defer res.Close()

	if err := res.JSON(data); err != nil {
		return "", err
	}

	return res.Header.Get(clients.HeaderETag), nil
}

func (cl *clientChronos) getFileInternal(bucket, path string, date *time.Time) (*gentleman.Response, string, error) {
	req := cl.http.Get().
		AddPath(fmt.Sprintf(pathToFileChronos, cl.appName, bucket, path))

	if date != nil {
		req = req.SetQuery("atDate", date.Format(time.RFC3339))
	}

	res, err := req.Send()

	if err != nil {
		return nil, res.Header.Get(HeaderContentType), err
	}

	return res, res.Header.Get(HeaderContentType), nil
}

// SaveJSON saves generic data serializing it to JSON in Chronos
func (cl *clientChronos) SaveJSON(bucket, path string, data interface{}) (string, error) {
	res, err := cl.http.Put().
		AddPath(fmt.Sprintf(pathToFileChronos, cl.appName, bucket, path)).
		JSON(data).
		Send()

	if err != nil {
		return "", err
	}

	return res.Header.Get(clients.HeaderETag), nil
}

// DeleteFile deletes a file from the workspace
func (cl *clientChronos) DeleteFile(bucket, path string) error {
	_, err := cl.http.Delete().
		AddPath(fmt.Sprintf(pathToFile, cl.appName, bucket, path)).
		Send()

	return err
}
