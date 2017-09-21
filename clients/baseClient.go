package clients

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"time"

	"gopkg.in/h2non/gentleman.v1"
	"gopkg.in/h2non/gentleman.v1/context"
	"gopkg.in/h2non/gentleman.v1/plugin"
	"gopkg.in/h2non/gentleman.v1/plugins/auth"
	"gopkg.in/h2non/gentleman.v1/plugins/headers"
	"gopkg.in/h2non/gentleman.v1/plugins/timeout"
	"gopkg.in/h2non/gentleman.v1/plugins/transport"
)

const HeaderETag = "ETag"

type RequestRecorder interface {
	BeforeDial(req *http.Request)
	Record(req *http.Request, res *http.Response, responseTime time.Duration)
}

type Config struct {
	Account   string
	Workspace string
	Region    string
	Endpoint  string
	AuthToken string
	AuthFunc  func() string
	UserAgent string
	Recorder  RequestRecorder
	Timeout   time.Duration
	Transport http.RoundTripper
}

func CreateClient(service string, config *Config, workspaceBound bool) *gentleman.Client {
	if config == nil {
		panic("config cannot be <nil>")
	}

	if config.Timeout <= 0 {
		config.Timeout = 5 * time.Second
	}

	cl := gentleman.New().
		Use(timeout.Request(config.Timeout)).
		Use(headers.Set("User-Agent", config.UserAgent)).
		Use(responseErrors())
	if config.Recorder != nil {
		cl = cl.Use(requestRecorder(config.Recorder))
	}

	if url := endpoint(service, config); url != "" {
		cl = cl.BaseURL(url)
	}

	if path := basePath(config, workspaceBound); path != "" {
		cl = cl.Path(path)
	}

	if config.AuthToken != "" {
		cl = cl.Use(auth.Bearer(config.AuthToken))
	} else if config.AuthFunc != nil {
		cl = cl.UseRequest(func(ctx *context.Context, h context.Handler) {
			ctx.Request.Header.Set("Authorization", "Bearer "+config.AuthFunc())
			h.Next(ctx)
		})
	}

	if config.Transport != nil {
		cl = cl.Use(transport.Set(config.Transport))
	}

	return cl
}

func responseErrors() plugin.Plugin {
	return plugin.NewResponsePlugin(func(c *context.Context, h context.Handler) {
		if 200 <= c.Response.StatusCode && c.Response.StatusCode < 400 {
			h.Next(c)
			return
		}

		var descr ErrorDescriptor
		var buf []byte
		var err error

		if buf, err = ioutil.ReadAll(c.Response.Body); err != nil {
			descr = ErrorDescriptor{Code: "undefined"}
		} else if err = json.Unmarshal(buf, &descr); err != nil || descr.Code == "" || descr.Message == "" {
			descr = ErrorDescriptor{Code: "undefined", Message: string(buf)}
		}

		h.Error(c, ResponseError{
			Response:   c.Response,
			StatusCode: c.Response.StatusCode,
			Code:       descr.Code,
			Message:    descr.Message,
		})
	})
}

func requestRecorder(recorder RequestRecorder) plugin.Plugin {
	const startTimeKey = "startTime"

	p := plugin.New()
	p.SetHandlers(plugin.Handlers{
		"before dial": func(c *context.Context, h context.Handler) {
			recorder.BeforeDial(c.Request)
			c.Set(startTimeKey, time.Now())
			h.Next(c)
		},
		"response": func(c *context.Context, h context.Handler) {
			responseTime := time.Since(c.Get(startTimeKey).(time.Time))
			recorder.Record(c.Request, c.Response, responseTime)
			h.Next(c)
		},
	})
	return p
}

func endpoint(service string, config *Config) string {
	if config.Endpoint != "" {
		return "http://" + strings.TrimRight(config.Endpoint, "/")
	} else if service != "" {
		return fmt.Sprintf("http://%s.%s.vtex.io", service, config.Region)
	} else {
		return ""
	}
}

func basePath(config *Config, workspaceBound bool) string {
	if workspaceBound {
		return "/" + config.Account + "/" + config.Workspace
	}

	return ""
}
