package clients

import (
	"fmt"
	"net/http"
	"net/url"
)

type NoUserAgentError struct {
	message string
}

func NewNoUserAgentError(message string) *NoUserAgentError {
	return &NoUserAgentError {
		message: message,
	}
}

func (e *NoUserAgentError) Error() string {
    return e.message
}

type ResponseError struct {
	Response   *http.Response
	StatusCode int
	Code       string
	Message    string
}

func (err ResponseError) Error() string {
	var url *url.URL
	if err.Response != nil && err.Response.Request != nil {
		url = err.Response.Request.URL
	}
	return fmt.Sprintf("(%d %v at %v) %v", err.StatusCode, err.Code, url, err.Message)
}
