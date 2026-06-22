# CUE and workspaced

## Stance

**CUE is a feature, not a cage.** Workspaced cares about the **evaluated**
configuration: modules enabled, inputs, driver weights, module `config` bags,
hosts, and anything else loaders decode. How you *author* that result is yours.

Outside-the-box is welcome: definitions (`#…`), comprehensions, shared fragments,
per-host overlays, factoring palettes/tools into reusable blocks, building maps
from lists **inside CUE** so the *output* is merge-friendly, embedding files,
splitting packages — whatever still evaluates to what workspaced understands.

**Prefer the user’s existing `workspaced.cue` as the local dialect.** Init’s
minimal example is a starter, not law.

This skill is not a CUE language tutorial. If you need CUE mechanics, use CUE
docs; here we only cover how workspaced meets CUE.

## What workspaced actually consumes

Conceptually:

1. Load cue (+ preludes/schema constraints in the program).
2. Evaluate/unify toward a config object.
3. Decode slices (modules, inputs, …) for apply, tools, drivers, etc.

So clever authoring is fine when the **result** still has the fields and shapes
those steps expect. Clever authoring that never appears under the keys workspaced
reads is just unused CUE.

Schema/preambles define **what the program knows how to interpret**. If you need
a brand-new top-level concept the schema doesn’t expose, that’s extending
workspaced (contributor territory), not “more creative cue” alone.

## Expressiveness patterns (encouraged when useful)

Examples of legitimate moves:

- Define a `#Module` or shared config struct and unify many modules with it.
- Comprehend `modules` or `hosts` from a smaller table you maintain once.
- Keep a list in CUE for *your* ergonomics, then project it to a **keyed struct**
  at the boundary workspaced merges (see merge note below).
- Per-host or per-environment overlays (`hosts`, tags, conditionals in cue).
- Centralize repeated `config` fragments (greeting defaults, feature flags) and
  reference them from modules.

There is no requirement that cue look flat or match init line-for-line.

## Hard edges (small set)

These are **runtime/merge/program** edges, not style preferences.

### 1. Merge-friendly module config

Workspaced composes configuration with **deep merge** in mind, especially across
modules/layers. **Naked lists at merge points** in module `config` tend to
replace or fight each other rather than compose — which is why the project
discourages list-shaped **module configs** specifically.

That does **not** mean “never use lists in CUE.” It means:

- If something is merged across contributors/layers and should accumulate, prefer
  **maps/objects with stable keys**, or
- Keep lists in your authoring layer and **emit a map** in the evaluated config
  workspaced sees.

Templates and ordinary cue data elsewhere can still use lists freely when they
aren’t at those merge boundaries.

### 2. Lock is not CUE

Creativity belongs in cue + module sources. `workspaced.lock.json` is **derived
pins** (and automation metadata). Hand-editing lock to “make it work” usually
fights the next `mod lock` and confuses reproducibility.

### 3. Right file, right command

Expressive cue in a file that this command never loads does nothing. See
`config-and-roots.md`.

### 4. Evaluated types still matter

CUE might allow very open structures; workspaced may still fail at decode/apply
if a field is the wrong kind for a consumer (e.g. module expects a string path).
That’s a contract issue, not “be less creative” — adjust the evaluated shape.

## Inspecting evaluated config

When the question is “what does workspaced *see*?” prefer program inspect paths
over guessing:

- `home config …` / `codebase config …` — get, eval, dump, layers (see `--help`)
- Read raw cue only as the authoring source; layers/eval show composition.

If inspect output doesn’t match your mental model, you likely have a root/merge
issue, not insufficient CUE cleverness.

## Minimal shapes (examples, not prescriptions)

Init-style module enablement (one possible form):

```cue
workspaced: {
	modules: {
		example: {
			input: "self"
			path:  "modules/example"
			config: {
				enable: true
			}
		}
	}
}
```

You might instead generate `modules` via comprehension from a list of names,
share `config` via definitions, or gate modules per host. All fine if evaluation
lands on the same *kind* of structure.

## Gotchas

- **Imposing init style on a mature cue file** — users often have non-trivial CUE;
  extend their patterns instead of rewriting to the tutorial shape.
- **“Lists are banned” overread** — only sensitive at **merged module config**
  (and similar merge points). Elsewhere, lists are normal CUE.
- **Schema != style guide** — schema limits what workspaced decodes; it doesn’t
  mean authoring must be minimal or non-DRY.
- **Evaluates in `cue eval`, missing in apply** — wrong root/key/command family,
  or field not part of workspaced’s decoded model.
- **Over-editing lock after cue creativity** — fix cue/modules, then `mod lock`;
  don’t treat lock as a second source of truth for intent.
- **Contributing vs using** — needing new schema fields/preambles is development;
  this skill stops at using existing config surface creatively.
