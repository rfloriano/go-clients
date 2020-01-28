package courier

import (
	"fmt"
	"net/http"

	"github.com/vtex/go-clients/clients"
	"gopkg.in/h2non/gentleman.v1"
)

type Courier interface {
	SendEvent(resource, topic string, body interface{}, extraHeaders http.Header) error
	SendLog(resource, level string, body interface{}, extraHeaders http.Header) error
}

type Client struct {
	http *gentleman.Client
}

func NewClient(config *clients.Config) Courier {
	cl := clients.CreateClient("courier", config, true)
	return &Client{cl}
}

const (
	eventPath = "/events/%v"
	logPath   = "/logs/%v"
)

func (cl *Client) SendEvent(resource, topic string, body interface{}, extraHeaders http.Header) error {
	request := cl.http.Put().AddPath(fmt.Sprintf(eventPath, topic)).
		AddQuery("resource", resource).JSON(body)
	addHeadersToRequest(request, extraHeaders)
	_, err := request.Send()

	return err
}

func addHeadersToRequest(request *gentleman.Request, headers http.Header) {
	for k, vs := range headers {
		for _, v := range vs {
			request.AddHeader(k, v)
		}
	}
}

func (cl *Client) SendLog(resource, level string, body interface{}, extraHeaders http.Header) error {
	request := cl.http.Put().AddPath(fmt.Sprintf(logPath, level)).
		AddQuery("resource", resource).JSON(body)
	addHeadersToRequest(request, extraHeaders)
	_, err := request.Send()

	return err
}
