package httplog

import (
	"fmt"
	"strings"
)

func (l *log) curl() string {
	var r = l.Req
	var b strings.Builder

	fmt.Fprintf(&b, "curl")
	if r.Method != "GET" && r.Method != "POST" {
		fmt.Fprintf(&b, " -X %s", r.Method)
	}

	fmt.Fprintf(&b, " %s", singleQuoted(fmt.Sprintf("%s://%s%s", l.scheme(), r.Host, r.URL)))

	if r.Method == "POST" {
		fmt.Fprintf(&b, " --data-raw %s", singleQuoted(l.ReqBody.String()))
	}

	for name, vals := range r.Header {
		for _, val := range vals {
			fmt.Fprintf(&b, " -H %s", singleQuoted(fmt.Sprintf("%s: %s", name, val)))
		}
	}

	return b.String()
}

func (l *log) scheme() string {
	if l.Req.TLS != nil {
		return "https"
	}
	return "http"
}

func singleQuoted(v string) string {
	return fmt.Sprintf("'%s'", strings.ReplaceAll(v, "'", `'\''`))
}
