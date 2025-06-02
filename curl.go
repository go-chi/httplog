package httplog

import (
	"fmt"
	"net/http"
	"strings"
)

func curl(r *http.Request, reqBody string) string {
	var b strings.Builder

	fmt.Fprintf(&b, "curl")
	if r.Method != "GET" && r.Method != "POST" {
		fmt.Fprintf(&b, " -X %s", r.Method)
	}

	fmt.Fprintf(&b, " %s", singleQuoted(requestURL(r)))

	if r.Method == "POST" {
		fmt.Fprintf(&b, " --data-raw %s", singleQuoted(reqBody))
	}

	for name, vals := range r.Header {
		for _, val := range vals {
			fmt.Fprintf(&b, " -H %s", singleQuoted(fmt.Sprintf("%s: %s", name, val)))
		}
	}

	return b.String()
}

func singleQuoted(v string) string {
	return fmt.Sprintf("'%s'", strings.ReplaceAll(v, "'", `'\''`))
}

func requestURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}

	return fmt.Sprintf("%s://%s%s", scheme, r.Host, r.URL)
}
