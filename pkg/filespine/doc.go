// Package filespine composes dest files from providers into an fs.FS.
// Open returns the combined file.
//
// Providers add full dest decls and/or keyed slots. Compose merges them.
//
// CUE hosts mount the dest schema with Mount or Constrain, then Parse
// the value at that path. Workspaced mounts at workspaced.file.
//
// Spec: docs/specs/file-spine.md.
package filespine
