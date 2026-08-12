# 03 — Types, structs & errors

> Teaches: structs, JSON tags, value receivers, the zero value, and
> idiomatic error handling. Grounded in: `internal/project/settings.go` and
> `internal/config/config.go`.

Go's data model is built on **structs** — named collections of fields —
plus two rules that shape how almost every Go program is written: the
**zero value** and **errors as values**. This chapter reads the two files
that persist manigot's settings and shows both rules in action.

## A struct with JSON tags

`internal/project/settings.go` holds the settings that travel with a
project. The heart of it is a struct:

```go
type Settings struct {
	BaseBranch string `json:"baseBranch,omitempty"`
	JobBranchPrefix string `json:"jobBranchPrefix,omitempty"`
}
```

A struct field is `Name Type`. The backquoted `json:"..."` is a **struct
tag** — metadata attached to the field. Here it tells the `encoding/json`
package how to map the Go field to a JSON key:

- `baseBranch` ↔ `BaseBranch`, `jobBranchPrefix` ↔ `JobBranchPrefix`.
- `omitempty` means: when marshaling, skip the field if it's the zero value
  (empty string). An empty `.manigot/manigot.json` therefore stays a bare
  `{}` instead of being filled with empty strings.

The doc comments on each field explain the *semantics* — and note how
they're written from the caller's perspective ("Empty is treated as
'main' — see BaseBranchValue"). That's idiomatic Go documentation: what
the field means and what the empty value means.

Structs are the workhorse type in Go. Everything that needs to group
data — settings, profiles, UI state, command results — is a struct. The
`Profile` struct in `internal/config/config.go` is another example:

```go
type Profile struct {
	ID    string
	Label string
	Tool  string
	Auth  string
}
```

No tags needed there because `Profile` is never serialized to JSON — only
used in memory. Tags exist to describe the boundary between Go data and
some other format (JSON here, but the same mechanism serves YAML, TOML,
and database mappings).

## Value receivers: methods on structs

A method is a function with a receiver — the value it belongs to. Go has
two receiver kinds, and this codebase uses the value receiver
deliberately. From `settings.go`:

```go
func (s Settings) BaseBranchValue() string {
	if s.BaseBranch == "" {
		return "main"
	}
	return s.BaseBranch
}
```

`(s Settings)` makes `BaseBranchValue` a method on `Settings`, callable as
`s.BaseBranchValue()`. Because the receiver is a **value** (not a pointer
`*Settings`), the method gets a *copy* — it can read `s.BaseBranch` but
could never modify the original. That's exactly right here: the method
derives a default from the value, it doesn't change it.

The pointer receiver `(s *Settings)` exists for methods that must modify
the struct (or that want to avoid copying a large struct). This file
deliberately uses value receivers everywhere — `Settings` is two small
strings, so copying is free and the "read-only view of the data" semantics
are a useful guarantee.

Also note the naming convention: a **getter that applies a default** is
called `BaseBranchValue()` (not `GetBaseBranch()`). In Go, "getter" names
drop the `Get`, and the `Value` suffix signals "the effective value after
defaults". The same pattern appears in `config.go`:
`ProfileValue()`, `RecentActivityCountValue()`.

## The zero value: defaults for free

The single most important Go concept in these files: **every type has a
zero value, and an uninitialized variable holds it.** For a string it's
`""`; for an int, `0`; for a struct, the struct whose fields are all zero.

This makes "missing config" a non-event. Look at `Load` in `settings.go`:

```go
func Load(root string) (Settings, error) {
	data, err := os.ReadFile(Path(root))
	if err != nil {
		if os.IsNotExist(err) {
			return Settings{}, nil
		}
		return Settings{}, err
	}
	var s Settings
	if err := json.Unmarshal(data, &s); err != nil {
		return Settings{}, fmt.Errorf("%s: %w", Path(root), err)
	}
	return s, nil
}
```

Two idiomatic moves:

1. **Missing file → zero value, not an error.** `if os.IsNotExist(err) {
   return Settings{}, nil }` — the file not existing is the normal
   pre-first-save state, so it yields `Settings{}` (the zero value). Every
   caller then gets working defaults via `BaseBranchValue()`. The package
   comment says it explicitly: "A file that does not exist yet ... is not
   an error — it yields the zero-value Settings, which every caller treats
   as 'defaults'." This "degrade to defaults" shape is a manigot-wide
   philosophy; `config.Load`'s comment says it mirrors `project.Load`
   "exactly so the two packages degrade the same way."
2. **`var s Settings` declares a zero value, then `json.Unmarshal` fills
   it.** Unmarshal only sets fields that appear in the JSON; anything
   missing stays zero. Absent fields and absent files both end up as
   defaults — one mental model.

The zero value is why so much Go code needs no constructor functions: the
zero value is often already a usable value. manigot leans on this
everywhere — a `Settings{}` with no file behind it is a perfectly good
configuration.

## Errors as values

Go has no exceptions. A function that can fail returns an `error` as its
last result, and the caller checks it:

```go
data, err := os.ReadFile(Path(root))
if err != nil {
	...
}
```

This is the `if err != nil` pattern — the most common idiom in Go. Errors
are ordinary values: you can return them, compare them, wrap them.

The files show the three canonical error moves:

**1. Wrapping with `%w` preserves the cause chain.**

```go
if err := json.Unmarshal(data, &s); err != nil {
	return Settings{}, fmt.Errorf("%s: %w", Path(root), err)
}
```

`fmt.Errorf` with `%w` **wraps** the error: the returned error carries the
original inside it. Any caller up the stack can `errors.Is(err, ...)` /
`errors.As(err, ...)` (chapter 05) to check the underlying cause, while the
message gains context — the path that failed to parse. Without `%w`
(using `%v`), the context is added but the cause chain is lost. Rule of
thumb used here: *never discard an error without saying where it came
from*; wrap it, and the message names the file.

**2. `os.IsNotExist(err)` is the "file missing" test.**

`os.ReadFile` returns a specific error when the file doesn't exist.
`os.IsNotExist(err)` recognizes it (including through wrappers). Every
"maybe the file isn't there yet" read in manigot uses this to decide
between "normal, use defaults" and "something actually broke".

**3. Decide what's an error and what isn't.**

The error-returning functions here are careful about *what* counts as
failure:

- `Load`: file missing → `nil` error (normal); unreadable → error; invalid
  JSON → error with the path in the message.
- `Save`: returns `err` directly for filesystem problems, and a descriptive
  `fmt.Errorf` for a bad profile ID: `"cannot save: %q is not a valid
  profile id"` — note `%q` for quoting the offending value.

A clean pattern to copy: **return `(value, error)` and make "nothing to
read" return the zero value with a nil error.** Callers end up with no
error-handling boilerplate for the common case.

## The whole file as a design lesson

`settings.go` is 95 lines and reads like a spec. Read it as one flow:

```
Path()   — where the file lives        ("<root>/.manigot/manigot.json")
Load()   — read + parse, defaults on missing/invalid
Save()   — mkdir + marshal + write
BaseBranchValue() — the one place "empty means main" is decided
```

Each function has one job, a doc comment saying what it guarantees, and
error handling that distinguishes "expected absence" from "real failure."
This is what idiomatic Go looks like at the package level: small, honest,
composable.

## Exercise

Open `internal/config/config.go` and find:

1. Three struct types and their JSON tags. Which fields have `omitempty`,
   and why do you think each does?
2. The `Load` function — it has **two** `os.IsNotExist` branches (for the
   settings file and the `.env` handling). What does each treat as "normal"
   instead of an error?
3. `UpsertEnv` — trace what it does line by line. What's the trick with
   `lines[len(lines)-1]` at the end, and what does it handle?

**Next:** [04 — Testing](04-testing.md) — how manigot verifies all of this
with Go's built-in testing package.
