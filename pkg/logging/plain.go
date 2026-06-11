package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"
)

// colorEnabled reports whether we should emit ANSI color codes.
// Respects the NO_COLOR convention and disables on dumb terminals / CI /
// non-tty stderr.
func colorEnabled() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	if os.Getenv("CI") != "" {
		return false
	}
	fi, err := os.Stderr.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// levelLetter returns a single-character representation of the level.
func levelLetter(l slog.Level) string {
	switch {
	case l == slog.LevelDebug:
		return "D"
	case l == slog.LevelInfo:
		return "I"
	case l == slog.LevelWarn:
		return "W"
	case l >= slog.LevelError:
		return "E"
	default:
		return l.String()[:1]
	}
}

// coloredLevel returns the level letter with the level's color used as
// background. The foreground (the letter itself) is left as-is (terminal
// default / current fg color, possibly bolded for visibility), per the
// request for "filled" badges.
func coloredLevel(l slog.Level) string {
	switch {
	case l == slog.LevelDebug:
		// gray / bright black background (subtle)
		return "\x1b[100mD\x1b[0m"
	case l == slog.LevelInfo:
		// bright cyan background
		return "\x1b[46;1mI\x1b[0m"
	case l == slog.LevelWarn:
		// bright yellow background
		return "\x1b[43;1mW\x1b[0m"
	default:
		// bright red background
		return "\x1b[41;1mE\x1b[0m"
	}
}

// FormatPlain renders a slog.Record using the project's compact plain (no ANSI)
// log format. This is the single implementation of the "L msg key=val ..."
// style (single letter level) used when colors are disabled or when plain
// output is explicitly required (e.g. some snapshots).
//
// Prefer Format / FormatPrepend for normal use — they automatically pick the
// colored version (background-colored level letter + styled key=value) when
// the terminal supports it.
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

// Format renders a slog.Record using the project's formatter.
// When color is enabled (no NO_COLOR, real tty, etc.) it produces a single
// letter level indicator with the level color as background + colored
// key=value formatting. Otherwise it falls back to the plain single-letter
// format.
//
// This is the function used for normal direct logging and for logs that
// appear inside progress-bar (bubbletea) renderers so they stay consistent.
func Format(r slog.Record) string {
	if colorEnabled() {
		attrs := make([]slog.Attr, 0, r.NumAttrs())
		r.Attrs(func(a slog.Attr) bool {
			attrs = append(attrs, a)
			return true
		})
		return formatColored(r.Level, r.Message, attrs)
	}
	return FormatPlain(r)
}

// FormatPrepend is like Format, but inserts the provided attrs right after the
// level+message and before any attrs already present on the record. This is
// primarily used by the taskgroup recorder to inject the synthetic "task"
// attribute while still getting the correct (plain or colored) output.
func FormatPrepend(r slog.Record, pre ...slog.Attr) string {
	if len(pre) == 0 {
		return Format(r)
	}
	if colorEnabled() {
		all := make([]slog.Attr, 0, len(pre)+r.NumAttrs())
		all = append(all, pre...)
		r.Attrs(func(a slog.Attr) bool {
			all = append(all, a)
			return true
		})
		return formatColored(r.Level, r.Message, all)
	}
	return FormatPlainPrepend(r, pre...)
}

// formatPlain is the core string builder used by both FormatPlain variants
// and by PlainHandler (non-color path).
func formatPlain(level slog.Level, msg string, attrs []slog.Attr) string {
	var b strings.Builder
	b.WriteString(levelLetter(level))
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

// formatColored is the colorful version used by PlainHandler when
// colorEnabled() is true. It produces a single-letter level indicator
// with ANSI colors + colored key=value formatting.
func formatColored(level slog.Level, msg string, attrs []slog.Attr) string {
	var b strings.Builder
	b.WriteString(coloredLevel(level))
	if msg != "" {
		b.WriteByte(' ')
		b.WriteString(msg)
	}
	for _, a := range attrs {
		appendColoredLogAttr(&b, a)
	}
	return b.String()
}

func appendColoredLogAttr(b *strings.Builder, a slog.Attr) {
	a.Value = a.Value.Resolve()
	if a.Equal(slog.Attr{}) {
		return
	}
	if a.Value.Kind() == slog.KindGroup {
		subs := a.Value.Group()
		if len(subs) == 0 {
			return
		}
		for _, sub := range subs {
			key := a.Key
			if key != "" {
				key = key + "." + sub.Key
			} else {
				key = sub.Key
			}
			appendColoredLogAttr(b, slog.Attr{Key: key, Value: sub.Value})
		}
		return
	}

	// Colored key=value: dim cyan key + dim = + normal value
	b.WriteString(" \x1b[2;36m")
	b.WriteString(a.Key)
	b.WriteString("\x1b[0m\x1b[2m=\x1b[0m")
	b.WriteString(formatPlainLogValue(a.Value))
}

// PlainHandler is a slog.Handler that emits records using the project's
// compact log format. When color is enabled (no NO_COLOR, real tty, etc.)
// it renders a single colored letter for the level + pretty key=value pairs.
// Otherwise it falls back to the plain single-letter format.
type PlainHandler struct {
	w        io.Writer
	opts     slog.HandlerOptions
	pre      []slog.Attr // accumulated from WithAttrs (and simple group markers)
	useColor bool
}

// NewPlainHandler returns a handler that writes compact log lines to w.
// Color is auto-detected (disabled when NO_COLOR is set, TERM=dumb, CI,
// or stderr is not a terminal).
func NewPlainHandler(w io.Writer, opts *slog.HandlerOptions) *PlainHandler {
	if opts == nil {
		opts = &slog.HandlerOptions{}
	}
	return &PlainHandler{
		w:        w,
		opts:     *opts,
		useColor: colorEnabled(),
	}
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

	var line string
	if h.useColor {
		line = formatColored(r.Level, r.Message, attrs)
	} else {
		line = formatPlain(r.Level, r.Message, attrs)
	}
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
		w:        h.w,
		opts:     h.opts,
		pre:      make([]slog.Attr, len(h.pre)+len(attrs)),
		useColor: h.useColor,
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
