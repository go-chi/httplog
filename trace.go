package httplog

import (
	"cmp"
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

const (
	_headerTraceID = "X-Trace-ID"
	_logFieldTrace = "trace_id"
	_logFieldSpan  = "span_id"
)

type contextKey struct {
	name string
}

func (k *contextKey) String() string {
	return "httplog context value " + k.name
}

var (
	_contextKeyTrace = &contextKey{"trace_id"}
	_contextKeySpan  = &contextKey{"span_id"}
)

// NeTransport returns a new http.RoundTripper that propagates the TraceID.
func NewTransport(header string, base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return traceTransport{
		Header: cmp.Or(header, _headerTraceID),
		Base:   base,
	}
}

type traceTransport struct {
	Header string
	Base   http.RoundTripper
}

func (t traceTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if id, ok := r.Context().Value(_contextKeyTrace).(string); ok {
		r.Header.Set(cmp.Or(t.Header, _headerTraceID), id)
	}
	return t.Base.RoundTrip(r)
}

func newID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
