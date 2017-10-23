package colossus

import (
	"bytes"
	"fmt"
	"net/http"

	"github.com/vtex/go-clients/clients"
	"gopkg.in/h2non/gentleman.v1"
)

type Colossus interface {
	SendEventJ(subject, key string, body interface{}) error
	SendEventB(subject, key string, body []byte) error
	SendLogJ(subject, level string, body interface{}) error
	SendLogB(subject, level string, body []byte) error
	SendEvent(subject, key string, body interface{}, extraHeaders http.Header) error
}

type Client struct {
	http *gentleman.Client
}

func NewClient(config *clients.Config) Colossus {
	cl := clients.CreateClient("colossus", config, true)
	return &Client{cl}
}

const (
	eventPath = "/events/%v?subject=%v"
	logPath   = "/logs/%v?subject=%v"
)

func (cl *Client) SendEventJ(subject, key string, body interface{}) error {
	return cl.SendEvent(subject, key, body, nil)
}

func (cl *Client) SendEvent(subject, key string, body interface{}, extraHeaders http.Header) error {
	request := cl.http.Put().AddPath(fmt.Sprintf(eventPath, key, subject)).JSON(body)
	for k, vs := range extraHeaders {
		for _, v := range vs {
			request.AddHeader(k, v)
		}
	}
	_, err := request.Send()

	return err
}

func (cl *Client) SendEventB(subject, key string, body []byte) error {
	_, err := cl.http.Put().
		AddPath(fmt.Sprintf(eventPath, key, subject)).
		Body(bytes.NewReader(body)).Send()

	return err
}

func (cl *Client) SendLogJ(subject, level string, body interface{}) error {
	_, err := cl.http.Put().
		AddPath(fmt.Sprintf(logPath, level, subject)).
		JSON(body).Send()

	return err
}

func (cl *Client) SendLogB(subject, level string, body []byte) error {
	_, err := cl.http.Put().
		AddPath(fmt.Sprintf(logPath, level, subject)).
		Body(bytes.NewReader(body)).Send()

	return err
}
