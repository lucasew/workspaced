package compat

import "errors"

// ErrIncompatible marks a component as not applicable/compatible in the current environment.
var ErrIncompatible = errors.New("component is incompatible")
