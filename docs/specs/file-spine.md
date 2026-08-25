# File spine

`workspaced.file` is the dest tree. Module templates lower into it. Apply
reads a dest `fs.FS`. `Open(name)` returns the combined file.

Implementation: `internal/filespine`, schema in `internal/configcue/schema.cue`.

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

Same path, different `type` → error.

## Slots

```cue
#Slot: string | #SlotText | #SlotRef
#SlotText: close({kind: "text", text: string})
#SlotRef:  close({kind: "ref",  ref: string})
```

Bare string is text. Structs need `kind`. Same key must unify.

There is no `runtime.env` and no `{kind: "env"}`.

## Lowering

| Source | Slot |
|---|---|
| `.bashrc.d.tmpl/20-alias.sh` | `file.".bashrc"` `lines` / `values."20-alias.sh"` |
| `.bashrc.tmpl` | `file.".bashrc"` `text` / `values.content` |
| static file | `file."<path>"` `ref` / `values.src` |

CUE can add or override the same key. Go walks the CUE value. Dest `Open`
does not take a slot key.

## Example

```cue
workspaced: file: ".bashrc": {
	type: "lines"
	values: {
		"00-umask": "umask 022"
		"10-path":  {kind: "text", text: "export PATH=$HOME/bin:$PATH"}
	}
}
```

## Out of scope

- `json` / `toml` file types
- `file.home` vs `file.codebase` namespaces
- writable dest FS
- `runtime.env`
