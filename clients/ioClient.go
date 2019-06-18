package clients

import (
	goContext "context"
	"fmt"
	"net/http"
	"strings"
	"time"

	gentleman "gopkg.in/h2non/gentleman.v1"
	"gopkg.in/h2non/gentleman.v1/context"
	"gopkg.in/h2non/gentleman.v1/plugin"
	"gopkg.in/h2non/gentleman.v1/plugins/auth"
)

const (
	HeaderETag      = "ETag"
	startTimeKey    = "startTime"
	MasterWorkspace = "master"
)

type IOClientOptions struct {
	ClientOptions
	Region string
}

func CreateBaseInfraClient(service string, opts IOClientOptions) *gentleman.Client {
	return CreateBaseClient(infraEndpoint(service, opts.Region), opts.ClientOptions)
}

func CreateBaseAppClient(vendor, name string, opts IOClientOptions) *gentleman.Client {
	return CreateBaseClient(appEndpoint(vendor, name, opts.Region), opts.ClientOptions)
}

type RequestRecorder interface {
	BeforeDial(req *http.Request)
	Record(req *http.Request, res *http.Response, responseTime time.Duration)
}

type IORequestContext struct {
	Account   string
	Workspace string
	AuthToken string
	Recorder  RequestRecorder
	Context   goContext.Context
}

type Config struct {
	IOClientOptions
	IORequestContext
}

func WithContext(base *gentleman.Client, ctx IORequestContext) *gentleman.Client {
	cl := gentleman.New().UseParent(base)

	if path := basePath(&ctx); path != "" {
		cl.Path(path)
	}
	if ctx.AuthToken != "" {
		cl.Use(auth.Bearer(ctx.AuthToken))
	}
	if ctx.Recorder != nil {
		cl.Use(newRequestRecorderPlugin(ctx.Recorder))
	}
	if ctx.Context != nil && ctx.Context != goContext.Background() {
		cl.Use(contextBinder(ctx.Context))
	}
	return cl
}

func withContextCompat(base *gentleman.Client, config *Config, workspaceBound bool) *gentleman.Client {
	if workspaceBound && (config.Account == "" || config.Workspace == "") {
		panic("Missing account or workspace for workspace-bound client")
	}
	if !workspaceBound && (config.Account != "" || config.Workspace != "") {
		panic("Non workspace-bound client with account or workspace config")
	}
	return WithContext(base, config.IORequestContext)
}

func CreateAppClient(vendor, name string, config *Config) *gentleman.Client {
	base := CreateBaseAppClient(vendor, name, config.IOClientOptions)
	return withContextCompat(base, config, true)
}

func CreateClient(service string, config *Config, workspaceBound bool) *gentleman.Client {
	base := CreateBaseInfraClient(service, config.IOClientOptions)
	return withContextCompat(base, config, workspaceBound)
}

func CreateGenericClient(baseURL string, config *Config, workspaceBound bool) *gentleman.Client {
	base := CreateBaseClient(baseURL, config.ClientOptions)
	return withContextCompat(base, config, workspaceBound)
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

// TODO: Move this to a separate plugins package (or just a separate file)
type requestRecorderPlugin struct {
	recorder RequestRecorder
}

func (p *requestRecorderPlugin) BeforeDial(c *context.Context, h context.Handler) {
	p.recorder.BeforeDial(c.Request)
	c.Set(startTimeKey, time.Now())
	h.Next(c)
}

func (p *requestRecorderPlugin) Response(c *context.Context, h context.Handler) {
	p.recordResponse(c)
	h.Next(c)
}

func (p *requestRecorderPlugin) Error(c *context.Context, h context.Handler) {
	//Every response with status code >= 400 is transformed to an Error
	//by the middleware "responseErrors". That's why this code is not inside
	//of response handler.
	if c.Response != nil && c.Response.StatusCode == http.StatusNotFound {
		p.recordResponse(c)
	}
	h.Next(c)
}

func (p *requestRecorderPlugin) recordResponse(c *context.Context) {
	if startTime, ok := c.GetOk(startTimeKey); ok {
		responseTime := time.Since(startTime.(time.Time))
		p.recorder.Record(c.Request, c.Response, responseTime)
	}
}

func newRequestRecorderPlugin(recorder RequestRecorder) plugin.Plugin {
	recorderPlugin := &requestRecorderPlugin{recorder}
	p := plugin.New()
	p.SetHandlers(plugin.Handlers{
		"before dial": recorderPlugin.BeforeDial,
		"response":    recorderPlugin.Response,
		"error":       recorderPlugin.Error,
	})
	return p
}

func appEndpoint(vendor, name, region string) string {
	return fmt.Sprintf("http://%s.%s.%s.vtex.io", name, vendor, region)
}

func infraEndpoint(service, region string) string {
	return fmt.Sprintf("http://%s.%s.vtex.io", service, region)
}

func basePath(ctx *IORequestContext) string {
	if ctx.Account != "" && ctx.Workspace != "" {
		return "/" + ctx.Account + "/" + ctx.Workspace
	}
	return ""
}

func UserAgentName(userAgent string) string {
	appName := strings.SplitN(userAgent, "/", 2)
	return strings.ToLower(appName[0])
}
