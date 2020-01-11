package auth

import (
	"fmt"

	"github.com/vtex/go-clients/clients"
	"github.com/vtex/go-clients/common"

	"gopkg.in/h2non/gentleman.v1"
)

type AuthEngine interface {
	GetAllowedActions(resource string, context map[string][]string, policies []common.Policy) (*Permissions, error)
}

const (
	authEnginePath = "/api/engine/evaluate"
)

type Client struct {
	http *gentleman.Client
}

func NewAuthEngineClient(config *clients.Config) AuthEngine {
	cl := clients.CreateGenericClient(fmt.Sprintf("http://authorization-engine.vtex.com"), config, false)
	return &Client{cl}
}

func (cl *Client) GetAllowedActions(resource string, context map[string][]string, policies []common.Policy) (*Permissions, error) {
	body := Body{
		Resource: resource,
		Context:  context,
		Policies: policies,
	}

	res, err := cl.http.Post().AddPath(authEnginePath).JSON(body).Send()
	if err != nil {
		return nil, err
	}

	var permissions Permissions
	if err := res.JSON(&permissions); err != nil {
		return nil, err
	}

	return &permissions, nil
}
