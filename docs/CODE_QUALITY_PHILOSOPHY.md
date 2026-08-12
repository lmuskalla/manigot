# CODE_QUALITY_PHILOSOPHY: the standard this project writes code against

A general engineering philosophy, written to stand on its own. It does not
depend on this project's history — it is not a reaction to anything that
came before. It is the standard this codebase holds itself to because it is
the standard good code meets, and it would apply to any codebase of similar
shape. The sibling documents are the local records: `CODE_QUALITY.md`
(where the code is today) and `CODE_QUALITY_TASKS.md` (how to close the
gap). This one is the law they are measured against.

## The core stance

Three beliefs underneath everything in this document:

1. **Code is read far more often than it is written.** The reader is the
   customer. Every line is written once and read dozens of times, by
   strangers and by your future self. Optimize for the reading.
2. **Boring, small, and explicit beat clever, large, and implicit.** A
   codebase that surprises no one is a codebase that can be changed
   safely. Predictability is a feature.
3. **The code is the only spec that survives.** Requirements live in code,
   intent lives in documentation, and the two must agree. When they
   disagree, both are wrong until they are fixed together.

## Structure

**1. Small pieces, single purpose.** A function does one thing; a file
holds one concern; a package owns one concept. Size is a symptom, not the
rule — but it is a useful one: a function past ~50 lines is usually doing
more than one thing, a file past ~400 lines is usually holding more than
one concern, a package past ~1,000 lines is usually covering more than one
concept. When a piece exceeds its size budget, the question to ask is not
"how do I split this?" but "what is the second thing this is doing?"

**2. One concept per package; the package name is the responsibility
statement.** A package called `job` owns jobs. If something is not about
jobs, it does not live there — no matter how convenient. When a package's
doc comment needs its second "and also," the package needs splitting. The
name is a contract: it tells the reader what they will find and, just as
importantly, what they will not.

**3. Composition over inheritance.** Prefer plain struct fields and
function parameters over embedding and over shared mutable bases. Embedding
is for genuine API inheritance (a type that *is* another type, with all of
its methods); if you only want the data, use a field. Composition keeps
dependencies explicit: you can see, at the call site, what a piece depends
on.

**4. Interfaces are consumer-defined, minimal, and rare.** An interface
exists where a real boundary needs one — most commonly so a test can
provide a fake, or so a caller can accept any implementation of a narrow
behavior. Define it where it is consumed, keep it to the methods actually
called, and do not invent one for design purity. "Accept interfaces,
return structs" is a good default, but the deeper rule is: the interface
must pay for itself by enabling something concrete (a fake, a plugin, a
second implementation) — not by making the design look clean. An interface
with exactly one implementation, created for the interface's sake, is
decoration.

**5. Dependencies point inward and never cycle.** Layering is a directed
graph: lower layers know nothing about higher ones. When a higher layer
needs something a lower layer owns, it depends on it; when a lower layer
needs something a higher layer owns, the thing must move down — and if
moving it down would create a cycle, the honest fix is extracting the
shared piece into a place both can reach, not duplicating it to dodge the
cycle. A cycle the compiler enforces you out of is a design signal, not an
inconvenience.

**6. High cohesion, low coupling — concretely.** Cohesion means the files
of a package change together: a change to one concept touches one package,
not three. Coupling means the seams between packages are narrow and
explicit: parameters and return values, not shared mutable state. If a
change to one concept requires edits in three packages, the concept has
been smeared across the tree.

## Data and state

**7. State flows through parameters, not globals.** A function's inputs
and outputs are visible at its signature; globals hide them. The only
package-level mutable state that is acceptable is a documented seam — an
injectable point that tests override (a clock, a command resolver, a
beep). Even those are compromises: they should be few, named as seams, and
revisited when a cleaner mechanism appears.

**8. Derive over cache — but cache the expensive, stable derivation.**
If the source of truth can change underneath you, derive: reading the
truth costs a little and is always right. If the derivation is expensive
and the source is stable, cache — but caching requires an invalidation
story, stated in the same place as the cache. A cache without a stated
invalidation rule is a future bug.

**9. Immutable by default; change by constructing.** Data that is shared
(across goroutines, across layers, across time) is safer read-only.
Prefer returning new values over mutating inputs, and treat mutation as
the exception, confined to the place that owns the state — typically the
UI loop or an explicitly stateful component.

**10. Make the hard part pure.** Decision logic — the logic that would be
a pain to test and a disaster to get wrong — should be pure functions:
same inputs, same outputs, no I/O. File reads, network calls, and process
execution live at the edges, behind the pure core. This is the single
highest-leverage structural move: a state machine you can unit-test
without fixtures is a state machine you can trust.

## Errors and failure

**11. Errors are values; handle them where you can, propagate where you
can't.** Handle means: the caller can do something sensible, including
degrade. Propagate means: the caller can't, so pass it up with context.
Classify what callers branch on — a small set of sentinel or typed errors
that say *which* failure happened — and leave everything else as opaque
detail. A codebase's sentinel set should be countable on one hand, each
one documented with its meaning and its branchable callers.

**12. Wrap with context; preserve the cause.** Every propagation adds
what the current layer knows ("cannot archive job X") and keeps the
original cause (`%w`). Discarding the wrapped error — building a new error
from a string instead — is data loss: it destroys the chain that tells a
debugger *where* the failure originated.

**13. Error types, not strings.** The message is presentation; the type is
the API. Consumers that need to branch must be able to — on a type or
sentinel, never by parsing wording. Wording changes are then free, and
wording is owned by the boundary that frames it for the user, not by the
layer that detected the failure.

**14. Fail loud at integrity boundaries; degrade at convenience
boundaries.** When the system cannot know the correct answer — an
inconsistent state, a missing prerequisite, an ambiguous input — error,
clearly and loudly. When it can safely fall back — an optional file
missing, a display feature unavailable — degrade, but visibly: the user
and the log must be able to tell that a fallback happened. Silent
wrongness is the worst outcome: worse than an error, because nothing
says it happened.

## Naming

**15. Names are the first documentation.** Package = concept, type = noun,
function = verb, boolean = question (`ok`, `exists`, `dirty`). A good name
makes the doc comment redundant; a bad name makes it a lie. When you
cannot name a thing clearly, you do not understand it yet — naming
difficulty is a design finding, not a vocabulary problem.

**16. One term per concept.** Every concept gets exactly one name, used
everywhere. Terminology drift — the same thing called "entry," "record,"
and "row" in three files — is a quiet readability killer: it makes
searches miss, reviews argue, and newcomers learn the same concept twice.
Drift found in review gets fixed in the same change, not backlogged.

**17. Clarity over brevity.** `worktreeForBranch` beats `wt4b`. A longer
name that says what it means is cheaper than a comment that explains what
a short name means. Abbreviations are only acceptable where the field
itself has made them canonical.

## Duplication and abstraction

**18. The rule of three, with a nuance.** Two copies: wait and watch —
they may be converging on different needs. Three copies: consolidate. The
nuance: when the copies are **identical and semantically load-bearing** —
logic whose divergence would be silent breakage rather than harmless
forking (matching rules, parsing rules, security checks) — consolidate at
two, because divergence there fails without notice.

**19. Duplication beats the wrong abstraction.** When consolidation would
force a fake generalization — a parameter that exists only to serve the
two current callers, a base type with one real user — keep the copies and
say so in a comment. The wrong abstraction is worse than the duplication:
it is the duplication plus a lie about what the code has in common.

**20. Abstract at the second concrete consumer, not the first imagined
one.** The general version is written when a second real need exists, not
when a first one is imagined. Premature abstraction is the most common way
codebases grow a parallel vocabulary of near-synonyms.

## Functions and flow

**21. One level of abstraction per function.** A function that alternates
between "what we are doing" and "how the mechanism works" is doing two
jobs. Decompose the mechanism into its own function; the reader of the
outer function should be able to follow the intent without descending.

**22. Guard clauses and early returns over nesting.** Validate inputs
first, return errors immediately, and keep the happy path linear. Code
that reads top-to-bottom without tracking an `else` is code that is
hard to get wrong.

**23. Small functions are a ceiling, not a target.** The goal is not
function-count maximization; it is that each function holds one idea.
Splitting for its own sake scatters the idea across call sites. When a
function is long but holds one idea and reads clearly, it is fine — but
that is the exception, and it should look like an exception.

## Testing

**24. Test behavior, not implementation.** A test should assert what the
code does — the decision it makes, the output it produces, the state it
leaves — not how it does it. Refactoring should not break tests; changed
behavior should. A test that breaks on a pure refactor is a coupling test,
and coupling tests are the tax that makes refactoring expensive.

**25. Match the test shape to the code shape.** Decision functions get
table-driven tests (inputs, expected outputs, one row per branch).
Multi-step flows get scenario tests — build a real environment, run the
flow, assert on the outcome. Both shapes are correct; using the wrong one
for the code is where tests become unreadable.

**26. Contract tests are deliberate; coupling tests are accidents.**
When the output wording is a user-facing contract, pinning it in a test is
a choice, and the choice should be written down where the pin lives. When
a test pins something that is merely incidental — internal structure,
helper names, intermediate values — that is accidental coupling, and it
gets removed when found. The difference is judgment, which is why testing
is a craft and not a rule.

## Consistency

**27. One way to do things.** For every recurring problem — argument
parsing, error propagation, subprocess invocation, state loading — there
is one canonical way, and new code follows it. The reader's cognitive
load is the real cost: two equivalent styles for the same thing mean every
file requires a decision that should already have been made. When a second
style exists, pick one — not because the other is wrong, but because
consistency itself is the feature.

**28. Tooling enforces; review judges.** Everything mechanical — format,
lint, static analysis, build — is enforced by tools and never argued in
review. Review is reserved for judgment: structure, naming, trade-offs,
the things a tool cannot decide. A review spent on formatting is a review
that did not happen.

## Simplicity and scope

**29. YAGNI is the default.** Generality must be earned by a second
concrete consumer. A configuration option, an interface, a parameter —
each is a tax on every future reader, charged to solve a problem that
exists today, never one that is imagined.

**30. The cost of code is its lifetime, not its build.** The expensive
part of any feature is not writing it; it is every future reader learning
it, every future change threading through it. Size every decision against
that cost. A feature that saves the author a day and costs every reader an
hour is a bad trade unless the author's day was the point.

**31. Boring over clever.** Cleverness is a liability with a payoff
schedule: clever code impresses once and confuses forever. When a problem
genuinely demands a clever solution, the documentation of *why* is part of
the solution — clever code without its justification is a trap.

## What we deliberately do NOT want

- **Abstraction for its own sake.** Repository patterns, dependency
  injection containers, and interface hierarchies exist to solve
  problems this codebase does not have. They would be decoration.
- **Framework churn.** No rewriting for the sake of a new version, no
  "modern stack" drift. The stack changes when a concrete problem forces
  it, not when a new release appears.
- **A test for every trivial line.** Coverage targets are floors, not
  ceilings. Tests exist to catch the regressions that matter; a getter
  does not need a test file, and a branching decision function with four
  branches absolutely does.
- **Premature generality.** The general version waits for the second
  consumer. Imagined futures do not count as consumers.
- **Perfectionism as a blocker.** A pragmatic approximation, shipped and
  reviewed, beats an elegant design that waits. Solid means the code
  meets the standards here — not that it is the Platonic ideal.
- **Cleverness as a virtue.** Being impressed by code is a hobby; being
  able to change it safely is the job.

## How to apply this

The decision heuristics follow directly:

1. Does a concern have a single owning package? New code for that concern
   goes there, period — never a second home.
2. Is the logic a decision? Make it pure, test it as a table, keep I/O at
   the edges.
3. Is a string, constant, or helper about to be written that exists
   elsewhere? Find the single source first. Duplicating it is the default
   no.
4. Does a new abstraction have two concrete consumers today? If not, it
   does not exist yet.
5. Is an interface's only justification "design cleanliness"? It is
   decoration; remove it.
6. Is an error message being built in the layer that detected the
   failure? The message is the boundary's job; the layer returns a fact.

When expedience conflicts with any of this, expedience may win — but the
deviation is written down where it happened, with the reason. That is what
"deliberate" means.

## How this document is used

- Consulted when designing, reviewing, and refactoring — the same moments
  the assessment and the tasks are consulted.
- It is itself subject to its own rules, most pointedly the last section's:
  when the project's needs change, this document is amended visibly, with
  the reason — not preserved as scripture, and not quietly ignored.
