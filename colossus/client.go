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
	SendLog(subject, key string, body interface{}, extraHeaders http.Header) error
}

type Client struct {
	http *gentleman.Client
}

func NewClient(config *clients.Config) Colossus {
	cl := clients.CreateClient("colossus", config, true)
	return &Client{cl}
}

const (
	eventPath = "/events/%v"
	logPath   = "/logs/%v"
)

func (cl *Client) SendEventJ(subject, key string, body interface{}) error {
	return cl.SendEvent(subject, key, body, nil)
}

func (cl *Client) SendEvent(subject, key string, body interface{}, extraHeaders http.Header) error {
	request := cl.http.Put().AddPath(fmt.Sprintf(eventPath, key)).
		AddQuery("subject", subject).JSON(body)
	addHeadersToRequest(request, extraHeaders)
	_, err := request.Send()

	return err
}

func (cl *Client) SendEventB(subject, key string, body []byte) error {
	_, err := cl.http.Put().
		AddPath(fmt.Sprintf(eventPath, key)).
		AddQuery("subject", subject).
		Body(bytes.NewReader(body)).Send()

	return err
}

func (cl *Client) SendLogJ(subject, level string, body interface{}) error {
	return cl.SendLog(subject, level, body, nil)
}

func (cl *Client) SendLog(subject, level string, body interface{}, extraHeaders http.Header) error {
	request := cl.http.Put().AddPath(fmt.Sprintf(logPath, level)).
		AddQuery("subject", subject).JSON(body)
	addHeadersToRequest(request, extraHeaders)
	_, err := request.Send()

	return err
}

func (cl *Client) SendLogB(subject, level string, body []byte) error {
	_, err := cl.http.Put().
		AddPath(fmt.Sprintf(logPath, level)).
		AddQuery("subject", subject).
		Body(bytes.NewReader(body)).Send()

	return err
}

func addHeadersToRequest(request *gentleman.Request, headers http.Header) {
	for k, vs := range headers {
		for _, v := range vs {
			request.AddHeader(k, v)
		}
	}
}
