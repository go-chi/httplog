package httplog

import (
	"fmt"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

// Guards the concatenated default message against drifting from the
// original fmt.Sprintf("%s %s => HTTP %v (%v)") format.
func TestDefaultLogMessageFormat(t *testing.T) {
	for _, tc := range []struct {
		method, url string
		status      int
		duration    time.Duration
	}{
		{"GET", "/api/users?q=1", 200, 1234567 * time.Nanosecond},
		{"POST", "/api/users", 422, 2 * time.Second},
		{"DELETE", "/x", 500, 0},
		{"OPTIONS", "/", 204, 90 * time.Minute},
	} {
		r := httptest.NewRequest(tc.method, tc.url, nil)

		want := fmt.Sprintf("%s %s => HTTP %v (%v)", r.Method, r.URL, tc.status, tc.duration)
		got := r.Method + " " + r.URL.String() + " => HTTP " + strconv.Itoa(tc.status) + " (" + tc.duration.String() + ")"

		if got != want {
			t.Errorf("message format drifted:\n  got:  %q\n  want: %q", got, want)
		}
	}
}
