package apps

import (
	"encoding/json"
	"fmt"
	"github.com/vtex/go-clients/clients"
	"gopkg.in/h2non/gentleman.v1"
	"io"
	"strings"
)

// Apps is an interface for interacting with apps
type Apps interface {
	GetApp(account, workspace, app string, context []string) (*ActiveApp, error)
	ListFiles(account, workspace, app string, context []string) (*FileList, error)
	GetFile(account, workspace, app string, context []string, path string) (io.ReadCloser, error)
	GetFileB(account, workspace, app string, context []string, path string) ([]byte, error)
	GetFileJ(account, workspace, app string, context []string, path string, dest interface{}) error
	GetDependencies(account, workspace string) (map[string][]string, error)
}

// Client is a struct that provides interaction with apps
type AppsClient struct {
	http  *gentleman.Client
	cache clients.ValueCache
}

// NewClient creates a new Apps client
func NewAppsClient(endpoint, authToken, userAgent string, reqCtx clients.RequestContext) Apps {
	cl, vc := clients.CreateClient(endpoint, authToken, userAgent, reqCtx)
	return &AppsClient{cl, vc}
}

const (
	pathToDependencies = "/%v/%v/dependencies"
	pathToApp          = "/%v/%v/apps/%v"
	pathToFiles        = "/%v/%v/apps/%v/files"
	pathToFile         = "/%v/%v/apps/%v/files/%v"
)

// GetApp describes an installed app's manifest
func (cl *AppsClient) GetApp(account, workspace, app string, context []string) (*ActiveApp, error) {
	const kind = "manifest"
	res, err := cl.http.Get().AddPath(fmt.Sprintf(pathToApp, account, workspace, app)).
		UseRequest(clients.Cache).
		SetQuery("context", strings.Join(context, "/")).Send()
	if err != nil {
		return nil, err
	}

	if cached, ok, err := cl.cache.GetFor(kind, res); err != nil {
		return nil, err
	} else if ok {
		return cached.(*ActiveApp), nil
	}

	var manifest ActiveApp
	if err := res.JSON(&manifest); err != nil {
		return nil, err
	}

	cl.cache.SetFor(kind, res, &manifest)

	return &manifest, nil
}

func (cl *AppsClient) ListFiles(account, workspace, app string, context []string) (*FileList, error) {
	const kind = "file-list"
	res, err := cl.http.Get().AddPath(fmt.Sprintf(pathToFiles, account, workspace, app)).
		UseRequest(clients.Cache).
		SetQuery("context", strings.Join(context, "/")).Send()
	if err != nil {
		return nil, err
	}

	if cached, ok, err := cl.cache.GetFor(kind, res); err != nil {
		return nil, err
	} else if ok {
		return cached.(*FileList), nil
	}

	var files FileList
	if err := res.JSON(&files); err != nil {
		return nil, err
	}

	cl.cache.SetFor(kind, res, &files)

	return &files, nil
}

func (cl *AppsClient) getFile(account, workspace, app string, context []string, path string, useCache bool) (io.ReadCloser, error) {
	req := cl.http.Get().AddPath(fmt.Sprintf(pathToFile, account, workspace, app, path)).
		SetQuery("context", strings.Join(context, "/"))
	if useCache {
		req.UseRequest(clients.Cache)
	}

	return req.Send()
}

// GetFile gets an installed app's file as read closer
func (cl *AppsClient) GetFile(account, workspace, app string, context []string, path string) (io.ReadCloser, error) {
	res, err := cl.getFile(account, workspace, app, context, path, false)
	if err != nil {
		return nil, err
	}

	return res, nil
}

// GetFileB gets an installed app's file as bytes
func (cl *AppsClient) GetFileB(account, workspace, app string, context []string, path string) ([]byte, error) {
	const kind = "file-bytes"
	res, err := cl.getFile(account, workspace, app, context, path, true)
	if err != nil {
		return nil, err
	}

	gentRes := res.(*gentleman.Response)
	if cached, ok, err := cl.cache.GetFor(kind, gentRes); err != nil {
		return nil, err
	} else if ok {
		return cached.([]byte), nil
	}

	bytes := gentRes.Bytes()
	cl.cache.SetFor(kind, gentRes, bytes)

	return bytes, nil
}

// GetFileJ gets an installed app's file as deserialized JSON object
func (cl *AppsClient) GetFileJ(account, workspace, app string, context []string, path string, dest interface{}) error {
	b, err := cl.GetFileB(account, workspace, app, context, path)
	if err != nil {
		return err
	}

	return json.Unmarshal(b, dest)
}

func (cl *AppsClient) GetDependencies(account, workspace string) (map[string][]string, error) {
	const kind = "dependencies"
	res, err := cl.http.Get().AddPath(fmt.Sprintf(pathToDependencies, account, workspace)).
		UseRequest(clients.Cache).Send()
	if err != nil {
		return nil, err
	}

	if cached, ok, err := cl.cache.GetFor(kind, res); err != nil {
		return nil, err
	} else if ok {
		return cached.(map[string][]string), nil
	}

	var dependencies map[string][]string
	if err := res.JSON(&dependencies); err != nil {
		return nil, err
	}

	cl.cache.SetFor(kind, res, dependencies)
	return dependencies, err
}