package api

import (
	"context"
	"errors"
	"image"
)

// ErrDriverNotFound is returned by Get for unknown palette driver names.
var ErrDriverNotFound = errors.New("palette driver not found")

// LAB represents a color in CIELAB color space.
// L is lightness [0-100]; A is green-red; B is blue-yellow.
type LAB struct {
	L, A, B float64
}

// Polarity represents color scheme preference.
type Polarity int

const (
	PolarityAny Polarity = iota
	PolarityDark
	PolarityLight
)

// Options configures palette extraction.
type Options struct {
	Polarity   Polarity
	ColorCount int // 16 for base16, 24 for base24
	MaxSamples int // Limit pixels to sample (0 = all)
}

// Palette is a base16 scheme with optional base24 extensions (Base10-Base17).
// Hex fields are lowercase RRGGBB without a leading '#'.
type Palette struct {
	Base00 string `json:"base00"`
	Base01 string `json:"base01"`
	Base02 string `json:"base02"`
	Base03 string `json:"base03"`
	Base04 string `json:"base04"`
	Base05 string `json:"base05"`
	Base06 string `json:"base06"`
	Base07 string `json:"base07"`
	Base08 string `json:"base08"`
	Base09 string `json:"base09"`
	Base0A string `json:"base0A"`
	Base0B string `json:"base0B"`
	Base0C string `json:"base0C"`
	Base0D string `json:"base0D"`
	Base0E string `json:"base0E"`
	Base0F string `json:"base0F"`
	Base10 string `json:"base10,omitempty"`
	Base11 string `json:"base11,omitempty"`
	Base12 string `json:"base12,omitempty"`
	Base13 string `json:"base13,omitempty"`
	Base14 string `json:"base14,omitempty"`
	Base15 string `json:"base15,omitempty"`
	Base16 string `json:"base16,omitempty"`
	Base17 string `json:"base17,omitempty"`
}

// Driver extracts color palettes from images.
type Driver interface {
	// Name is the CLI slug used with --driver (e.g. "genetic", "materialyou").
	Name() string
	// Description is a short summary shown by `palette drivers`.
	Description() string
	Extract(ctx context.Context, img image.Image, opts Options) (*Palette, error)
}
