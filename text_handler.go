package httplog

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"runtime"
	"sync"
	"time"
)

type PrettyHandler struct {
	opts              *slog.HandlerOptions
	w                 io.Writer
	preformattedAttrs *bytes.Buffer
	groupPrefix       *string
	groupOpen         bool
	mu                sync.Mutex
}

var DefaultHandlerConfig = &slog.HandlerOptions{
	Level:     slog.LevelInfo,
	AddSource: true,
}

func NewPrettyHandler(w io.Writer, options ...*slog.HandlerOptions) *PrettyHandler {
	var opts *slog.HandlerOptions
	if len(options) == 0 {
		opts = DefaultHandlerConfig
	} else {
		opts = options[0]
	}

	return &PrettyHandler{
		opts:              opts,
		w:                 w,
		preformattedAttrs: &bytes.Buffer{},
		mu:                sync.Mutex{},
	}
}

var _ slog.Handler = &PrettyHandler{}

func (h *PrettyHandler) Enabled(ctx context.Context, level slog.Level) bool {
	minLevel := slog.LevelInfo
	if h.opts != nil && h.opts.Level != nil {
		minLevel = h.opts.Level.Level()
	}
	return level >= minLevel
}

func (h *PrettyHandler) Handle(ctx context.Context, r slog.Record) error {
	buf := &bytes.Buffer{}

	if !r.Time.IsZero() {
		timeAttr := slog.Attr{
			Key:   slog.TimeKey,
			Value: slog.TimeValue(r.Time),
		}
		if h.opts != nil && h.opts.ReplaceAttr != nil {
			timeAttr = h.opts.ReplaceAttr([]string{}, timeAttr)
		} else {
			timeAttr.Value = slog.StringValue(timeAttr.Value.Time().Format(time.RFC3339Nano))
		}
		// write time, level and source to buf
		cW(buf, false, bBlack, "%s", timeAttr.Value.String())
		buf.WriteString(" ")
	}

	levelAttr := slog.Attr{
		Key:   slog.LevelKey,
		Value: slog.StringValue(r.Level.String()),
	}
	if h.opts != nil && h.opts.ReplaceAttr != nil {
		levelAttr = h.opts.ReplaceAttr([]string{}, levelAttr)
	}
	cW(buf, true, levelColor(r.Level), "%s", levelAttr.Value.String())
	buf.WriteString(" ")

	if h.opts != nil && h.opts.AddSource {
		s := source(r)
		file := s.File
		line := s.Line
		function := s.Function
		cW(buf, true, nGreen, "%s:%s:%d", function, file, line)
		buf.WriteString(" ")
	}

	// write message to buf
	cW(buf, true, bWhite, "%s", r.Message)
	buf.WriteString(" ")
	// write preformatted attrs to buf
	buf.Write(h.preformattedAttrs.Bytes())
	// close group in preformatted attrs if open\
	if h.groupOpen {
		cW(h.preformattedAttrs, true, nWhite, "%s", "}")
	}

	// write record level attrs to buf
	attrs := []slog.Attr{}
	r.Attrs(func(attr slog.Attr) bool {
		attrs = append(attrs, attr)
		return true
	})
	writeAttrs(buf, attrs, false)

	buf.WriteString("\n")
	h.mu.Lock()
	defer h.mu.Unlock()
	h.w.Write(buf.Bytes())
	return nil
}

func (h *PrettyHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	h2 := h.clone()
	writeAttrs(h2.preformattedAttrs, attrs, false)
	return h2
}

func source(r slog.Record) *slog.Source {
	fs := runtime.CallersFrames([]uintptr{r.PC})
	f, _ := fs.Next()
	return &slog.Source{
		Function: f.Function,
		File:     f.File,
		Line:     f.Line,
	}
}

func writeAttrs(w *bytes.Buffer, attrs []slog.Attr, insideGroup bool) {
	for i, attr := range attrs {
		cW(w, true, nYellow, "%s: ", attr.Key)
		if insideGroup && i == len(attrs)-1 {
			writeAttrValue(w, attr.Value, false)
		} else {
			writeAttrValue(w, attr.Value, true)
		}
	}
}

func writeAttrValue(w *bytes.Buffer, value slog.Value, appendSpace bool) {
	if appendSpace {
		defer w.WriteString(" ")
	}
	switch v := value.Kind(); v {
	case slog.KindString:
		cW(w, true, nCyan, "%q", value.String())
	case slog.KindBool:
		cW(w, true, nCyan, "%t", value.Bool())
	case slog.KindInt64:
		cW(w, true, nCyan, "%d", value.Int64())
	case slog.KindDuration:
		cW(w, true, nCyan, "%s", value.Duration().String())
	case slog.KindFloat64:
		cW(w, true, nCyan, "%f", value.Float64())
	case slog.KindTime:
		cW(w, true, nCyan, "%s", value.Time().Format(time.RFC3339))
	case slog.KindUint64:
		cW(w, true, nCyan, "%d", value.Uint64())
	case slog.KindGroup:
		cW(w, true, nWhite, "{")
		writeAttrs(w, value.Group(), true)
		cW(w, true, nWhite, "%s", "}")
	default:
		cW(w, true, nCyan, "%s", value.String())
	}
}

func levelColor(l slog.Level) []byte {
	switch l {
	case slog.LevelDebug:
		return nYellow
	case slog.LevelInfo:
		return nGreen
	case slog.LevelWarn:
		return nRed
	case slog.LevelError:
		return bRed
	default:
		return bWhite
	}
}

func (h *PrettyHandler) WithGroup(name string) slog.Handler {
	h2 := h.clone()
	if h2.groupPrefix != nil {
		// end old group
		cW(h2.preformattedAttrs, true, nWhite, "}")
	}
	h2.groupOpen = true
	h2.groupPrefix = &name
	cW(h2.preformattedAttrs, true, bMagenta, "%s: {", name)
	return h
}

func (h *PrettyHandler) clone() *PrettyHandler {
	newBuffer := &bytes.Buffer{}
	newBuffer.Write(h.preformattedAttrs.Bytes())

	return &PrettyHandler{
		opts:              h.opts,
		w:                 h.w,
		groupPrefix:       h.groupPrefix,
		preformattedAttrs: newBuffer,
	}
}
