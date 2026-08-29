# Examples (semantics & typing)

Small, self-contained snippets. Evaluate mentally or with `cue eval` / `cue vet`
when a CUE binary is available. No `package`/`import` — paste into a single file
or your host’s fragment loader.

## 1. Types are values

```cue
a: int
b: a & 5          // 5
c: a & "x"        // _|_

n: number
i: n & 3          // 3 (int)
f: n & 3.14       // 3.14 (float)
```

## 2. Unification merges and tightens

```cue
base: {
	kind: string
	replicas: int & >=1
}

prod: base & {
	kind:     "Deployment"
	replicas: 3
}

// conflict
bad: base & {
	kind:     "Deployment"
	replicas: 0      // fails >=1
}
```

## 3. Disjunction + discriminator

```cue
#Resource: {
	#A: {api: "a", x: int}
	#B: {api: "b", y: string}
	#A | #B
}

ok:  #Resource & {api: "a", x: 1}
bad: #Resource & {api: "a", y: "nope"}   // _|_ no branch fits
```

## 4. Defaults

```cue
listen: {
	host: *"0.0.0.0" | string
	port: *8080 | int & >=1 & <=65535
}

// incomplete input selects defaults
d1: listen & {}

// override port only
d2: listen & {port: 443}
// host still default 0.0.0.0
```

## 5. Optional vs required

```cue
#User: {
	id!:   string
	name!: string
	email?: string
}

u1: #User & {id: "1", name: "Ann"}
u2: #User & {id: "1", name: "Ann", email: "a@b.c"}
// u3: #User & {id: "1"}  // _|_ missing name!
```

## 6. Closed definition rejects extras

```cue
#Point: {
	x: number
	y: number
}

p1: #Point & {x: 1, y: 2}
// p2: #Point & {x: 1, y: 2, z: 3}  // _|_
```

Open with ellipsis:

```cue
#PointOpen: {
	x: number
	y: number
	...
}

p3: #PointOpen & {x: 1, y: 2, z: 3}  // ok
```

## 7. Bounds and regex

```cue
#Semverish: string & =~"^[0-9]+\\.[0-9]+\\.[0-9]+$"

#Port: int & >=1 & <=65535

svc: {
	version: #Semverish & "1.2.3"
	port:    #Port & 8080
}
```

## 8. Lists: open tail

```cue
#IntList: [...int]

xs: #IntList & [1, 2, 3]
// ys: #IntList & [1, "x"]  // _|_

#PairThenMore: [string, string, ...int]
t: #PairThenMore & ["a", "b", 1, 2]
```

## 9. Pattern fields

```cue
#Annotations: {
	[string]: string
}

meta: #Annotations & {
	"app.kubernetes.io/name": "api"
	team:                     "platform"
}
```

## 10. Comprehension + outer constraint

```cue
#Name: string & =~"^[a-z][a-z0-9-]*$"

raw: ["api", "Web", "db"]   // "Web" invalid if constrained

// names: [...#Name] & [for n in raw {n}]  // _|_ due to Web

names: [...#Name] & [for n in raw if n =~ "^[a-z]" {n}]
// ["api", "db"]
```

## 11. Layered config (unify as merge)

```cue
defaults: {
	logLevel: *"info" | "debug" | "warn" | "error"
	features: {
		metrics: *true | bool
	}
}

env_dev: {
	logLevel: "debug"
}

env_prod: {
	features: {metrics: true}
}

dev:  defaults & env_dev
prod: defaults & env_prod
```

Order of `&` does not matter for the result (commutativity / associativity).

## 12. Self-check schema against data

```cue
#Schema: {
	host: string
	port: int & >0
}

// data-only fragment
given: {
	host: "localhost"
	port: 8080
}

// validation = unify
validated: #Schema & given
```

If `validated` is not bottom and is concrete, data satisfies schema.

## 13. Hidden helper, public shape

```cue
_defaultPort: 8080

#Server: {
	host: string
	port: int | *_defaultPort
}

s: #Server & {host: "h"}
// s.port == 8080; _defaultPort not part of exported fields
```

## 14. Bottom propagation

```cue
x: int & string      // _|_
y: {a: x}            // struct containing bottom in a
z: y & {a: 1}        // still _|_ (cannot salvage field a)
```

Fix at the conflicting conjuncts (`x`), not only at `z`.

## Agent checklist when editing CUE

1. What must be **true** after unify? Write that as constraints, not prose.
2. Are composition boundaries **structs with stable keys** (if layers merge)?
3. Should extras be **forbidden** (`#Def` / `close`) or **allowed** (`...`)?
4. Missing data: **optional** (`?`), **default** (`*`), or **required** (`!`)?
5. Alternatives: **disjunct** with a clear discriminator field.
6. Failure: find the **first conflicting field/bounds/branch**, fix source data
   or relax the schema deliberately.
