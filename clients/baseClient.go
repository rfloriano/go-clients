package clients

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"time"

	"gopkg.in/h2non/gentleman.v1"
	"gopkg.in/h2non/gentleman.v1/context"
	"gopkg.in/h2non/gentleman.v1/plugin"
	"gopkg.in/h2non/gentleman.v1/plugins/headers"
	"gopkg.in/h2non/gentleman.v1/plugins/timeout"
	"gopkg.in/h2non/gentleman.v1/plugins/transport"
)

type ClientOptions struct {
	UserAgent string
	Timeout   time.Duration
	AuthFunc  func() string
	Transport http.RoundTripper
}

func CreateBaseClient(url string, opts ClientOptions) *gentleman.Client {
	if opts.Timeout <= 0 {
		opts.Timeout = 5 * time.Second
	}

	cl := gentleman.New().
		Use(headers.Set("User-Agent", opts.UserAgent)).
		Use(timeout.Request(opts.Timeout)).
		Use(responseErrors())

	if url != "" {
		cl.URL(url)
	}
	if opts.AuthFunc != nil {
		cl.UseRequest(func(ctx *context.Context, h context.Handler) {
			ctx.Request.Header.Set("Authorization", "Bearer "+opts.AuthFunc())
			h.Next(ctx)
		})
	}
	if opts.Transport != nil {
		cl.Use(transport.Set(opts.Transport))
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
