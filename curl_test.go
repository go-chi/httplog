package httplog

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCURL(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users", nil)
	req.Host = "example.com"
	req.Header.Set("Authorization", "Bearer secret")
	got := CURL(req, "")
	expected := "curl 'http://example.com/api/v1/users' -H 'Authorization: Bearer secret'"
	if got != expected {
		t.Errorf("CURL(GET) = %q, want %q", got, expected)
	}

	postReq := httptest.NewRequest(http.MethodPost, "/api/v1/users", nil)
	postReq.Host = "example.com"
	postGot := CURL(postReq, `{"name":"alice"}`)
	expectedPost := `curl 'http://example.com/api/v1/users' --data-raw '{"name":"alice"}'`
	if postGot != expectedPost {
		t.Errorf("CURL(POST) = %q, want %q", postGot, expectedPost)
	}

	putReq := httptest.NewRequest(http.MethodPut, "/api/v1/users/1", nil)
	putReq.Host = "example.com"
	putGot := CURL(putReq, "")
	expectedPut := `curl -X PUT 'http://example.com/api/v1/users/1'`
	if putGot != expectedPut {
		t.Errorf("CURL(PUT) = %q, want %q", putGot, expectedPut)
	}
}
