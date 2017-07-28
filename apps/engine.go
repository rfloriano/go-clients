package apps

import (
	"fmt"
	"io"

	"github.com/vtex/go-clients/clients"
	"gopkg.in/h2non/gentleman.v1"
	"gopkg.in/h2non/gentleman.v1/context"
	"gopkg.in/h2non/gentleman.v1/plugin"
)

// Apps is an interface for interacting with apps
type Apps interface {
	GetApp(app, parentID string) (*ActiveApp, string, error)
	ListFiles(app, parentID string) (*FileList, string, error)
	GetFile(app, parentID, path string) (io.ReadCloser, string, error)
	GetBundle(app, parentID, rootFolder string) (io.ReadCloser, string, error)
	GetDependencies() (map[string][]string, string, error)
}

// Client is a struct that provides interaction with apps
type AppsClient struct {
	http *gentleman.Client
}

// NewClient creates a new Apps client
func NewAppsClient(config *clients.Config) Apps {
	cl := clients.CreateClient("apps", config, true)
	return &AppsClient{cl}
}

const (
	pathToDependencies = "/dependencies"
	pathToApp          = "/apps/%v"
	pathToFiles        = "/apps/%v/files"
	pathToFile         = "/apps/%v/files/%v"
	pathToBundle       = "/apps/%v/bundle/%v"
)

// GetApp describes an installed app's manifest
func (cl *AppsClient) GetApp(app, parentID string) (*ActiveApp, string, error) {
	res, err := cl.http.Get().
		AddPath(fmt.Sprintf(pathToApp, app)).
		Use(addParent(parentID)).
		Send()
	if err != nil {
		return nil, "", err
	}

	var manifest ActiveApp
	if err := res.JSON(&manifest); err != nil {
		return nil, "", err
	}

	return &manifest, res.Header.Get(clients.HeaderETag), nil
}

func (cl *AppsClient) ListFiles(app, parentID string) (*FileList, string, error) {
	res, err := cl.http.Get().
		AddPath(fmt.Sprintf(pathToFiles, app)).
		Use(addParent(parentID)).
		Send()
	if err != nil {
		return nil, "", err
	}

	var files FileList
	if err := res.JSON(&files); err != nil {
		return nil, "", err
	}

	return &files, res.Header.Get(clients.HeaderETag), nil
}

// GetFile gets an installed app's file as read closer
func (cl *AppsClient) GetFile(app, parentID string, path string) (io.ReadCloser, string, error) {
	res, err := cl.http.Get().
		AddPath(fmt.Sprintf(pathToFile, app, path)).
		Use(addParent(parentID)).
		Send()
	if err != nil {
		return nil, "", err
	}

	return res, res.Header.Get(clients.HeaderETag), nil
}

func (cl *AppsClient) GetBundle(app, parentID, rootFolder string) (io.ReadCloser, string, error) {
	res, err := cl.http.Get().
		AddPath(fmt.Sprintf(pathToBundle, app, rootFolder)).
		Use(addParent(parentID)).
		Send()
	if err != nil {
		return nil, "", err
	}

	return res, res.Header.Get(clients.HeaderETag), nil
}

func (cl *AppsClient) GetDependencies() (map[string][]string, string, error) {
	res, err := cl.http.Get().
		AddPath(pathToDependencies).
		Send()
	if err != nil {
		return nil, "", err
	}

	var dependencies map[string][]string
	if err := res.JSON(&dependencies); err != nil {
		return nil, "", err
	}

	return dependencies, res.Header.Get(clients.HeaderETag), err
}

func addParent(parentID string) plugin.Plugin {
	return plugin.NewRequestPlugin(func(ctx *context.Context, h context.Handler) {
		if parentID != "" {
			query := ctx.Request.URL.Query()
			query.Set("parent", parentID)
			ctx.Request.URL.RawQuery = query.Encode()
		}
		h.Next(ctx)
	})
}
