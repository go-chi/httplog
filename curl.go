package httplog

import (
	"fmt"
	"net/http"
	"strings"
)

// CURL returns a curl command for the given request and body.
func CURL(req *http.Request, reqBody string) string {
	var b strings.Builder

	fmt.Fprintf(&b, "curl")
	if req.Method != "GET" && req.Method != "POST" {
		fmt.Fprintf(&b, " -X %s", req.Method)
	}

	fmt.Fprintf(&b, " %s", singleQuoted(requestURL(req)))

	if req.Method == "POST" {
		fmt.Fprintf(&b, " --data-raw %s", singleQuoted(reqBody))
	}

	for name, vals := range req.Header {
		for _, val := range vals {
			fmt.Fprintf(&b, " -H %s", singleQuoted(fmt.Sprintf("%s: %s", name, val)))
		}
	}

	return b.String()
}

func singleQuoted(v string) string {
	return fmt.Sprintf("'%s'", strings.ReplaceAll(v, "'", `'\''`))
}

func scheme(r *http.Request) string {
	if r.TLS != nil {
		return "https"
	}
	return "http"
}

func requestURL(r *http.Request) string {
	// r.URL is absolute for proxy absolute-form requests (RFC 9112, section 3.2.2);
	// prepending scheme and host again would duplicate them.
	if r.URL.IsAbs() {
		return r.URL.String()
	}
	return fmt.Sprintf("%s://%s%s", scheme(r), r.Host, r.URL)
}
