# Types and values

In CUE, “types” are ordinary values higher in the lattice. Predeclared kinds
subsume their concrete members.

## Predeclared kinds (values)

| Kind | Role |
|------|------|
| `_` | Top — any value |
| `_|_` | Bottom — error / unsatisfiable |
| `null` | JSON null (keyword value) |
| `bool` | `true` / `false` |
| `int` | Integers (arbitrary precision in model) |
| `float` | Floating-point |
| `number` | Supertype of `int` and `float` |
| `string` | UTF-8 text |
| `bytes` | Byte sequences (single-quoted literals) |

Keywords `null`, `true`, `false` are **values**, not field references (JSON
compat). Other keywords (`package`, `import`, `for`, …) are contextual; this
skill ignores `package`/`import`.

## Literals (quick)

```cue
// ints
42
0xFF
0o755
0b1010
1_000_000
1.5Gi          // SI/IEC multipliers (truncated toward zero when fractional)

// floats
3.14
1e-9

// strings / bytes
"hello"
'multi\xffbyte'
"""
multiline
string
"""
#"raw \(not interpolated)"#
#"interpolated \#(x)"#

// interpolation
"user=\(user) host=\(host)"
```

Comments: `//` to end of line. Commas are often optional at line ends (spec
auto-insert rules); list elements still need separators when on one line.

## Identifiers and labels

- Normal field: `name`, `foo_bar`, `$x`
- **Definition**: starts with `#` — e.g. `#Server` — special scoping/closing
- **Hidden**: starts with `_` or `_#` — not exported / not part of regular
  concrete output in the usual sense
- Double-underscore `__…` reserved as keywords for implementations

Labels may be identifiers, strings, or expressions in parentheses / brackets
(computed labels).

## Structs

Only composite builder for complex values. Map from labels to values.

```cue
{
	name: string
	port: int
	meta: {
		owner: string
	}
}
```

**Unification merges fields** by name; same field unifies values.

**Subsumption (informal):** struct `A` is an instance of struct `B` if for every
regular field in `B`, `A` has a corresponding field whose value is an instance
of `B`’s field value (plus rules for optional/required/closed — see
`constraints.md`).

Embedding / selectors: `.field`, `"field"`, optional chaining patterns in
expressions; references resolve in lexical/definition scopes (host may add
extra scopes).

## Lists

Syntactic sugar over a cons-like structure; in the model, lists unify elementwise
and support open tails.

```cue
[1, 2, 3]
[string]: [string]     // not valid alone; element constraints via:
[...string]            // open list of strings
[int, ...string]       // first int, then zero+ strings
[int, int, ...int]     // at least two ints, maybe more
```

Open list marker `...` allows additional elements. Closed lists have fixed
length/elements and reject extra items when unified with longer lists.

```cue
[1, 2] & [1, 2, 3]     // _|_ if first list is closed (typical list lit is closed)
[1, 2, ...] & [1, 2, 3] // ok: [1, 2, 3]
```

## Bounds and match constraints

Bounds are infinite disjunctions of allowed atoms / patterns.

```cue
// numeric / order
>=0
>10 & <100
!=0

// equality-style
==1
!="disabled"

// regex on strings
=~"^[a-z]+$"
!~"password"

// combine with kinds
int & >=0 & <=65535
string & =~"^https://"
```

Failed bounds yield bottom.

## Definitions (`#Name`)

Reusable, usually **closed** schemas. Definitions participate in a separate
reference namespace from normal fields.

```cue
#Endpoint: {
	url:  string & =~"^https?://"
	port: int & >=1 & <=65535
}

api: #Endpoint & {
	url:  "https://example.com"
	port: 443
}
```

Hidden definitions `_#Name` combine definition and hidden rules.

Refer to definitions by name in the appropriate scope; they do not appear as
ordinary data fields in exported concrete structs unless explicitly referenced.

## Aliases

`X=expr` or field aliases bind a local name for disambiguation / self-reference
patterns (see spec for exact forms). Useful inside comprehensions and field
declarations that need to mention the field’s value.

## Operators (semantic groups)

| Group | Ops | Notes |
|-------|-----|-------|
| Unify / disjunct | `&`, `\|` | Core lattice ops |
| Logic | `&&`, `\|\|`, `!` | Boolean; also used in conditions |
| Compare | `==`, `!=`, `<`, `<=`, `>`, `>=` | Produce bool or constraints (context) |
| Match | `=~`, `!~` | Regex |
| Arithmetic | `+`, `-`, `*`, `/` | Numbers; `+` also concat for lists/strings in contexts |
| Default | `*` | Prefix on disjunct term |
| Selector | `.`, `[]` | Field / index access |
| Call | `f(x)` | Builtins and user functions (tooling/host) |

Precedence follows the spec; when unsure, parenthesize — especially around
`&` / `|` mixes.

## Builtins (common)

Exact set is implementation/version dependent. Frequently used in constraints:

- `len(x)` — length of string/bytes/list/struct (regular fields)
- `close(s)` — close a struct (no extra fields)
- `and([a,b,…])` / `or([a,b,…])` — bulk unify / disjunct
- Math, time, path, encoding builtins in standard CUE — verify with `cue help`
  / docs for the version you target.

Host programs (workspaced, custom loaders) may inject additional constraints
or builtins; treat those as environment-specific.

## JSON / data interop mental note

CUE values that are fully concrete and use only regular fields correspond
closely to JSON (plus CUE’s richer numbers/bytes). Incomplete values are
schemas/templates, not exportable as final data without defaults or more input.
