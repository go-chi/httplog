package httplog

import (
	"bytes"
	"net/http"
	"testing"
)

func TestLogBodyContentTypes(t *testing.T) {
	o := &Options{
		LogBodyContentTypes: defaultOptions.LogBodyContentTypes,
		LogBodyMaxLen:       defaultOptions.LogBodyMaxLen,
	}

	tests := []struct {
		name        string
		contentType string
		body        string
		want        string
	}{
		{"json is logged", "application/json", `{"a":1}`, `{"a":1}`},
		{"json with charset is logged", "application/json; charset=utf-8", `{"a":1}`, `{"a":1}`},
		{"missing content type is logged", "", "hello", "hello"},
		{"binary is redacted", "image/png", "\x89PNG", "[body redacted for Content-Type: image/png]"},
		{"gzip is redacted", "application/gzip", "data", "[body redacted for Content-Type: application/gzip]"},
		{"html is redacted", "text/html", "<html>", "[body redacted for Content-Type: text/html]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			header := http.Header{}
			if tt.contentType != "" {
				header.Set("Content-Type", tt.contentType)
			}
			got := logBody(bytes.NewBufferString(tt.body), header, o)
			if got != tt.want {
				t.Errorf("logBody() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLogBodyMaxLen(t *testing.T) {
	o := &Options{
		LogBodyContentTypes: []string{"text/plain"},
		LogBodyMaxLen:       5,
	}
	header := http.Header{}
	header.Set("Content-Type", "text/plain")

	if got := logBody(bytes.NewBufferString("hello world"), header, o); got != "hello... [trimmed]" {
		t.Errorf("logBody() = %q", got)
	}
}
