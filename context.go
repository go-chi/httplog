package httplog

type ctxKey struct{}

func (ctxKey) String() string {
	return "httplog context"
}
