package sphinx

import (
	"fmt"

	"github.com/vtex/go-clients/clients"
	"github.com/vtex/go-clients/common"

	"gopkg.in/h2non/gentleman.v1"
)

type Sphinx interface {
	GetResourcePolicies(service string) (*ResourcePolicies, error)
	GetRolePolicies(role string) (*common.Policies, error)
}

const (
	resourcePoliciesPath = "/service/%v/resourcePolicies"
	rolePoliciesPath     = "/v2/roles/%v/policies"
)

type Client struct {
	http *gentleman.Client
}

func NewSphinxClient(config *clients.Config) Sphinx {
	cl := clients.CreateClient("sphinx", config, true)
	return &Client{cl}
}

func (cl *Client) GetResourcePolicies(service string) (*ResourcePolicies, error) {
	res, err := cl.http.Get().AddPath(fmt.Sprintf(resourcePoliciesPath, service)).Send()
	if err != nil {
		return nil, err
	}

	var resourcePolicies ResourcePolicies
	if err := res.JSON(&resourcePolicies); err != nil {
		return nil, err
	}

	return &resourcePolicies, nil
}

func (cl *Client) GetRolePolicies(role string) (*common.Policies, error) {
	res, err := cl.http.Get().AddPath(fmt.Sprintf(rolePoliciesPath, role)).Send()
	if err != nil {
		return nil, err
	}

	var policies common.Policies
	if err := res.JSON(&policies); err != nil {
		return nil, err
	}

	return &policies, nil
}
