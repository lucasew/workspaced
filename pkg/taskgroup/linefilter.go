package taskgroup

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

// One-line virtual terminal. CR, backspace, and in-line CSI rewrite the
// current row. Newline commits it. Row-moving sequences (cursor up/down,
// CUP, erase display) are dropped so a subprocess cannot walk off the line
// onto the progress bars.
const (
	lineMaxCols    = 1024
	lineMaxPending = 8192
	lineMaxCSINum  = 1_000_000
)

type lineFilter struct {
	pending []byte
	line    []rune
	col     int
	// afterCR: the next glyph starts a new live frame (replaces the row)
	// instead of overlaying from column 0. A lone \r\n is CRLF and still
	// commits the text written before the CR.
	afterCR bool
}

// write consumes raw terminal bytes. Complete lines (terminated by '\n')
// are returned without the delimiter. Current() is the uncommitted row.
func (f *lineFilter) write(p []byte) []string {
	if len(p) == 0 {
		return nil
	}
	f.pending = append(f.pending, p...)
	if len(f.pending) > lineMaxPending {
		// Hostile / unterminated OSC: drop the backlog rather than grow.
		f.pending = f.pending[len(f.pending)-lineMaxPending:]
	}

	var committed []string
	s := f.pending
	i := 0
	for i < len(s) {
		switch s[i] {
		case 0x1b:
			next, ok := f.consumeESC(s, i)
			if !ok {
				f.pending = s[i:]
				return committed
			}
			i = next
			continue
		case 0x9b:
			next, ok := f.consumeCSI(s, i+1)
			if !ok {
				f.pending = s[i:]
				return committed
			}
			i = next
			continue
		}

		r, size := utf8.DecodeRune(s[i:])
		if r == utf8.RuneError && size == 1 {
			if !utf8.FullRune(s[i:]) {
				f.pending = s[i:]
				return committed
			}
			i++
			continue
		}
		i += size
		if line := f.putRune(r); line != nil {
			committed = append(committed, *line)
		}
	}
	f.pending = f.pending[:0]
	return committed
}

func (f *lineFilter) current() string {
	return string(f.line)
}

func (f *lineFilter) take() string {
	s := string(f.line)
	f.reset()
	return s
}

func (f *lineFilter) reset() {
	f.line = f.line[:0]
	f.col = 0
	f.afterCR = false
}

func (f *lineFilter) putRune(r rune) *string {
	switch r {
	case '\n':
		return f.newline()
	case '\r':
		f.col = 0
		f.afterCR = true
	case '\b':
		if f.col > 0 {
			f.col--
		}
	case '\t':
		next := ((f.col / 8) + 1) * 8
		for f.col < next {
			f.put(' ')
		}
	case '\a':
		// bell
	default:
		if r < 0x20 || r == 0x7f {
			return nil
		}
		f.put(r)
	}
	return nil
}

func (f *lineFilter) newline() *string {
	s := string(f.line)
	f.reset()
	if s == "" {
		return nil
	}
	return &s
}

func (f *lineFilter) put(r rune) {
	if f.afterCR {
		f.line = f.line[:0]
		f.col = 0
		f.afterCR = false
	}
	if f.col >= lineMaxCols {
		return
	}
	if f.col < len(f.line) {
		f.line[f.col] = r
	} else {
		for len(f.line) < f.col {
			f.line = append(f.line, ' ')
		}
		f.line = append(f.line, r)
	}
	f.col++
}

// consumeESC starts at ESC. ok is false when the sequence is incomplete.
func (f *lineFilter) consumeESC(s []byte, i int) (int, bool) {
	if i+1 >= len(s) {
		return i, false
	}
	switch s[i+1] {
	case '[':
		return f.consumeCSI(s, i+2)
	case ']', 'P', 'X', '^', '_':
		return consumeStringTerm(s, i+2)
	case '(', ')', '*', '+', '#':
		if i+2 >= len(s) {
			return i, false
		}
		return i + 3, true
	default:
		return i + 2, true
	}
}

func (f *lineFilter) consumeCSI(s []byte, i int) (int, bool) {
	start := i
	for i < len(s) {
		c := s[i]
		if c >= 0x40 && c <= 0x7e {
			f.applyCSI(string(s[start:i]), c)
			return i + 1, true
		}
		if c >= 0x20 && c <= 0x3f {
			i++
			continue
		}
		return i + 1, true
	}
	return start, false
}

func (f *lineFilter) applyCSI(params string, final byte) {
	p := params
	for len(p) > 0 && (p[0] == '?' || p[0] == '>' || p[0] == '=' || p[0] == '!') {
		p = p[1:]
	}
	switch final {
	case 'm': // SGR — strip
	case 'K':
		f.eraseInLine(csiNum(p, 0))
	case 'G':
		n := csiNum(p, 1)
		f.col = clampInt(n-1, 0, lineMaxCols)
	case 'C':
		f.col = clampInt(f.col+csiNum(p, 1), 0, lineMaxCols)
	case 'D':
		f.col = clampInt(f.col-csiNum(p, 1), 0, lineMaxCols)
	default:
		// A/B/H/f/J and the rest leave the line: ignore.
	}
}

func (f *lineFilter) eraseInLine(n int) {
	switch n {
	case 0:
		if f.col < len(f.line) {
			f.line = f.line[:f.col]
		}
	case 1:
		for i := 0; i < f.col && i < len(f.line); i++ {
			f.line[i] = ' '
		}
	case 2:
		f.line = f.line[:0]
	}
}

func consumeStringTerm(s []byte, i int) (int, bool) {
	for i < len(s) {
		if s[i] == 0x07 {
			return i + 1, true
		}
		if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '\\' {
			return i + 2, true
		}
		i++
	}
	return 0, false
}

func csiNum(p string, def int) int {
	p = strings.TrimSpace(p)
	if p == "" {
		return def
	}
	if i := strings.IndexByte(p, ';'); i >= 0 {
		p = p[:i]
	}
	n, err := strconv.Atoi(p)
	if err != nil || n < 0 {
		return def
	}
	return min(n, lineMaxCSINum)
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	return min(v, hi)
}
