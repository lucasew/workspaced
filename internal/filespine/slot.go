package filespine

import (
	"errors"
	"fmt"
	"io"
	"os"

	"cuelang.org/go/cue"
)

const (
	KindText = "text"
	KindRef  = "ref"
)

var (
	ErrUnknownSlotKind = errors.New("unknown slot kind")
	ErrInvalidSlot     = errors.New("invalid slot")
)

// Slot is one values entry. Bare CUE strings become KindText.
type Slot struct {
	Kind string
	Text string
	Ref  string
}

func (s Slot) writeTo(w io.Writer) error {
	switch s.Kind {
	case KindText:
		_, err := io.WriteString(w, s.Text)
		return err
	case KindRef:
		f, err := os.Open(s.Ref)
		if err != nil {
			return fmt.Errorf("ref %q: %w", s.Ref, err)
		}
		_, copyErr := io.Copy(w, f)
		closeErr := f.Close()
		return errors.Join(copyErr, closeErr)
	default:
		return fmt.Errorf("%w: %s", ErrUnknownSlotKind, s.Kind)
	}
}

func (s Slot) equal(o Slot) bool {
	return s.Kind == o.Kind && s.Text == o.Text && s.Ref == o.Ref
}

func parseSlot(v cue.Value) (Slot, error) {
	if err := v.Err(); err != nil {
		return Slot{}, err
	}
	switch v.Kind() {
	case cue.StringKind:
		text, err := v.String()
		if err != nil {
			return Slot{}, err
		}
		return Slot{Kind: KindText, Text: text}, nil
	case cue.StructKind:
		kindVal := v.LookupPath(cue.ParsePath("kind"))
		kind, err := kindVal.String()
		if err != nil {
			return Slot{}, fmt.Errorf("%w: missing kind", ErrInvalidSlot)
		}
		switch kind {
		case KindText:
			text, err := v.LookupPath(cue.ParsePath("text")).String()
			if err != nil {
				return Slot{}, fmt.Errorf("slot kind text: %w", err)
			}
			return Slot{Kind: KindText, Text: text}, nil
		case KindRef:
			ref, err := v.LookupPath(cue.ParsePath("ref")).String()
			if err != nil {
				return Slot{}, fmt.Errorf("slot kind ref: %w", err)
			}
			return Slot{Kind: KindRef, Ref: ref}, nil
		default:
			return Slot{}, fmt.Errorf("%w: %s", ErrUnknownSlotKind, kind)
		}
	default:
		return Slot{}, fmt.Errorf("%w: want string or struct, got %s", ErrInvalidSlot, v.Kind())
	}
}
