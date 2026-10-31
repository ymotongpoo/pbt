# Design of pbt, a property-based testing library for Go (draft)

This is the design as of 2026-10-31. The library is named `pbt` and its import
path is `go.ymotongpoo.com/pbt`. The section "Repository and module setup" at
the end lists what that requires. A Japanese version of this document is in
[DESIGN.ja.md](DESIGN.ja.md); when the two disagree, this file wins.

## Goals

**Property-based testing** (PBT) generates random inputs and checks that a
property holds for each of them. When one fails, it minimizes the input
before it reports it. Go has [rapid](https://github.com/flyingmutant/rapid)
and [gopter](https://github.com/leanovate/gopter), but both cover far less
than Erlang's [PropEr](https://proper-testing.github.io/) or Haskell's
[QuickCheck](https://hackage.haskell.org/package/QuickCheck). This library
sets out to:

- Bring the PropEr feature set to Go: statistics, stateful testing with
  symbolic variables, parallel command testing, targeted PBT, existential
  properties, function generation, and generators derived from types.
- Keep what rapid does well: type-safe generators built on generics,
  shrinking that needs no user code, and failures that are saved and replayed.
- Feel like a natural extension of the standard `testing` package.
- Express every example and exercise of *PropEr Testing* by Fred Hebert, the
  author of *Learn You Some Erlang for Great Good!*. The edition is the one
  published at [propertesting.com](https://propertesting.com/). This document
  calls it "the book".

gopter predates generics and works through `interface{}`, so rapid is the
point of comparison.

## Feature comparison

The rapid column reflects the
[pkg.go.dev index of v1.3.0](https://pkg.go.dev/pgregory.net/rapid). The pbt
column is the proposal in this document.

| Feature | PropEr | QuickCheck | rapid v1.3.0 | pbt (proposed) |
| --- | --- | --- | --- | --- |
| Type-safe generators | n/a (dynamic typing) | yes | yes | yes |
| Shrinking without user code | partial (custom types need a shrinker) | partial (`shrink` must be implemented) | yes | yes |
| Shrinking across Bind (`?LET`) | partial (shrinks the inner value, regenerates the outer) | no (`>>=` loses shrinking) | yes | yes |
| Shrinking control (`?SHRINK`, `?LETSHRINK`) | yes | yes | no | partial (subtree shrinking is automatic; value-level hints come later) |
| Size parameter (`sized`, `resize`) | yes | yes | no | yes |
| Weighted choice (`frequency`, `weighted_union`) | yes | yes | no (`OneOf` is uniform) | yes |
| Distribution checks (`cover`, `checkCoverage`) | no | yes | no | yes |
| Statistics (`collect`, `classify`, `aggregate`, `measure`) | yes | yes | no | yes |
| Property combinators (`conjunction`, `?IMPLIES`, `?WHENFAIL`, `?TIMEOUT`) | yes | yes | no | yes |
| Stateful testing | yes | no (separate libraries) | yes | yes |
| Symbolic variables | yes | n/a | no | yes |
| Parallel command testing (interleaving check) | yes | no | no | yes |
| Targeted PBT | yes | no | no | yes |
| Existential properties (`?EXISTS`) | yes | no | no | yes |
| Function generation | yes | yes | no | yes |
| Generators derived from types | yes (from type declarations) | partial (generic deriving) | yes (`Make[T]`) | yes (reflection and code generation) |
| Checking functions against specs (`check_spec`) | yes | no | no | partial (limited to "does not panic" and similar) |
| Saving failures and replaying them | partial (in-process only) | no (only a seed to pass again) | yes | yes |
| Hooking into a coverage-guided fuzzer | no | no | yes (`MakeFuzz`) | no (out of scope) |

## Reusing testing.F

Users see Go's fuzzing as `testing.F`, but the implementation lives in
`internal/fuzz`. The table lists what the Go 1.27.1 sources show and what
follows from it.

| Aspect | Fact | Consequence |
| --- | --- | --- |
| Visibility | `internal/fuzz` cannot be imported from user modules | Copying the code is the only way to reuse it |
| Minimization | `isMinimizable` returns true only for `string` and `[]byte`; integers and floats are never minimized. The algorithm trims the tail and then deletes single bytes | Useless for structured values |
| Mutation | Covers every primitive type | A reference for the neighborhood function of targeted PBT |
| Coverage | Available only in a binary built by `go test -fuzz` | Not information a library can read during a normal `go test` |
| Process model | A coordinator and worker processes that talk over shared memory and RPC | Too heavy for a PBT run that finishes within a second |
| `F.Fuzz` arguments | Only primitive types, `string` and `[]byte` | Structured values would have to be built from `[]byte` by hand |

Decision (2026-10-31): pbt neither reuses `testing.F` nor plugs into it. The
shrinker is our own, and pbt saves failures under `testdata/pbt/`, following
the idea of `testdata/fuzz`. A choice sequence is a byte sequence, so one
function that returns `func(*testing.T, []byte)` could connect to `F.Fuzz`
later. That is outside the scope of this library.

## The core decision: how to shrink

**Shrinking** replaces a failing input with simpler inputs until it finds a
minimal one that still fails. The shrinking method bounds both the API and
the feature set, so it comes first. There are three candidates.

| Method | Examples | Mechanism | Shrinks across Bind | User effort | Value-level hints |
| --- | --- | --- | --- | --- | --- |
| Type-directed | QuickCheck, PropEr, gopter | Each generator carries a function from a value to its shrink candidates | no (QuickCheck's `>>=` loses shrinking; PropEr's `?LET` shrinks the inner value and regenerates the outer one, so progress on the outer value is lost) | A shrinker for every custom type | yes |
| Integrated | [Hedgehog](https://hedgehog.qa/), [falsify](https://hackage.haskell.org/package/falsify) | A generator returns a lazy tree of a value and its shrink candidates; Bind composes the trees | yes | none | partial (by rewriting the tree) |
| Choice sequence | [Hypothesis](https://hypothesis.readthedocs.io/), rapid | A generator is a deterministic function of a sequence of choices; the shrinker rewrites the sequence and regenerates the value | yes | none | no in Hypothesis; pbt adds `ShrinkTo` and `LetShrink`, described below |

pbt uses the **choice sequence** method, for three reasons. Shrinking works
across Bind, Filter, and any generator code a user writes. A command sequence
in a stateful test becomes a sequence of command choices and argument choices.
Deleting commands and shrinking arguments therefore use the same machinery. A
choice sequence is a byte sequence, so saving a failure and replaying it from
a seed share one representation. rapid shows that the method is practical in
Go.

The method comes from Hypothesis, but only the shrinking mechanism does. My
disagreement with Hypothesis, described in the next section, is about keeping
distribution control away from users, not about the mechanism. Integrated
shrinking would be possible with lazy trees built on Go 1.23's `iter.Seq`. It
would cost memory to hold the trees and a second representation for saved
failures, so pbt does not use it.

Some PropEr features have no direct expression in a choice sequence. pbt
recovers them as follows.

- `?LETSHRINK`, which shrinks a tree to one of its subtrees. The sequence
  records, with a label, which generator consumed which span of choices. The
  shrinker replaces a span with a descendant span that carries the same
  label. Hypothesis calls this `pass_to_descendant`. For explicit control,
  pbt also offers `LetShrink`: it generates all parts and records one extra
  choice, "combine the parts or return one of them". Generation always
  combines; shrinking flips the choice to a part. This covers the book's
  `A ++ B ++ C` and tree examples in Chapter 8.
- The size parameter. Hypothesis and rapid hide it; pbt exposes it as PropEr
  does. The runner grows the size from a start size to a maximum, and during
  shrinking the size stays fixed while the choices shrink.
- `?SHRINK`, which shrinks toward values of other generators. Chapter 8 uses
  it to re-center years on 1970 and time-zone minutes on 0, 15, 30 and 45.
  pbt offers `ShrinkTo(g, alts...)`, which has the structure of a
  `Frequency` whose first alternatives have weight 0 and whose last
  alternative is `g`. Generation only picks `g`; shrinking lowers the index
  and lands on an alternative. `ShrinkToward(center)` covers the common case
  of pulling a number toward one value.
- pbt does not offer general value-level shrinking, which would need an
  encoding from arbitrary values back into choices. The two tools above
  express every use in the book.

## Design principle: users control the distribution

Hypothesis has no way to weight the alternatives of `one_of`. In July 2022 I
asked on hypothesis-users whether PropEr's `frequency/1` had an equivalent.
The maintainer, Zac Hatfield-Dodds,
[replied](https://groups.google.com/d/msgid/hypothesis-users/c67575ea-16c7-41f0-9e88-e16c1d589f8cn%40googlegroups.com)
with three reasons for not providing one. Probability options make the API
larger and harder to learn. They focus users on particular kinds of data
instead of the whole domain. And bugs hide where users did not think to look,
so Hypothesis, with HypoFuzz and CrossHair, outperforms almost any user at
finding them.

pbt does not take that position. In a long-running stateful test, you often
need to fix the ratio of reads to writes, or how often the test enters an
error path. Otherwise the test never reaches the state you want. PropEr and
QuickCheck expose `frequency` and `sized` and, at the same time, give you
`collect` and `classify` to observe the distribution. That pairing says the
user owns the distribution. If pbt hands you the distribution, it also hands
you the tools to check it. It therefore includes an equivalent of
QuickCheck's `cover` and `checkCoverage`. You declare that a class must make
up at least 20% of the cases. The test fails when the statistics do not bear
that out.

To the argument that the engine beats people at exploration, pbt answers with
targeted PBT. You write a utility function and the engine searches for inputs
that maximize it, so you also decide where the search goes.

Weights in a choice sequence follow one rule. A weight affects only the
probability at generation time; shrinking moves toward the first alternative
no matter what the weights are. The order of the alternatives in `Frequency`
is the shrinking priority. Primitive generators still lean toward boundary
values by default, and a combination of `Frequency` and `Just` overrides that.

## API sketch

Like rapid, a property draws values from generators inside its body. Go has
no variadic type parameters, so a QuickCheck-style `ForAll` that lists
generators needs one function per arity. `ForAll`, `ForAll2` and `ForAll3`
exist as sugar. The equivalents of PropEr's macros are functions that treat a
`Property` as a value. The listing below is the API as implemented; the
package documentation is the reference.

```go
package pbt

// A generator: a deterministic function that consumes choices and returns a value.
type Gen[V any] struct{ /* unexported */ }
func (g Gen[V]) Draw(t *T, label string) V
func (g Gen[V]) Example(seed ...uint64) V                 // proper_gen:pick

func Int() Gen[int]
func IntRange(lo, hi int) Gen[int]
func IntRangeToward(lo, hi, center int) Gen[int]          // ?SHRINK re-centering for numbers
func Integer[I integer]() Gen[I]                          // and IntegerRange, IntegerRangeToward
func Float64() Gen[float64]                               // and Float64Range, Float32, Float32Range
func Bool() Gen[bool]
func Rune() Gen[rune]
func String() Gen[string]                                 // and StringN, StringOf, StringOfN
func Bytes() Gen[[]byte]                                  // and BytesN
func SliceOf[E any](elem Gen[E]) Gen[[]E]                 // and SliceOfN, SliceOfDistinct, SliceOfNDistinct
func MapOf[K comparable, V any](k Gen[K], v Gen[V]) Gen[map[K]V] // and MapOfN
func Ptr[E any](elem Gen[E], allowNil bool) Gen[*E]
func Permutation[E any](values []E) Gen[[]E]
func SampledFrom[E any](values []E) Gen[E]                // elements
func Any() Gen[any]                                       // term(), bounded
func Just[V any](v V) Gen[V]                              // exactly
func Custom[V any](name string, f func(*T) V) Gen[V]
func Map[A, B any](g Gen[A], f func(A) B) Gen[B]
func Bind[A, B any](g Gen[A], f func(A) Gen[B]) Gen[B]   // ?LET
func Filter[V any](g Gen[V], pred func(V) bool) Gen[V]   // ?SUCHTHAT
func OneOf[V any](gs ...Gen[V]) Gen[V]                   // union
type Weighted[V any] struct{ Weight int; Gen Gen[V] }
func Frequency[V any](ws ...Weighted[V]) Gen[V]          // weighted_union; weight 0 is reachable only by shrinking
func Sized[V any](f func(size int) Gen[V]) Gen[V]        // ?SIZED
func Resize[V any](size int, g Gen[V]) Gen[V]            // resize
func Lazy[V any](f func() Gen[V]) Gen[V]                 // ?LAZY
func ShrinkTo[V any](g Gen[V], alts ...Gen[V]) Gen[V]    // ?SHRINK
func LetShrink[V any](parts []Gen[V], combine func([]V) V) Gen[V] // ?LETSHRINK
func NoShrink[V any](g Gen[V]) Gen[V]                    // noshrink
func SampleShrink[V any](g Gen[V], seed ...uint64) iter.Seq[V]    // proper_gen:sampleshrink
func Func[A comparable, B any](ret Gen[B]) Gen[*Fn[A, B]] // function/2; call through Fn.Call
func Func2[A, B comparable, C any](ret Gen[C]) Gen[*Fn2[A, B, C]]
func AsAny[V any](g Gen[V]) Gen[any]                     // widen for OneOf over different types
func Make[V any](opts ...MakeOption) Gen[V]              // derived from the type by reflection; pbt struct tags, WithType, WithField
func Recursive[V any](leaf Gen[V], extend func(self Gen[V]) Gen[V]) Gen[V] // bounded recursion
func Bounded[V any](leaf, node Gen[V]) Gen[V]            // one recursion budget per value, shared across types
func NoPanic(f any, opts ...MakeOption) Property           // check_spec, limited to the signature
// cmd/pbtgen: go run go.ymotongpoo.com/pbt/cmd/pbtgen -type T1,T2 writes GenT1, GenT2 as source

// Properties.
type Property func(*T)
type TB interface { Helper(); Name() string; Logf(string, ...any); Errorf(string, ...any) } // satisfied by *testing.T
func Check(tb TB, prop Property, opts ...Option)
func CheckExists(tb TB, prop Property, opts ...Option)   // ?EXISTS
func ForAll[A any](g Gen[A], body func(*T, A)) Property  // and ForAll2, ForAll3
func Conjunction(subs ...Named) Property                 // conjunction
func Timeout(d time.Duration, prop Property) Property    // ?TIMEOUT, checked after the property returns
func Recover(f func()) (panicked any)
func Checks(n int) Option                                // numtests
func Seed(seed uint64) Option
func Sizes(start, max int) Option                        // start_size, max_size
func ShrinkTime(d time.Duration) Option
func SearchSteps(n int) Option                           // search_steps
func Setup(f func(t *T) (teardown func())) Option        // ?SETUP
func CheckCoverage() Option                              // checkCoverage
func NoSave() Option
func TestdataDir(dir string) Option
func (t *T) Assume(cond bool)                            // ?IMPLIES
func (t *T) Classify(cond bool, label string)            // classify
func (t *T) Collect(v any)                               // collect
func (t *T) Aggregate(values ...any)                     // aggregate
func (t *T) Measure(name string, v float64)              // measure
func (t *T) Cover(cond bool, pct float64, label string)  // cover
func (t *T) OnFail(f func())                             // ?WHENFAIL
func (t *T) Maximize(name string, utility float64)       // ?MAXIMIZE; Minimize reverses the sign
func (t *T) Size() int
// plus Errorf, Error, Fatalf, Fatal, Fail, FailNow, Failed, Logf, Log, Skip, Skipf, SkipNow, Cleanup

// Stateful testing.
type Var struct{ /* unexported */ }                      // {var, N}; resolved through Env
type Env struct{ /* unexported */ }
func (e Env) Get(v Var) any
func Resolve[T any](env Env, v Var) T
type Command[S, Sys any] interface {
	Precondition(state S) bool
	Execute(sys Sys, env Env) any
	NextState(state S, result Var) S                     // symbolic during generation and execution
	Postcondition(state S, env Env, result any) error
}
type Cmd[S, Sys any] struct {                            // Command built from function fields
	Name string; Args []any
	Pre  func(state S) bool
	Run  func(sys Sys, env Env) any
	Next func(state S, result Var) S
	Post func(state S, env Env, result any) error
}
func Commands[S, Sys any](init S, gen func(S) Gen[Command[S, Sys]]) Gen[Program[S, Sys]] // and CommandsN
func NoCommands[S, Sys any]() Gen[Command[S, Sys]]       // ends the sequence
func (p Program[S, Sys]) RunSequential(t *T, sys Sys)
func ParallelCommands[S, Sys any](init S, gen func(S) Gen[Command[S, Sys]]) Gen[ParallelProgram[S, Sys]] // and ParallelCommandsN
func (p ParallelProgram[S, Sys]) RunParallel(t *T, sys Sys)
func CommandName(c any) string
```

The layer that corresponds to the book's `proper_fsm` in Chapter 11 lives in
the subpackage `fsm`. It pairs a state name with state data and attaches
weights to the transitions listed for each state (`weight/3`). It runs on top
of the `Command` machinery. Statistics label each command with its source
state, as `zip(state_names, command_names)` does.

```go
package fsm // go.ymotongpoo.com/pbt/fsm

type State[N comparable, D any] struct{ Name N; Data D }
type Transition[N comparable, D, Sys any] struct {
	Weight int                                            // weight/3; 0 is reachable only by shrinking
	To     N
	Cmd    pbt.Gen[pbt.Command[D, Sys]]                   // precondition/4, next_state_data/5, postcondition/5 on the data
}
type Machine[N comparable, D, Sys any] interface {
	Initial() (N, D)                                      // initial_state, initial_state_data
	Transitions(name N, data D) []Transition[N, D, Sys]   // the state functions
}
func Commands[N comparable, D, Sys any](m Machine[N, D, Sys]) pbt.Gen[pbt.Program[State[N, D], Sys]]
func ParallelCommands[N comparable, D, Sys any](m Machine[N, D, Sys]) pbt.Gen[pbt.ParallelProgram[State[N, D], Sys]]
```

Run-time options come from the flags `-pbt.checks`, `-pbt.seed`,
`-pbt.startsize`, `-pbt.maxsize`, `-pbt.shrinktime`, `-pbt.searchsteps`,
`-pbt.nosave` and `-pbt.v`; options passed to `Check` take precedence. The
book's `rebar3 proper -n`, `-p` and `--start_size` map to `-pbt.checks`,
`go test -run` and `-pbt.startsize`.

Stateful tests use **symbolic variables** (`Var`) because, while the command
sequence is generated, no command has run yet and no result exists. PropEr
lets the model state refer to results as `{var, N}` during generation and
resolves them to real values during execution. Go does the same with a `Var` and an environment `Env` that looks values up
at run time. Commands return results of different types and one generator
yields commands of many kinds, so `Var` carries no type parameter; the
generic `Resolve[T]` performs the typed lookup. `NextState` receives the Var
both during generation and during execution, so the model never depends on
real results and shrinking can regenerate the sequence; `Postcondition`
receives the real result and the Env.
The command generator receives the model state, so the weights of a
`Frequency` can depend on it. That matches the use of `frequency` inside
PropEr's `command/1`. While shrinking, every deletion of a command
re-evaluates the preconditions, and a sequence that violates one is not a
counterexample candidate. The book's Chapter 10 depends on this: shrinking
starts to work only once the preconditions return false. On failure, the
runner prints the executed commands, their results, and the model state at
each step. Chapter 9 writes the same by hand inside `?WHENFAIL`.

Parallel command testing follows PropEr's `parallel_commands`. The generator
produces a sequential prefix followed by at most three parallel branches, and
the runner executes the branches in goroutines. It then enumerates the
interleavings of the recorded histories to find one that the sequential model
accepts. The check keeps only the order within each branch and ignores the
real-time order between branches, which is what PropEr does as well; it is
therefore weaker than a linearizability check. With at most twelve commands,
brute force is enough.

Targeted PBT in PropEr is simulated annealing over neighborhood functions on
values. In a choice sequence, the neighborhood is a small mutation of the best
sequence so far; the mutator in `internal/fuzz` is the reference for it. The
same search implements existential properties, and pbt shrinks the witness
it finds before it reports it.

## Priorities

| Phase | Contents | Done when |
| --- | --- | --- |
| 0 | Core engine: the choice sequence with labeled spans, ChaCha8 randomness from `math/rand/v2`, the runner (count, size growth, seed), the shrinker (span deletion, zeroing, binary search on single values, redistribution between neighbors, replacement by a descendant span), panic recovery, the minimal counterexample reported as labeled draws, saving to `testdata/pbt/` and automatic replay | rapid's public tests, ported, shrink to counterexamples at least as small |
| 1 | The full generator set (matching PropEr's `proper_types`), function generation, property combinators, statistics output and distribution checks, shrinking control (`ShrinkTo`, `ShrinkToward`, `LetShrink`, `NoShrink`, `SampleShrink`) | Every example and exercise of Chapters 1 to 8 of the book is ported |
| 2 | Sequential stateful testing, symbolic variables, `Setup`, precondition re-evaluation while shrinking, history output, the `fsm` package | The sequential cache of Chapter 9, the bookstore of Chapter 10 and the circuit breaker of Chapter 11 are ported |
| 3 | Parallel command testing (interleaving check), combined with the race detector and `testing/synctest` | The race planted in the cache of Chapter 9 is detected |
| 4 | Targeted PBT, existential properties | The targeted PBT examples in the PropEr documentation are ported (the online edition of the book has no chapter on them) |
| 5 | Generators derived from types (`Make[T]` through reflection, constraints in struct tags, code generation with `go/types` under `go generate`), checking functions against specs | Generators for structs with recursive types are derived automatically |
| 6 | Visualization of shrinking, documentation, performance comparison with rapid | A migration guide with a correspondence table exists |

Phase 0 uses rapid as the yardstick because the two share a method, so the
difference shows directly in shrinking quality. As of 2026-10-31, Phases 0
to 4 are implemented and tested, including parallel command testing and
targeted search, every chapter of the book is ported under `examples/`,
and Phase 5 exists: the reflection-based `Make[T]` with struct tags and
overrides, the code generator `cmd/pbtgen` built on `go/types`, which writes
the same generators as source and can fill unexported fields, `Recursive`
for bounded recursion, and `NoPanic` as the signature-only form of
`check_spec`. The shrink quality checks of rapid are mirrored in
`parity_test.go`, MIGRATION.md maps rapid's API onto pbt, and the `bench/`
module compares the two with the results in BENCHMARK.md. The Chapter 10
port runs on PostgreSQL as well when `PBT_BOOKSTORE_DSN` is set; that run
found two differences from SQLite, the backslash as the default `LIKE`
escape and the enforced column width, both recorded in the chapter's
README. Visualization stays at the verbose shrink log.

## What Go offers

Go has features that PropEr and QuickCheck lack in this form.

- Generics make generators type-safe. Methods cannot have type parameters, so
  `Map` and `Bind` are top-level functions.
- `iter.Seq` from Go 1.23 suits lazy enumeration of shrink candidates and
  statistics. It would also represent the trees if the design ever moved to
  integrated shrinking.
- ChaCha8 in `math/rand/v2` is seeded, deterministic, and easy to split into
  streams.
- The race detector combines with parallel command testing.
- `testing/synctest`, stable since Go 1.25, runs properties that involve
  timeouts deterministically under virtual time. rapid added `SyncTest` in
  v1.3.0.
- `go/types` with `go generate` can derive generators from type declarations
  as code, type-safely, where PropEr's `proper_typeserver` works at run time.

Supported versions are the latest stable release and the two before it
(decided 2026-10-31). In October 2026 that means Go 1.25, 1.26 and 1.27, and
the `go` directive in `go.mod` is 1.25. The floor rises with every new
release.

## Coverage of the book's problems

The online edition of the book has eleven chapters. Apart from the Elixir
material, its examples and exercises fit the table below. The Go port of
each chapter's examples and exercises lives under `examples/` and serves as
the acceptance test of its phase. Some exercises ask for an explanation
rather than code: Chapter 4 Q1 and Q2, Chapter 5 Q1 to Q3, Chapter 8 Q1 and
Q2, and Chapter 10 Q1, Q2, Q4 and Q5. Their Go answers go in the README of
each chapter.

| Chapter | Examples and exercises | Features needed | How pbt covers them |
| --- | --- | --- | --- |
| 1 | Properties of the cash register `cash/3` (the change adds up; larger denominations come first); a counterexample that crashes | `ForAll`, panic recovery, counterexample output | Phase 0 |
| 2 | `rebar3 proper -n`, `-p`, trying generators in `rebar3 as test shell` | Case count, property selection, sampling a generator | `-pbt.checks`, `go test -run`, `Gen.Example` |
| 3 | A model for `biggest/1`, invariants of `sort`, generalizing `last`, symmetry of encode and decode | Basic generators | Phase 1 |
| 4 | The default generator table, `prop_biggest` (fails on the empty list, then `non_empty`), shrinking `[-1,6,3,1]` to `[0,1]`, `prop_sort`, `prop_last`, exercises Q3 to Q7 (`lists:seq`, `keysort`, fixing the `sets:union` model, `dict:merge`, `word_count`) | Equivalents of PropEr's default generators, `NonEmpty`, `Any` | Phase 1. `term()` maps to an `Any()` over primitive and composite types; the sort examples use typed values such as `[]int` |
| 5 | `collect` and `aggregate` with buckets, `resize`, `?SIZED`, generators as functions, `?LET`, `?SUCHTHAT`, `frequency` (`text_like`), recursion with `?LAZY` (`wordy_string`), deterministic sized recursion, symbolic calls (`dict_symb`), exercises Q4 to Q6 (terminating trees, pairs of timestamps, generating a file handle) | Statistics, size, `Bind`, `Filter`, `Frequency`, `Lazy`, readable counterexamples | Phase 1. Labeled draws in the report replace symbolic calls: the report shows the inputs used to build a value, not only the final value |
| 6 | The CSV round trip, the `"\r\na"` counterexample, the one-column ambiguity (use `Size+1` columns), nested `?LET` sharing a size between header and records, `date()`, example tests next to properties, coverage | Basic features, replay | Phase 1. Example tests are plain `testing` tests in the same file; coverage comes from `go test -cover` |
| 7 | Layered `?LET` (price list, then items and the expected total), `elements(PriceList)`, `?SIZED` recursion, `collect` buckets, negative tests (a result or a specific error), generating duplicates (`vector(length+1, elements)`), the discovery that 92% of the runs had invalid input | `Bind`, statistics, `Cover` | Phase 1. Errors are Go `error` values; `Recover` turns a panic into a value |
| 8 | Re-centering with `?SHRINK` (years to 1970, time-zone minutes to 0, 15, 30, 45), `?LETSHRINK` (tree to subtree, `A ++ B ++ C`, the meal of Q3, independent shrinking in the checkout of Q4), `sampleshrink` | Shrinking control and observation | Phase 1. `ShrinkTo`, `ShrinkToward`, `LetShrink`, `SampleShrink` |
| 9 | The statem callbacks, the cache model, history and state in `?WHENFAIL`, `aggregate(command_names)`, an injected bug, `parallel_commands`, exposing a race with `erlang:yield()`, `+T9` | `Command`, history output, command-name statistics, parallel execution with an interleaving check | Phases 2 and 3. `runtime.Gosched()` together with `-race` |
| 10 | The bookstore on PostgreSQL, ISBN generators, `friendly_unicode`, intent labels through a shim (`new`, `exists`, `avail`), `partial/1`, preconditions that return false so that shrinking works, `--start_size`, where to start and stop the system (Q3), the caveat of using real results in `next_state` (Q4) | Precondition re-evaluation while shrinking, `Setup`, `Var`, `-pbt.startsize` | Phase 2. The shim labels become part of the command name and show up in the statistics |
| 11 | `proper_fsm`: state names, `weight/3`, `precondition/4`, `postcondition/5`, `next_state_data/5`, statistics by `state_names`, `?TRAPEXIT`, the circuit breaker (ok, tripped, blocked) and the model fixes | The `fsm` package | Phase 2 |

Some Erlang-specific constructs take a different form in Go.

- Atoms become string constants or typed constants.
- `term()` becomes an `Any()` with a bounded set of types. Comparison and
  ordering are defined per type in Go, so the examples that use `list(term())`
  use typed values.
- Symbolic calls serve two purposes in the book: readable counterexamples and
  shrinking of the arguments. Labeled draws in the report cover the first;
  the choice sequence itself covers the second.
- `?TRAPEXIT` becomes panic recovery. pbt does not detect leaked goroutines.
- Scheduler perturbation with `+T9` becomes inserted `runtime.Gosched()` calls
  plus `-race`.
- `rebar3 proper` is `go test`. pbt has no scaffolding like
  `rebar3 new proper base`.

The bookstore of Chapter 10 runs on the pure-Go SQLite implementation
[modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite), so that
`go test` alone is enough, and on PostgreSQL through the pgx driver when
`PBT_BOOKSTORE_DSN` names a database. Of the bugs the book finds, the `LIKE`
wildcards (`%` and `_`) and the cap on `available` reproduce on SQLite. The
handling of NUL bytes is driver-specific and does not. The README of the
chapter states what reproduces. The circuit breaker of Chapter 11 is a small
implementation in `examples/` that mirrors the API of Klarna's
`circuit_breaker`. It has the three states ok, tripped and blocked, thresholds
for errors and timeouts, and manual block, deblock and reset. Existing Go
libraries assume time-based recovery and have no manual block, so they do not
fit the book's model.

## Repository and module setup

The root module has no external dependencies; the book ports form a separate
module.

```text
go.ymotongpoo.com/pbt        root module, no external dependencies
├── go.mod
├── LICENSE.md               Apache License 2.0
├── fsm/                     the equivalent of proper_fsm (Chapter 11)
└── examples/                separate module go.ymotongpoo.com/pbt/examples with the book ports
    └── go.mod
```

The root `go.mod` reads as follows. `go mod init` writes the version of the
local toolchain (1.27.1), so the repository pins the directive at 1.25 by hand.

```text
module go.ymotongpoo.com/pbt

go 1.25
```

For the import path `go.ymotongpoo.com/pbt`, the URL
`https://go.ymotongpoo.com/pbt?go-get=1` must return HTML that contains a
`go-import` meta tag. `go get` and the module proxy (proxy.golang.org) fetch
that URL and clone the repository the tag names. To fetch the subpackage
`go.ymotongpoo.com/pbt/fsm`, the go command requests `/pbt/fsm?go-get=1` and
then `/pbt?go-get=1` to confirm the prefix `go.ymotongpoo.com/pbt`. Every path
under `/pbt` therefore returns the same HTML.

The site is a second Cloudflare Pages project next to www.ymotongpoo.com,
with `go.ymotongpoo.com` as its custom domain. The custom-domain setup creates
the DNS CNAME. The files live in the orphan branch `pages` of this repository,
which shares no history with `main`:

```text
_redirects
pbt/index.html
index.html        for the root; carries no go-import tag
README.md         what the branch is and how to deploy it
```

Connect the Pages project to this repository with `pages` as the production
branch, or run `wrangler pages deploy` from a checkout of the branch. With the Git integration, disable preview deployments for other
branches so that `main` is never published as a site.

`_redirects` rewrites every path under `/pbt` to the same HTML. Cloudflare
Pages supports status 200 rewrites (proxying) in `_redirects`, and a rule may
contain one splat (`*`).

```text
/pbt    /pbt/index.html 200
/pbt/*  /pbt/index.html 200
```

`pbt/index.html` is shown below. The repository is `github.com/ymotongpoo/pbt`.
The `go-source` tag lets pkg.go.dev link to the source and does not affect
fetching. The page redirects human visitors to pkg.go.dev.

```html
<!DOCTYPE html>
<html lang="ja">
<head>
<meta charset="utf-8">
<meta name="go-import" content="go.ymotongpoo.com/pbt git https://github.com/ymotongpoo/pbt">
<meta name="go-source" content="go.ymotongpoo.com/pbt https://github.com/ymotongpoo/pbt https://github.com/ymotongpoo/pbt/tree/main{/dir} https://github.com/ymotongpoo/pbt/blob/main{/dir}/{file}#L{line}">
<meta http-equiv="refresh" content="0; url=https://pkg.go.dev/go.ymotongpoo.com/pbt">
<title>go.ymotongpoo.com/pbt</title>
</head>
<body><a href="https://pkg.go.dev/go.ymotongpoo.com/pbt">pkg.go.dev/go.ymotongpoo.com/pbt</a></body>
</html>
```

The same meta tag resolves `examples/` as the separate module
`go.ymotongpoo.com/pbt/examples`. The go command looks for `examples/go.mod`
in the repository, and its tags take the form `examples/v0.1.0`.

Verify the setup after publishing, once a tag such as `v0.1.0` exists.
proxy.golang.org fetches a version on first request and caches it, so a
re-pushed tag needs a new version number.

```sh
curl -sL 'https://go.ymotongpoo.com/pbt?go-get=1' | grep go-import
cd "$(mktemp -d)" && go mod init example.com/check && go get go.ymotongpoo.com/pbt@latest
```

pkg.go.dev renders documentation only for modules whose license it
recognizes; Apache License 2.0 qualifies. A major version 2 or later changes
the module path to `go.ymotongpoo.com/pbt/v2`.

## Decisions

The decisions below date from 2026-10-31.

| Item | Decision |
| --- | --- |
| Position on Hypothesis | The disagreement is with keeping distribution control from users, not with the engine. Shrinking uses choice sequences; distribution control and distribution checks are design principles |
| `testing.F` | Neither reused nor connected |
| Where failures are saved | `testdata/pbt/<test name>/<hash>`, replayed by every `go test` run |
| Supported versions | The latest stable release and the two before it; Go 1.25 and later at present |
| Name and import path | Package `pbt`, module path `go.ymotongpoo.com/pbt` |
| License | Apache License 2.0 |
| Compatibility layer for rapid | None |
| Acceptance target | Every example and exercise in the book can be ported |

Two details remain open: the database for the Chapter 10 port (SQLite by
default, as proposed above) and the value set of `Any()`. Both can wait
until implementation.

The next document is the Phase 0 specification. It covers:

- the representation of the choice sequence,
- the span model,
- the order of the shrink passes and their stopping conditions,
- the file format under `testdata/pbt/`.
