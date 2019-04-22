package apps

import (
	"fmt"
	"io"
	"strings"

	"github.com/vtex/go-clients/clients"
	"gopkg.in/h2non/gentleman.v1"
)

// Registry is an interface for interacting with the registry
type Registry interface {
	GetApp(id string) (*PublishedApp, string, error)
	ListFiles(id string) (*FileList, string, error)
	GetFile(id, path string) (io.ReadCloser, string, error)
	GetBundle(id, rootFolder string) (io.ReadCloser, string, error)
}

// Client is a struct that provides interaction with apps
type RegistryClient struct {
	http *gentleman.Client
}

// NewClient creates a new Registry client
func NewRegistryClient(config *clients.Config) Registry {
	if config.Workspace != clients.MasterWorkspace {
		configCopy := *config
		configCopy.Workspace = clients.MasterWorkspace
		config = &configCopy
	}
	cl := clients.CreateClient("apps", config, true)
	return &RegistryClient{cl}
}

type RegistryBaseClient struct {
	WithContext func(ctx clients.IORequestContext) Registry
}

func NewRegistryBaseClient(opts clients.IOClientOptions) *RegistryBaseClient {
	base := clients.CreateBaseInfraClient("apps", opts)
	return &RegistryBaseClient{
		WithContext: func(ctx clients.IORequestContext) Registry {
			ctx.Workspace = clients.MasterWorkspace
			return &RegistryClient{clients.WithContext(base, ctx)}
		},
	}
}

const (
	metadataPath    = "/registry/%v/%v"
	fileListPath    = "/registry/%v/%v/files"
	fileContentPath = "/registry/%v/%v/files/%v"
	bundlePath      = "/registry/%v/%v/bundle/%v"
)

// GetApp returns the app metadata
func (cl *RegistryClient) GetApp(id string) (*PublishedApp, string, error) {
	name, version, err := getNameVersion(id)
	if err != nil {
		return nil, "", err
	}

	res, err := cl.http.Get().
		AddPath(fmt.Sprintf(metadataPath, name, version)).
		Send()
	if err != nil {
		return nil, "", err
	}

	var m PublishedApp
	if err := res.JSON(&m); err != nil {
		return nil, "", err
	}

	return &m, res.Header.Get(clients.HeaderETag), nil
}

func (cl *RegistryClient) ListFiles(id string) (*FileList, string, error) {
	name, version, err := getNameVersion(id)
	if err != nil {
		return nil, "", err
	}

	res, err := cl.http.Get().
		AddPath(fmt.Sprintf(fileListPath, name, version)).
		Send()
	if err != nil {
		return nil, "", err
	}

	var l FileList
	if err := res.JSON(&l); err != nil {
		return nil, "", err
	}

	return &l, res.Header.Get(clients.HeaderETag), nil
}

func (cl *RegistryClient) GetFile(id, path string) (io.ReadCloser, string, error) {
	name, version, err := getNameVersion(id)
	if err != nil {
		return nil, "", err
	}

	res, err := cl.http.Get().
		AddPath(fmt.Sprintf(fileContentPath, name, version, path)).
		Send()
	if err != nil {
		return nil, "", err
	}

	return res, res.Header.Get(clients.HeaderETag), nil
}

func (cl *RegistryClient) GetBundle(id, rootFolder string) (io.ReadCloser, string, error) {
	name, version, err := getNameVersion(id)
	if err != nil {
		return nil, "", err
	}

	res, err := cl.http.Get().
		AddPath(fmt.Sprintf(bundlePath, name, version, rootFolder)).
		Send()
	if err != nil {
		return nil, "", err
	}

	return res, res.Header.Get(clients.HeaderETag), nil
}

func getNameVersion(id string) (string, string, error) {
	segments := strings.SplitN(id, "@", 2)
	if len(segments) != 2 {
		return "", "", fmt.Errorf("Not a composed app identifier: %s", id)
	}
	return segments[0], segments[1], nil
}
