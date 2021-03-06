package clients

import (
	"encoding/json"
	"net/http"
	"net/textproto"
	"strconv"
	"sync"
	"time"

	uuid "github.com/satori/go.uuid"
	"github.com/sirupsen/logrus"
)

var (
	header = textproto.CanonicalMIMEHeaderKey

	enableTraceHeader     = header("X-Vtex-Trace-Enable")
	traceHeader           = header("X-Call-Trace")
	operationIDHeader     = header("X-Operation-Id")
	smartCacheHeader      = header("X-Vtex-Meta")
	solvedConflictsHeader = header("X-Vtex-Solved-Conflicts")

	headersToRecord = []string{smartCacheHeader, solvedConflictsHeader}
)

func NewIOHeadersRecorder(parent *http.Request) *IOHeadersRecorder {
	var headers http.Header
	if parent != nil {
		headers = parent.Header
	}

	enableTrace, _ := strconv.ParseBool(headers.Get(enableTraceHeader))
	operationID := headers.Get(operationIDHeader)
	if operationID == "" {
		id, err := uuid.NewV4()
		if err == nil {
			operationID = id.String()
		}
	}

	return &IOHeadersRecorder{
		parentLogFields: requestLogFields(parent),
		recordedHeaders: http.Header{},
		enableTrace:     enableTrace,
		callTrace:       []*CallTree{},
		operationID:     operationID,
	}
}

type IOHeadersRecorder struct {
	mu sync.RWMutex

	parentLogFields logrus.Fields
	recordedHeaders http.Header
	enableTrace     bool
	callTrace       []*CallTree
	operationID     string

	written bool
}

func (r *IOHeadersRecorder) BeforeDial(req *http.Request) {
	if r.enableTrace {
		req.Header.Set(enableTraceHeader, "true")
	}
	if r.operationID != "" {
		req.Header.Set(operationIDHeader, r.operationID)
	}
}

// Record records a request made in order to accumulate headers
func (r *IOHeadersRecorder) Record(req *http.Request, res *http.Response, responseTime time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.written && (req.Method == http.MethodGet || req.Method == http.MethodHead) {
		r.log("headers_record_after_written", req, nil).
			Warn("Request recorded after parent response already written")
	}

	for _, h := range headersToRecord {
		r.recordedHeaders[h] = append(r.recordedHeaders[h], res.Header[h]...)
	}

	if r.enableTrace {
		r.recordChildCallTree(req, res, responseTime)
	}
}

func (r *IOHeadersRecorder) CopyFrom(other *IOHeadersRecorder) {
	// We grab an initial copy of the other data to avoid a potential deadlock
	// if we grabbed locks of the 2 instances at the same time.
	headersCopy, callTraceCopy := other.recordedDataCopy()

	r.mu.Lock()
	defer r.mu.Unlock()

	copyHeaders(r.recordedHeaders, headersCopy)
	r.callTrace = append(r.callTrace, callTraceCopy...)
}

func (r *IOHeadersRecorder) recordedDataCopy() (http.Header, []*CallTree) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	headers := make(http.Header, len(r.recordedHeaders))
	copyHeaders(headers, r.recordedHeaders)

	callTrace := make([]*CallTree, len(r.callTrace))
	copy(callTrace, r.callTrace)

	return headers, callTrace
}

func copyHeaders(dest, src http.Header) {
	for name, values := range src {
		dest[name] = append(dest[name], values...)
	}
}

// AddResponseHeaders writes accumulated headers to an outgoing response
func (r *IOHeadersRecorder) AddResponseHeaders(out http.Header) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	r.written = true

	for h, values := range r.recordedHeaders {
		out[h] = append(out[h], values...)
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

func (r *IOHeadersRecorder) ClearSmartCacheHeaders() {
	r.mu.Lock()
	delete(r.recordedHeaders, smartCacheHeader)
	r.mu.Unlock()
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
