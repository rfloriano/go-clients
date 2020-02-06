package clients

import (
	goContext "context"
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

const (
	MasterWorkspace = "master"
	HeaderETag      = "ETag"
	startTimeKey    = "startTime"
)

type ClientType int

const (
	AppClient = iota
	InfraClient
	PlatformClient
	ExternalClient
)

type RequestRecorder interface {
	BeforeDial(req *http.Request)
	Record(req *http.Request, res *http.Response, responseTime time.Duration)
}

type Config struct {
	Account   string
	Workspace string
	Region    string
	AuthToken string
	Endpoint  string
	AuthFunc  func() string
	UserAgent string
	Recorder  RequestRecorder
	Context   goContext.Context
	Timeout   time.Duration
	Transport http.RoundTripper
}

type Service struct {
	Name   string
	Vendor string
	Major  int
}

func CreateAppClient(service *Service, major int, config *Config) *gentleman.Client {
	return CreateGenericClient("http://app.io.vtex.com", service, config, AppClient)
}

func CreateInfraClient(service *Service, config *Config) *gentleman.Client {
	return CreateGenericClient("http://infra.io.vtex.com", service, config, InfraClient)
}

func CreatePlatformClient(config *Config) *gentleman.Client {
	return CreateGenericClient("http://platform.io.vtex.com", nil, config, PlatformClient)
}

func CreateExternalClient(url string, config *Config) *gentleman.Client {
	return CreateGenericClient(url, nil, config, ExternalClient)
}

func CreateGenericClient(url string, service *Service, config *Config, clientType ClientType) *gentleman.Client {
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
	if config.Context != nil && config.Context != goContext.Background() {
		cl = cl.Use(contextBinder(config.Context))
	}

	if config.Endpoint != "" {
		cl = cl.URL(config.Endpoint)
	} else if url != "" {
		cl = cl.URL(url)
	}

	if path := basePath(service, config, clientType); path != "" {
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

func contextBinder(ctx goContext.Context) plugin.Plugin {
	return plugin.NewRequestPlugin(func(c *context.Context, h context.Handler) {
		newCtx := ctx
		if original := c.Request.Context(); original != goContext.Background() {
			newCtx = linkedContext(original, newCtx)
		}
		c.Request = c.Request.WithContext(newCtx)
		h.Next(c)
	})
}

func linkedContext(ctx1, ctx2 goContext.Context) goContext.Context {
	linked, cancel := goContext.WithCancel(goContext.Background())
	go func() {
		defer cancel()
		select {
		case <-ctx1.Done():
		case <-ctx2.Done():
		}
	}()
	return linked
}

func requestRecorder(recorder RequestRecorder) plugin.Plugin {
	p := plugin.New()
	p.SetHandlers(plugin.Handlers{
		"before dial": func(c *context.Context, h context.Handler) {
			recorder.BeforeDial(c.Request)
			c.Set(startTimeKey, time.Now())
			h.Next(c)
		},
		"response": func(c *context.Context, h context.Handler) {
			recordResponse(recorder, c)
			h.Next(c)
		},
		"error": func(c *context.Context, h context.Handler) {
			//Every response with status code >= 400 is transformed to an Error
			//by the middleware "responseErrors". That's why this code is not inside
			//of response handler.
			if c.Response != nil && c.Response.StatusCode == http.StatusNotFound {
				recordResponse(recorder, c)
			}
			h.Next(c)
		},
	})
	return p
}

func recordResponse(recorder RequestRecorder, c *context.Context) {
	if startTime, ok := c.GetOk(startTimeKey); ok {
		responseTime := time.Since(startTime.(time.Time))
		recorder.Record(c.Request, c.Response, responseTime)
	}
}

func basePath(service *Service, config *Config, clientType ClientType) string {
	switch clientType {
	case AppClient, InfraClient:
		return fmt.Sprintf("/%s/v%d/%s/%s", service.Name, service.Major, config.Account, config.Workspace)
	case PlatformClient:
		return "/" + config.Account + "/" + config.Workspace
	default:
		return ""
	}
}

func UserAgentName(config *Config) string {
	appName := strings.SplitN(config.UserAgent, "/", 2)
	return strings.ToLower(appName[0])
}
