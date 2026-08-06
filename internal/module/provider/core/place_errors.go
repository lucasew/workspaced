package core

import "errors"

// place step / path errors (tabled for %w + errors.Is).
var (
	errPlaceMissingOp         = errors.New("missing op")
	errPlaceUnknownOp         = errors.New("unknown op")
	errPlaceRequireNoPatterns = errors.New("require needs patterns")
	errPlaceEmptyPattern      = errors.New("empty pattern")
	errPlaceEmptyNegation     = errors.New("empty after !")
	errPlaceRequireNoMatch    = errors.New("no path matched")
	errPlaceRequireMustNot    = errors.New("pattern must not match")
	errPlaceBadPattern        = errors.New("invalid pattern")
	errPlaceEmptyPath         = errors.New("empty path")
	errPlacePathEscape        = errors.New("path escapes origin")
	errPlaceMoveFrom          = errors.New("move from path")
	errPlaceMoveTo            = errors.New("move to path")
)
