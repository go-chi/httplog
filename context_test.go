package httplog

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestSetAttrsConcurrent(t *testing.T) {
	const n = 200
	ctx := context.WithValue(context.Background(), ctxKeyLogAttrs{}, newLogAttrs())

	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		i := i
		go func() {
			defer wg.Done()
			SetAttrs(ctx, slog.Int("n", i))
		}()
	}
	wg.Wait()

	got := getAttrs(ctx)
	if len(got) != n {
		t.Fatalf("got %d attrs, want %d", len(got), n)
	}
}

func TestSetAttrsConcurrentThroughMiddleware(t *testing.T) {
	const n = 100
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	mw := RequestLogger(logger, &Options{Schema: SchemaECS})

	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var wg sync.WaitGroup
		wg.Add(n)
		for i := 0; i < n; i++ {
			i := i
			go func() {
				defer wg.Done()
				SetAttrs(r.Context(), slog.Int("n", i))
			}()
		}
		wg.Wait()
		w.WriteHeader(http.StatusOK)
	})

	mw(h).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))

	if buf.Len() == 0 {
		t.Fatal("expected a log line")
	}
}
