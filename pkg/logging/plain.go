package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"
)

// FormatPlain renders a slog.Record using the project's compact plain log
// format. This is the single implementation of the "LEVEL msg key=val ..."
// style used for task logs, bubbletea output, and direct logging.
//
// The format intentionally omits timestamps (they are rarely useful when
// logs are already correlated with task names or command transcripts).
func FormatPlain(r slog.Record) string {
	attrs := make([]slog.Attr, 0, r.NumAttrs())
	r.Attrs(func(a slog.Attr) bool {
		attrs = append(attrs, a)
		return true
	})
	return formatPlain(r.Level, r.Message, attrs)
}

// FormatPlainPrepend is like FormatPlain, but inserts the provided attrs right
// after the level+message and before any attrs already present on the record.
// This is used to emulate contributions from handler-level WithAttrs (e.g. the
// synthetic "task" attribute) for the plain string path while leaving the
// delegate handler unaffected.
func FormatPlainPrepend(r slog.Record, pre ...slog.Attr) string {
	if len(pre) == 0 {
		return FormatPlain(r)
	}
	all := make([]slog.Attr, 0, len(pre)+r.NumAttrs())
	all = append(all, pre...)
	r.Attrs(func(a slog.Attr) bool {
		all = append(all, a)
		return true
	})
	return formatPlain(r.Level, r.Message, all)
}

// formatPlain is the core string builder used by both FormatPlain variants
// and by PlainHandler.
func formatPlain(level slog.Level, msg string, attrs []slog.Attr) string {
	var b strings.Builder
	b.WriteString(level.String())
	if msg != "" {
		b.WriteByte(' ')
		b.WriteString(msg)
	}
	for _, a := range attrs {
		appendPlainLogAttr(&b, a)
	}
	return b.String()
}

func appendPlainLogAttr(b *strings.Builder, a slog.Attr) {
	a.Value = a.Value.Resolve()
	if a.Equal(slog.Attr{}) {
		return
	}
	if a.Value.Kind() == slog.KindGroup {
		subs := a.Value.Group()
		if len(subs) == 0 {
			// Bare group marker (e.g. from WithGroup with no children at emit time).
			// Emit nothing for flat output; grouping will have affected how the
			// frontend delivered subsequent attrs into the record.
			return
		}
		for _, sub := range subs {
			key := a.Key
			if key != "" {
				key = key + "." + sub.Key
			} else {
				key = sub.Key
			}
			appendPlainLogAttr(b, slog.Attr{Key: key, Value: sub.Value})
		}
		return
	}
	b.WriteByte(' ')
	b.WriteString(a.Key)
	b.WriteByte('=')
	b.WriteString(formatPlainLogValue(a.Value))
}

func formatPlainLogValue(v slog.Value) string {
	v = v.Resolve()
	switch v.Kind() {
	case slog.KindString:
		return quotePlainLogString(v.String())
	case slog.KindBool:
		return strconv.FormatBool(v.Bool())
	case slog.KindInt64:
		return strconv.FormatInt(v.Int64(), 10)
	case slog.KindUint64:
		return strconv.FormatUint(v.Uint64(), 10)
	case slog.KindFloat64:
		return strconv.FormatFloat(v.Float64(), 'g', -1, 64)
	case slog.KindGroup:
		// Should not normally be reached (groups are handled in appendPlainLogAttr),
		// but fall back to a compact representation.
		gs := v.Group()
		parts := make([]string, 0, len(gs))
		for _, sub := range gs {
			parts = append(parts, sub.Key+"="+formatPlainLogValue(sub.Value))
		}
		return "{" + strings.Join(parts, " ") + "}"
	default:
		return quotePlainLogString(fmt.Sprint(v.Any()))
	}
}

func quotePlainLogString(s string) string {
	if s == "" || strings.ContainsAny(s, " \t\r\n=\"") {
		return strconv.Quote(s)
	}
	return s
}

// PlainHandler is a slog.Handler that emits records using the project's
// compact plain format (via FormatPlain / formatPlain). Using it for the
// root logger makes direct logger.Info calls (self-update, self-install,
// etc.) produce the same style of output as logs coming from task execution.
type PlainHandler struct {
	w    io.Writer
	opts slog.HandlerOptions
	pre  []slog.Attr // accumulated from WithAttrs (and simple group markers)
}

// NewPlainHandler returns a handler that writes compact plain log lines to w.
// Only the Level field from opts is fully observed today; ReplaceAttr is
// applied to attributes (groups path is passed as nil).
func NewPlainHandler(w io.Writer, opts *slog.HandlerOptions) *PlainHandler {
	if opts == nil {
		opts = &slog.HandlerOptions{}
	}
	return &PlainHandler{w: w, opts: *opts}
}

func (h *PlainHandler) Enabled(_ context.Context, l slog.Level) bool {
	if h.opts.Level != nil {
		return l >= h.opts.Level.Level()
	}
	return l >= slog.LevelInfo
}

func (h *PlainHandler) Handle(ctx context.Context, r slog.Record) error {
	if !h.Enabled(ctx, r.Level) {
		return nil
	}

	// pre (from WithAttrs/WithGroup on the handler) then record attrs.
	attrs := make([]slog.Attr, 0, len(h.pre)+r.NumAttrs())
	attrs = append(attrs, h.pre...)
	r.Attrs(func(a slog.Attr) bool {
		attrs = append(attrs, a)
		return true
	})

	if h.opts.ReplaceAttr != nil {
		for i := range attrs {
			if rep := h.opts.ReplaceAttr(nil, attrs[i]); !rep.Equal(slog.Attr{}) {
				attrs[i] = rep
			} else {
				attrs[i] = slog.Attr{}
			}
		}
		// compact away dropped ones
		j := 0
		for i := range attrs {
			if !attrs[i].Equal(slog.Attr{}) {
				attrs[j] = attrs[i]
				j++
			}
		}
		attrs = attrs[:j]
	}

	line := formatPlain(r.Level, r.Message, attrs)
	if h.w != nil {
		_, err := fmt.Fprintln(h.w, line)
		return err
	}
	return nil
}

func (h *PlainHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}
	nh := &PlainHandler{
		w:    h.w,
		opts: h.opts,
		pre:  make([]slog.Attr, len(h.pre)+len(attrs)),
	}
	copy(nh.pre, h.pre)
	copy(nh.pre[len(h.pre):], attrs)
	return nh
}

func (h *PlainHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	// Insert a group marker. Real per-call grouped attrs are delivered by the
	// slog.Logger frontend into the Record; this marker mainly affects direct
	// handler wrapping. appendPlainLogAttr will render children with "name." prefix
	// if any grouped attrs end up here.
	return h.WithAttrs([]slog.Attr{slog.Group(name)})
}
