// Package filespine composes dest files from providers into an fs.FS.
// Open returns the combined file.
//
// Providers add full dest decls and/or keyed slots. Compose merges them.
//
// Spec: docs/specs/file-spine.md.
package filespine
