# Constraints in depth

Everything in CUE is a constraint; this page covers forms agents misuse most.

## Field presence: optional, required, regular

| Syntax | Meaning |
|--------|---------|
| `a: T` | If `a` is present, its value must satisfy `T`. Presence often implied by the enclosing struct’s use. |
| `a?: T` | **Optional:** field may be absent; if present, value must satisfy `T`. |
| `a!: T` | **Required:** field must exist (and satisfy `T`) for the struct to be valid in contexts that demand it. |

Informal subsumption among field modes (spec details refine this):

```
{a: x}  ⊑  {a!: x}  ⊑  {a?: x}
```

More specific (left) satisfies more permissive (right) requirements.

```cue
#NeedsName: {
	name!: string
	nick?: string
}

// ok
#NeedsName & {name: "sam"}

// bottom: missing required
#NeedsName & {}
```

## Closed vs open structs

- **Open** (default for ordinary structs): extra fields allowed unless constrained away.
- **Closed**: no fields beyond those declared (and pattern/ellipsis rules).

Definitions (`#Foo`) are **closed by default**. Ordinary `{…}` literals are
typically open unless you `close()` or use ellipsis/`#` patterns that restrict.

```cue
open: {
	a: int
}
open & {a: 1, b: 2}     // often ok: extra field b allowed

#Closed: {
	a: int
}
#Closed & {a: 1, b: 2}  // _|_ : b not allowed
```

Ellipsis for “rest” fields:

```cue
#Template: {
	id: string
	...                   // allow additional fields (open tail)
}

#Strictish: {
	id: string
	...string             // additional fields must be string
}
```

## Pattern constraints (bulk fields)

Apply one constraint to many labels matching a pattern:

```cue
#Labels: {
	[string]: string              // any field name -> string value
}

#Env: {
	[=~"^APP_"]: string           // fields matching regex
	PORT?: int
}

#Mixed: {
	known: int
	[_]:   _                      // other fields: any value
}
```

Pattern fields interact with closedness: they declare *allowed* extra shapes
rather than enumerating every key.

## Defaults and incomplete values

Incomplete values are normal during authoring:

```cue
server: {
	host: string
	port: *8080 | int
}
```

A consumer that needs concrete output (export, apply) must eventually supply or
default everything required. Unification with more specific data fills holes:

```cue
server & {host: "localhost"}
// {host: "localhost", port: 8080}  with default applied
```

## Disjunctions as tagged / sum shapes

Prefer **discriminators** (a field that differs per branch) so unification can
pick one branch cleanly:

```cue
#Event: {
	#Login:  {kind: "login", user: string}
	#Logout: {kind: "logout", at: string}
	#Login | #Logout
}

// picks #Login branch
#Event & {kind: "login", user: "ada"}
```

Without discriminators, large disjunctions may stay incomplete or explode
combinatorially when unified.

## Embedding and constraints

Structs can embed other structs/constraints; fields merge via unification.
Embedding is useful for mixin-style schemas:

```cue
#Meta: {
	labels?: {[string]: string}
}

#Service: {
	#Meta
	name: string
}
```

## Hidden fields and definitions as encapsulation

- `_internal: T` — implementation detail; not part of normal external shape.
- `#Schema` — schema-only; reference explicitly when applying.

Use these to keep exported concrete configs clean while still validating.

## Bottom as failed constraint (debugging mindset)

When evaluation fails, think **which conjuncts conflict**, not “type error at
line X” in a classical sense:

1. Same field, incompatible values (`port: 80` vs `port: 443`).
2. Value outside bounds (`port: 70000` vs `<=65535`).
3. Extra field in closed struct.
4. Missing required field (`name!`).
5. Wrong branch of disjunction (no branch accepts the data).
6. Regex / match failure.

Fix by relaxing the schema, correcting the data, or splitting disjuncts so the
intended branch matches.

## Comprehensions as constrained generation

Comprehensions produce values that must still satisfy outer constraints:

```cue
#Ports: [...int & >=1 & <=65535]

ports: #Ports & [for p in serviceList {p.port}]
```

If any generated element violates the element constraint, the whole unify fails.

## What not to expect

- No separate subtype declaration syntax beyond lattice order / definitions.
- No nominal classes: two structs with the same fields are compatible if they
  unify, regardless of “name” (definitions add reference/closing behavior, not
  Java-style nominal typing).
- No runtime reflection API in pure language core; tooling and builtins provide
  limited introspection (`len`, etc.).
