package clients

import (
	"encoding/json"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	uuid "github.com/satori/go.uuid"
)

const (
	metadataHeader    = "X-Vtex-Meta"
	enableTraceHeader = "X-Vtex-Trace-Enable"
	traceHeader       = "X-Call-Trace"
	requestIDHeader   = "X-Request-Id"
)

func NewIOHeadersRecorder(parent *http.Request) *IOHeadersRecorder {
	var headers http.Header
	if parent != nil {
		headers = parent.Header
	}

	enableTrace, _ := strconv.ParseBool(headers.Get(enableTraceHeader))
	requestID := headers.Get(requestIDHeader)
	if requestID == "" {
		requestID = uuid.NewV4().String()
	}

	return &IOHeadersRecorder{
		parentLogFields: requestLogFields(parent),
		responseHeaders: http.Header{},
		enableTrace:     enableTrace,
		callTrace:       []*CallTree{},
		requestID:       requestID,
	}
}

type IOHeadersRecorder struct {
	mu sync.RWMutex

	parentLogFields logrus.Fields
	responseHeaders http.Header
	enableTrace     bool
	callTrace       []*CallTree
	requestID       string

	written bool
}

func (r *IOHeadersRecorder) BeforeDial(req *http.Request) {
	if r.enableTrace {
		req.Header.Set(enableTraceHeader, "true")
	}
	req.Header.Set(requestIDHeader, r.requestID)
}

// Record records a request made in order to accumulate headers
func (r *IOHeadersRecorder) Record(req *http.Request, res *http.Response, responseTime time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.written && (req.Method == http.MethodGet || req.Method == http.MethodHead) {
		r.log("headers_record_after_written", req, nil).
			Warn("Request recorded after parent response already written")
	}

	for _, h := range res.Header[metadataHeader] {
		r.responseHeaders.Add(metadataHeader, h)
	}

	if r.enableTrace {
		r.recordChildCallTree(req, res, responseTime)
	}
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
			r.log("call_trace_marshal_error", nil, err).
				Error("Failed to marshall call trace")
		}
		out.Set(traceHeader, string(trace))
	}
}

func (r *IOHeadersRecorder) recordChildCallTree(req *http.Request, res *http.Response, responseTime time.Duration) {
	resh := res.Header.Get(traceHeader)
	var children []*CallTree
	if err := json.Unmarshal([]byte(resh), &children); err != nil && resh != "" {
		r.log("call_trace_child_error", req, err).
			Error("Failed to unmarshal child call trace")
		children = nil
	}

	cache := "miss"
	if _, ok := res.Header["X-From-Cache"]; ok {
		cache = "hit"
	}

	r.callTrace = append(r.callTrace, &CallTree{
		Call:     req.Method + " " + req.URL.String(),
		Time:     responseTime.Nanoseconds() / int64(time.Millisecond),
		Status:   res.StatusCode,
		Cache:    cache,
		Children: children,
	})
}

func (r *IOHeadersRecorder) log(code string, req *http.Request, err error) *logrus.Entry {
	logger := logrus.WithFields(logrus.Fields{
		"code":       code,
		"parent_req": r.parentLogFields,
	})
	if req != nil {
		logger = logger.WithField("child_req", requestLogFields(req))
	}
	if err != nil {
		logger = logger.WithError(err)
	}
	return logger
}

type CallTree struct {
	Call     string      `json:"call"`
	Status   int         `json:"status"`
	Cache    string      `json:"cache"`
	Time     int64       `json:"time"`
	Children []*CallTree `json:"children,omitempty"`
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
