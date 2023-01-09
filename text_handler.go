package httplog

import (
	"bytes"
	"io"
	"sync"
	"time"

	"golang.org/x/exp/slog"
)

type PrettyHandler struct {
	mu                sync.Mutex
	opts              *slog.HandlerOptions
	w                 io.Writer
	preformattedAttrs *bytes.Buffer
	groupPrefix       *string
	groupOpen         bool
}

var DefaultHandlerConfig = &slog.HandlerOptions{
	AddSource: true,
	Level:     slog.LevelInfo,
}

func NewPrettyHandler(w io.Writer, op ...*slog.HandlerOptions) *PrettyHandler {
	var config *slog.HandlerOptions
	if len(op) == 0 {
		config = DefaultHandlerConfig
	} else {
		config = op[0]
	}

	return &PrettyHandler{
		opts:              config,
		w:                 w,
		preformattedAttrs: &bytes.Buffer{},
		mu:                sync.Mutex{},
	}
}

var _ slog.Handler = &PrettyHandler{}

func (h *PrettyHandler) Enabled(level slog.Level) bool {
	minLevel := slog.LevelInfo
	if h.opts.Level != nil {
		minLevel = h.opts.Level.Level()
	}
	return level >= minLevel
}

func (h *PrettyHandler) Handle(r slog.Record) error {
	buf := &bytes.Buffer{}

	if !r.Time.IsZero() {
		timeAttr := slog.Attr{
			Key:   slog.TimeKey,
			Value: slog.TimeValue(r.Time),
		}
		if h.opts.ReplaceAttr != nil {
			timeAttr = h.opts.ReplaceAttr([]string{}, timeAttr)
		} else {
			timeAttr.Value = slog.StringValue(timeAttr.Value.Time().Format(time.RFC3339Nano))
		}
		// write time, level and source to buf
		cW(buf, true, nGreen, "%s", timeAttr.Value.String())
		buf.WriteString(" ")
	}

	levelAttr := slog.Attr{
		Key:   slog.LevelKey,
		Value: slog.StringValue(r.Level.String()),
	}
	if h.opts.ReplaceAttr != nil {
		levelAttr = h.opts.ReplaceAttr([]string{}, levelAttr)
	}
	cW(buf, true, levelColor(r.Level), "%s", levelAttr.Value.String())
	buf.WriteString(" ")

	if h.opts.AddSource {
		file, line := r.SourceLine()
		cW(buf, true, nGreen, "%s:%d", file, line)
		buf.WriteString(" ")
	}

	// write message to buf
	cW(buf, true, nWhite, "%s", r.Message)
	buf.WriteString(" ")
	// write preformatted attrs to buf
	buf.Write(h.preformattedAttrs.Bytes())
	// close group in preformatted attrs if open\
	if h.groupOpen {
		cW(h.preformattedAttrs, true, nWhite, "%s", "}")
	}
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
	case slog.StringKind:
		cW(w, true, nCyan, "%q", value.String())
	case slog.BoolKind:
		cW(w, true, nCyan, "%t", value.Bool())
	case slog.Int64Kind:
		cW(w, true, nCyan, "%d", value.Int64())
	case slog.DurationKind:
		cW(w, true, nCyan, "%s", value.Duration().String())
	case slog.Float64Kind:
		cW(w, true, nCyan, "%f", value.Float64())
	case slog.TimeKind:
		cW(w, true, nCyan, "%s", value.Time().Format(time.RFC3339))
	case slog.Uint64Kind:
		cW(w, true, nCyan, "%d", value.Uint64())
	case slog.GroupKind:
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
