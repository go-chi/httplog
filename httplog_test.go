package httplog

import (
	"context"
	"log/slog"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5/middleware"
)

func TestLogEntrySetFields(t *testing.T) {

	type args struct {
		handler *testHandler
		fields  map[string]interface{}
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "test_fields_set",
			args: args{
				handler: &testHandler{},
				fields: map[string]interface{}{
					"foo": 1000,
					"bar": "account",
				},
			},
		},
		{
			name: "test_empty",
			args: args{
				handler: &testHandler{},
				fields:  make(map[string]interface{}),
			},
		},
		{
			name: "test_fields_nil",
			args: args{
				handler: &testHandler{},
				fields:  nil,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := &RequestLoggerEntry{
				Logger: slog.New(tt.args.handler),
			}
			req := middleware.WithLogEntry(httptest.NewRequest("GET", "/", nil), entry)
			LogEntrySetFields(req.Context(), tt.args.fields)

			if len(tt.args.handler.attrs) != len(tt.args.fields) {
				t.Fatalf("expected %v, got %v", len(tt.args.handler.attrs), len(tt.args.fields))
			}
			// Ensure all fields are present in the handler
			for k, v := range tt.args.fields {
				for i, attr := range tt.args.handler.attrs {
					if attr.Key == k {
						if !attr.Value.Equal(slog.AnyValue(v)) {
							t.Fatalf("expected %v, got %v", attr.Value, v)
						}
						break
					}
					if i == len(tt.args.handler.attrs)-1 {
						t.Fatalf("expected %v, got %v", k, attr.Key)
					}
				}
			}
		})
	}
}

type testHandler struct {
	attrs []slog.Attr
}

func (*testHandler) Enabled(_ context.Context, l slog.Level) bool { return true }

func (h *testHandler) Handle(ctx context.Context, r slog.Record) error { return nil }

func (h *testHandler) WithAttrs(as []slog.Attr) slog.Handler {
	h.attrs = as
	return h
}

func (h *testHandler) WithGroup(name string) slog.Handler { return h }
