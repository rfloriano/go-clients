package vbasemetadata

import (
	"github.com/vtex/go-clients/clients"
	"github.com/vtex/go-clients/metadata"
	"github.com/vtex/go-clients/vbase"
)

// VBase is a limited interface for using VBase with Metadata fallback
type VBase interface {
	GetJSON(bucket, key string, data interface{}, metadataBucket string) (string, error)
	SaveJSON(bucket, key string, data interface{}) (string, error)
}

type client struct {
	vbase    vbase.VBase
	metadata metadata.Metadata
}

// NewClient creates a new Workspaces client
func NewClient(config *clients.Config) (VBase, error) {
	vbaseClient, err := vbase.NewClient(config)
	if err != nil {
		return nil, err
	}

	metadataClient, err := metadata.NewClient(config, nil)
	if err != nil {
		return nil, err
	}

	return &client{vbaseClient, metadataClient}, nil
}

func (cl *client) GetJSON(bucket, path string, data interface{}, metadataBucket string) (string, error) {
	etag, err := cl.vbase.GetJSON(bucket, path, data)
	if err != nil {
		etag, err = cl.metadata.Get(metadataBucket, path, data)
		if err == nil {
			etag, err = cl.vbase.SaveJSON(bucket, path, data)
		}
	}
	return etag, err
}

func (cl *client) SaveJSON(bucket, path string, data interface{}) (string, error) {
	return cl.vbase.SaveJSON(bucket, path, data)
}
