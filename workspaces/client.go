package workspaces

import (
	"fmt"

	"github.com/vtex/go-clients/clients"
	"gopkg.in/h2non/gentleman.v1"
)

type Workspaces interface {
	List() ([]*Workspace, error)
	Get(name string) (*Workspace, error)
	Create(name string) error
	Delete(name string) error
}

type Client struct {
	account string
	http    *gentleman.Client
}

func NewClient(config *clients.Config) Workspaces {
	cl := clients.CreatePlatformClient(config)
	return &Client{config.Account, cl}
}

const (
	accountPath   = "/%v"
	workspacePath = "/%v/%v"
)

func (cl *Client) List() ([]*Workspace, error) {
	res, err := cl.http.Get().AddPath(fmt.Sprintf(accountPath, cl.account)).Send()
	if err != nil {
		return nil, err
	}

	var workspaces []*Workspace
	if err := res.JSON(&workspaces); err != nil {
		return nil, err
	}

	return workspaces, nil
}

func (cl *Client) Get(name string) (*Workspace, error) {
	res, err := cl.http.Get().AddPath(fmt.Sprintf(workspacePath, cl.account, name)).Send()
	if err != nil {
		return nil, err
	}

	var workspace Workspace
	if err := res.JSON(&workspace); err != nil {
		return nil, err
	}

	return &workspace, nil
}

func (cl *Client) Create(name string) error {
	_, err := cl.http.Post().AddPath(fmt.Sprintf(accountPath, cl.account)).Send()
	return err
}

func (cl *Client) Delete(name string) error {
	_, err := cl.http.Delete().AddPath(fmt.Sprintf(workspacePath, cl.account, name)).Send()
	return err
}
