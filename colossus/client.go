package colossus

import (
	"bytes"
	"fmt"
	"net/http"

	"github.com/vtex/go-clients/clients"
	"gopkg.in/h2non/gentleman.v1"
)

type Colossus interface {
	SendEventJ(sender, subject, key string, body interface{}) error
	SendEventB(sender, subject, key string, body []byte) error
	SendLogJ(sender, subject, level string, body interface{}) error
	SendLogB(sender, subject, level string, body []byte) error
	SendEvent(sender, subject, key string, body interface{}, extraHeaders http.Header) error
}

type Client struct {
	http *gentleman.Client
}

func NewClient(config *clients.Config) Colossus {
	cl := clients.CreateClient("colossus", config, true)
	return &Client{cl}
}

const (
	eventPath = "/events/%v/%v/%v"
	logPath   = "/logs/%v/%v/%v"
)

func (cl *Client) SendEventJ(sender, subject, key string, body interface{}) error {
	return cl.SendEvent(sender, subject, key, body, nil)
}

func (cl *Client) SendEvent(sender, subject, key string, body interface{}, extraHeaders http.Header) error {
	request := cl.http.Post().AddPath(fmt.Sprintf(eventPath, sender, subject, key)).JSON(body)
	for k, vs := range extraHeaders {
		for _, v := range vs {
			request.AddHeader(k, v)
		}
	}
	_, err := request.Send()

	return err
}

func (cl *Client) SendEventB(sender, subject, key string, body []byte) error {
	_, err := cl.http.Post().
		AddPath(fmt.Sprintf(eventPath, sender, subject, key)).
		Body(bytes.NewReader(body)).Send()

	return err
}

func (cl *Client) SendLogJ(sender, subject, level string, body interface{}) error {
	_, err := cl.http.Post().
		AddPath(fmt.Sprintf(logPath, sender, subject, level)).
		JSON(body).Send()

	return err
}

func (cl *Client) SendLogB(sender, subject, level string, body []byte) error {
	_, err := cl.http.Post().
		AddPath(fmt.Sprintf(logPath, sender, subject, level)).
		Body(bytes.NewReader(body)).Send()

	return err
}
