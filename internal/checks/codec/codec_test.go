package codec

import (
	"errors"
	"testing"
)

func TestDecodeUnknownCodec(t *testing.T) {
	_, err := Decode("not_a_codec", "tool", nil)
	if err == nil {
		t.Fatal("expected error for unknown codec")
	}
	if !errors.Is(err, ErrUnknownCodec) {
		t.Fatalf("errors.Is(err, ErrUnknownCodec) = false; err=%v", err)
	}
}
