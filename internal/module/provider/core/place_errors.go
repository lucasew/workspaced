package core

import "errors"

// place step / path errors (tabled for %w + errors.Is).
var (
	errPlaceRequireNoMatch = errors.New("no path matched")
	errPlaceRequireMustNot = errors.New("pattern must not match")
	errPlaceBadPattern     = errors.New("invalid pattern")
	errPlaceEmptyPath      = errors.New("empty path")
	errPlacePathEscape     = errors.New("path escapes origin")
	errPlaceMoveFrom       = errors.New("move from path")
	errPlaceMoveTo         = errors.New("move to path")
)
