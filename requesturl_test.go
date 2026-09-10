package httplog

import (
	"net/http/httptest"
	"testing"
)

func TestRequestURL(t *testing.T) {
	// httptest.NewRequest with an absolute target keeps r.URL absolute,
	// mimicking proxy absolute-form requests. See issue #72.
	abs := httptest.NewRequest("GET", "http://localhost:8080/api/users?q=1", nil)
	if got, want := requestURL(abs), "http://localhost:8080/api/users?q=1"; got != want {
		t.Errorf("requestURL() = %q, want %q", got, want)
	}

	rel := httptest.NewRequest("GET", "/api/users", nil)
	rel.Host = "example.com"
	if got, want := requestURL(rel), "http://example.com/api/users"; got != want {
		t.Errorf("requestURL() = %q, want %q", got, want)
	}
}
