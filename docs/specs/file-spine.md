# File spine

`workspaced.file` is the dest tree. Enabled modules may declare `module.file`
in `module.cue`; those maps unify into `workspaced.file`. Module templates
also lower into it. Apply reads a dest `fs.FS`. `Open(name)` returns the
combined file.

Implementation: `pkg/filespine`, schema in `internal/configcue/schema.cue`.

## Path

`fs.FS` name. No leading `/`. No `~`. No `..`.

| Command | `Open(".bashrc")` |
|---|---|
| `home apply` | `$HOME/.bashrc` |
| `codebase apply` | `<repo>/.bashrc` |

## File types

| `type` | After unify | Encode |
|---|---|---|
| `lines` | all `values` keys | sort keys, join with `\n` |
| `text` | exactly one key | that slot |
| `ref` | exactly one key, kind `ref` | bytes from the source path |
| `json` / `toml` / `yaml` / `ini` | `values` is a map | marshal that map |

Same path, different `type` → error.

## Slots

```cue
#Slot: string | #SlotText | #SlotRef
#SlotText: close({kind: "text", text: string})
#SlotRef:  close({kind: "ref",  ref: string})
```

Bare string is text. Structs need `kind`. Same key must unify.

There is no `runtime.env` and no `{kind: "env"}`.

## Module `file`

In `module.cue` (`package module`):

```cue
module: file: {
	".config/foo.toml": {
		type: "toml"
		values: {theme: "base16"}
	}
}
```

`module.file` can read `workspaced.*` (runtime, other module config). Only
enabled modules contribute. Host `workspaced.file` still works and unifies
with the same keys.

## Lowering

| Source | Slot |
|---|---|
| `.bashrc.d.tmpl/20-alias.sh` | `file.".bashrc"` `lines` / `values."20-alias.sh"` |
| `.bashrc.tmpl` | `file.".bashrc"` `text` / `values.content` |
| static file | `file."<path>"` `ref` / `values.src` |

CUE can add or override the same key. Go walks the CUE value. Dest `Open`
does not take a slot key.

Structured types (`json`, `toml`, `yaml`, `ini`) take a map in `values` and
write that map. Root lists are not allowed. `ini` allows one section level
(`values.core.bare = true` → `[core]\nbare = true`). A `.json.tmpl` on disk
still lowers as `text`, not as `json`.

## Example

```cue
workspaced: file: ".bashrc": {
	type: "lines"
	values: {
		"00-umask": "umask 022"
		"10-path":  {kind: "text", text: "export PATH=$HOME/bin:$PATH"}
	}
}

workspaced: file: ".config/foo.json": {
	type: "json"
	values: {
		port: 8080
		name: "foo"
	}
}
```

## Out of scope

- `file.home` vs `file.codebase` namespaces
- writable dest FS
- `runtime.env`
