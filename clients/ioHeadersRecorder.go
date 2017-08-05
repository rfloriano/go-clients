package clients

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
)

const (
	metadataHeader    = "X-Vtex-Meta"
	enableTraceHeader = "X-Vtex-Trace-Enable"
	traceHeader       = "X-Call-Trace"
)

func NewIOHeadersRecorder(parent *http.Request) *IOHeadersRecorder {
	var callTrace []*CallTree
	enableTrace := false
	if parent != nil && parent.Header.Get(enableTraceHeader) == "true" {
		enableTrace = true
		callTrace = []*CallTree{}
	}

	return &IOHeadersRecorder{
		logFields: logrus.Fields{
			"code":   "io_headers_recorder",
			"parent": requestLogFields(parent),
		},
		responseHeaders: http.Header{},
		enableTrace:     enableTrace,
		callTrace:       callTrace,
	}
}

type IOHeadersRecorder struct {
	mu sync.RWMutex

	logFields       logrus.Fields
	responseHeaders http.Header
	enableTrace     bool
	callTrace       []*CallTree

	written bool
}

// Record records a request made in order to accumulate headers
func (r *IOHeadersRecorder) Record(req *http.Request, res *http.Response, responseTime time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.written {
		logrus.WithFields(r.logFields).
			WithField("request", requestLogFields(req)).
			Warn("Request recorded after parent response already written")
	}

	for _, h := range res.Header[metadataHeader] {
		r.responseHeaders.Add(metadataHeader, h)
	}

	if r.enableTrace {
		r.callTrace = append(r.callTrace, newCallTree(req, res, responseTime))
	}
}

func (r *IOHeadersRecorder) EnableTrace() bool {
	return r.enableTrace
}

// AddResponseHeaders writes accumulated headers to an outgoing response
func (r *IOHeadersRecorder) AddResponseHeaders(out http.Header) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	r.written = true

	for h, v := range r.responseHeaders {
		out[h] = append(out[h], v...)
	}

	if r.enableTrace {
		trace, err := json.Marshal(r.callTrace)
		if err != nil {
			logrus.WithError(err).
				WithFields(r.logFields).
				Error("Failed to marshall call trace")
		}
		out.Set(traceHeader, string(trace))
	}
}

type CallTree struct {
	Call     string      `json:"call"`
	Status   int         `json:"status"`
	Cache    string      `json:"cache"`
	Time     int64       `json:"time"`
	Children []*CallTree `json:"children,omitempty"`
}

func newCallTree(req *http.Request, res *http.Response, responseTime time.Duration) *CallTree {
	resh := res.Header.Get(traceHeader)
	var children []*CallTree
	if err := json.Unmarshal([]byte(resh), &children); err != nil && resh != "" {
		logrus.WithError(err).Error("Failed to unmarshal child call trace")
	}

	cache := "miss"
	if _, ok := res.Header["X-From-Cache"]; ok {
		cache = "hit"
	}

	return &CallTree{
		Call:     req.Method + " " + req.URL.String(),
		Time:     responseTime.Nanoseconds() / int64(time.Millisecond),
		Status:   res.StatusCode,
		Cache:    cache,
		Children: children,
	}
}

func requestLogFields(req *http.Request) logrus.Fields {
	if req == nil {
		return logrus.Fields{}
	}

	return logrus.Fields{
		"host":   req.URL.Host,
		"path":   req.URL.Path,
		"query":  req.URL.RawQuery,
		"method": req.Method,
	}
}
