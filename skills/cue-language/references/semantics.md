# Semantics (lattice, unify, disjunct, defaults)

Derived from the CUE spec *Values* / *Expressions* sections. Notation `⊑`
(subsumption / “is instance of”) is **not** CUE syntax; it is lattice order.

## Lattice

All values form a **partial order** with:

| Element | CUE | Role |
|---------|-----|------|
| Top | `_` | Most general; every value is an instance of `_` |
| Bottom | `_|_` | Least element; error / conflict; instance of everything |
| Atoms | `42`, `"x"`, `true`, `null`, … | Only instances: itself and `_|_` |
| Types-as-values | `int`, `string`, `bool`, … | Subsume their concrete members |

Examples of order (informal):

```
false ⊑ bool ⊑ _
5 ⊑ int ⊑ number ⊑ _
5.0 ⊑ float ⊑ number ⊑ _
_|_ ⊑ x ⊑ _     for any x
```

Incomparable when neither subsumes the other (`int` vs `bool`).

Concrete value: an atom, or a struct whose regular fields are recursively
concrete. Incomplete values (open types, unresolved refs, incomplete lists)
are valid as constraints but not fully “ground.”

## Unification (`&`) (meet)

`a & b` is the **most specific** value that is an instance of both `a` and `b`.

Properties (order-independent merging):

- Commutative, associative, idempotent: `a & a == a`
- If `a ⊑ b` then `a & b == a`
- `a & _|_ == _|_`
- Distributes over disjunction: `(a0 | a1) & b == (a0&b) | (a1&b)`

Practically: composing constraints, merging structs, tightening bounds.

```cue
// Struct unify merges fields
{a: 1} & {b: 2}        // {a: 1, b: 2}
{a: int} & {a: 5}      // {a: 5}
{a: 1} & {a: 2}        // _|_ (conflict)

// Type & value
int & 5                // 5
string & 5             // _|_
```

## Disjunction (`|`) (join)

`a | b` is the **least upper bound**: alternatives / sum type.

Properties:

- Commutative, associative, idempotent: `a | a == a`
- If `a ⊑ b` then `a | b == b`
- `a | _|_ == a`

Normalized disjunctions drop redundant terms when one subsumes another.

```cue
"tcp" | "udp"
int | string
{kind: "a", x: int} | {kind: "b", y: string}
```

## Defaults (`*` inside disjunctions)

A default marks a preferred alternative when the overall value is still
incomplete. Written as `*term` among disjuncts.

```cue
port: *8080 | int        // prefer 8080 if nothing more specific
proto: *"tcp" | "udp"
```

Rewrite intuition (spec has precise U/D/M rules):

- Unifying with something compatible with the default can **select** it.
- Unifying with something incompatible with the default but compatible with
  another disjunct **drops** the defaulted term.
- Once fully resolved, the `*` marker is gone; only the chosen value remains.

Do not overuse defaults: they are for “fill in when unspecified,” not for
masking real conflicts.

## Bottom and errors

`_|_` propagates through most operations. Common causes:

- Contradictory values (`1 & 2`, `int & string`)
- Failed bounds (`5 & >=10`)
- Required field missing when demanded by a closed/required schema
- Certain illegal cycles

Tooling often surfaces an **error message** attached to bottom; conceptually it
is still bottom in the lattice.

## Cycles

CUE allows **structural** and **reference** cycles under defined evaluation
rules (fixed-point / incomplete values). Practical guidance:

- Recursive schemas via definitions (`#List: {head: _, tail: null | #List}`) are normal.
- Cyclic *concrete* data that cannot converge becomes incomplete or bottom.
- Prefer acyclic concrete configs; use cycles mainly in schemas and generators.

## Comprehensions (semantic role)

`for`, `if`, `let` clauses build values (usually lists/structs) from other
values. They are **generators**, not a separate type system: the result still
unifies in the lattice like any other value.

```cue
[for x in [1, 2, 3] if x > 1 {x * 2}]   // [4, 6]
{for k, v in m if v.enabled { (k): v }}
```

## Attributes and interpolations

- Interpolations (`"hello \(name)"`) produce strings/bytes from expressions.
- Attributes (`@foo(...)`) are metadata for tooling; they do not change
  lattice position of the annotated value in normal evaluation (tooling may
  interpret them separately).

## What “typing” means in CUE

There is no separate static type layer. Checking is **unification against
constraints** you wrote (or that a host program injected). A value is “well
typed” relative to a schema when `value & schema` is not bottom and meets
concreteness requirements the consumer imposes.

```cue
#Person: {
	name: string
	age:  int & >=0
}

alice: #Person & {
	name: "Alice"
	age:  30
}
```
