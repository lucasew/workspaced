# Workspaced Development

This file consolidates project conventions and guidelines.

## Overview
User configs/dotfiles live in `workspaced.cue`. Templates use `{{ .Field }}` syntax.
- **⚠️ CRITICAL**: See `TEMPLATES.md` for complete template system documentation (5 types: static, simple, multi-file, index, .d.tmpl)

## Common Commands
- **Apply**: `workspaced apply` (builds Go code and applies configs)
- **Plan**: `workspaced plan` (dry-run, shows what would change)
- **Doctor**: `workspaced doctor` (check driver status)
  - Use `--verbose` flag to see full interface/provider paths

## Adding New Config Fields
When adding new config fields to `pkg/config/config.go`:
1. Add field to `GlobalConfig` struct with `json:"field_name"` tag
2. Create corresponding struct (e.g., `FooConfig`) with fields and tags
3. **CRITICAL**: Add `Merge()` method to new struct (see `PaletteConfig.Merge()` as example)
4. **CRITICAL**: Call merge in `GlobalConfig.Merge()`: `result.Foo = result.Foo.Merge(other.Foo)`
5. Add config section to `workspaced.cue`
6. Templates access via `{{ .Foo.Field }}`

**⚠️ IMPORTANT - Merge Methods:**
- LoadConfig() creates hardcoded defaults, then loads `workspaced.cue` and merges
- Without implementing `Merge()` and calling it in `GlobalConfig.Merge()`, the merge doesn't happen
- Result: values from `workspaced.cue` are ignored, templates generate empty fields
- Symptom: code compiles OK, config is read, but `{{ .Field }}` returns empty string
- Always implement Merge() for structs nested in GlobalConfig!

## CLI & Architecture
- **Intention-based Structure**: Commands are grouped by user intent:
  - `input`: User interaction (`text`, `confirm`, `choose`, `menu`).
  - `open`: Resource launching (`webapp`, `terminal`, generic URLs/files).
  - `system`: Hardware and session state (`audio`, `brightness`, `power`, `screen`).
  - `state`: Dotfiles lifecycle (`apply`, `plan`, `sync`, `doctor`).
- **Local-First**: CLI binary executes hardware/system logic locally whenever possible. Daemon handles shared state, tray, watchers, and cross-client coordination (OSD IDs).
- **Module System**:
  - Located in `modules/`. Atomic, parametric, and strictly unique (no claim collisions).
  - Uses `module.cue` for metadata and config validation.
  - **Zero-Intermediate**: Files are processed in-memory and streamed directly to targets.
- **Lazy Processing**: `source.File` interface delays content reading/rendering until strictly needed.
- **Strict Config**: No lists in module configs. Deep merge with zero substitution policy between different modules.
- **Top-level Aliases**: `sync`, `apply`, `plan`, and `open` are mirrored at root for ergonomics.
- **Tool providers**: Instead of scoping on tools, scope on registries. Ex: `uv` and `pip` shouldn't be backends, `pypi` and `pyx` should.

## Driver System
- Drivers provide platform-specific implementations for various features (audio, clipboard, notifications, etc.)
- Each driver implements a provider interface with:
  - `ID()`: Unique slug (e.g., "audio_pulse")
  - `Name()`: User-friendly name (e.g., "PulseAudio")
  - `DefaultWeight()`: Priority (0-100)
  - `CheckCompatibility()`: Verify if driver can run
  - `New()`: Create instance
- Use `workspaced doctor` to see all drivers and their status
- Configure driver weights in `workspaced.cue` under `workspaced.drivers`

## Runtime Guidelines
- **Network access**:
  - If an expected hash is available, use `fetchurl` driver.
  - If no expected hash is available, use `httpclient` driver.
  - Avoid direct `http.DefaultClient` usage outside driver implementations.
- **Process execution**:
  - Prefer `pkg/driver/exec` instead of direct `os/exec` in feature code.
  - Direct `os/exec` is acceptable only inside exec driver implementations.
- **Driver preload**:
  - Keep driver prelude import centralized in root command (`cmd/workspaced/root.go`).
  - Avoid duplicate `_ "workspaced/pkg/driver/prelude"` imports in subcommands.
- **Module lock model**:
  - `workspaced.cue`: declarative inputs, modules, and config.
  - `workspaced.lock.json`: resolved lock state (including source URL/hash and module version/source pins).
