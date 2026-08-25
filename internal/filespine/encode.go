package filespine

import (
	"bytes"
	"fmt"
	"io"
)

// Encode writes the combined dest bytes for one file.
func Encode(f File) ([]byte, error) {
	if err := f.checkArity(); err != nil {
		return nil, fmt.Errorf("file %q: %w", f.Path, err)
	}
	var buf bytes.Buffer
	switch f.Type {
	case TypeLines:
		keys := f.keys()
		for i, key := range keys {
			if i > 0 {
				if err := buf.WriteByte('\n'); err != nil {
					return nil, err
				}
			}
			if err := f.Values[key].writeTo(&buf); err != nil {
				return nil, fmt.Errorf("file %q values.%s: %w", f.Path, key, err)
			}
		}
	case TypeText, TypeRef:
		if err := writeOnlySlot(&buf, f); err != nil {
			return nil, fmt.Errorf("file %q: %w", f.Path, err)
		}
	default:
		return nil, fmt.Errorf("file %q: %w: %s", f.Path, ErrUnknownFileType, f.Type)
	}
	return buf.Bytes(), nil
}

func writeOnlySlot(w io.Writer, f File) error {
	for _, key := range f.keys() {
		return f.Values[key].writeTo(w)
	}
	return fmt.Errorf("%w: got 0", ErrTextKeyCount)
}
