# Migrating from rapid

pbt and [rapid](https://github.com/flyingmutant/rapid) share the same
engine design: generators are deterministic functions of a sequence of
choices, and shrinking rewrites the sequence. Most rapid tests translate
line by line. This page lists the correspondences and the differences.

## Running properties

| rapid | pbt |
| --- | --- |
| `rapid.Check(t, func(t *rapid.T) {...})` | `pbt.Check(t, func(t *pbt.T) {...})` |
| `rapid.MakeCheck(prop)` | `pbt.MakeCheck(prop)` |
| `rapid.MakeFuzz(prop)` | not provided; see the design document |
| `-rapid.checks`, `-rapid.seed`, `-rapid.shrinktime` | `-pbt.checks`, `-pbt.seed`, `-pbt.shrinktime` |
| `-rapid.failfile` (a temporary file) | failures are saved under `testdata/pbt/<Test>/` and replayed by `go test`; `-pbt.replay=FILE` or `pbt.ReplayFile(path)` replays one saved file in any test, and `pbt.NoReplay()` skips them |
| `-rapid.v`, `-rapid.debug`, `-rapid.log` | `-pbt.v` or `pbt.Verbose()` logs the summary and every accepted shrink step |
| `t.Skip`, `t.SkipNow` | the same, or `t.Assume(cond)` |

Options exist per call too: `pbt.Checks`, `pbt.Seed`, `pbt.Sizes`,
`pbt.ShrinkTime`, `pbt.SearchSteps`, `pbt.Setup`, `pbt.CheckCoverage`,
`pbt.NoSave`, `pbt.TestdataDir`, `pbt.ReplayFile`, `pbt.NoReplay`,
`pbt.Verbose`, `pbt.ReportLimit`, `pbt.ReportDepth`.

Shrinking in pbt keeps the kind of failure it started from, the assertion
site or the panic site, so the counterexample reports the bug the search
found rather than a simpler, unrelated one.

## Generators

| rapid | pbt |
| --- | --- |
| `*rapid.Generator[V]` | `pbt.Gen[V]` (a value, not a pointer) |
| `g.Draw(t, "label")` | `g.Draw(t, "label")`; a label inside a `Custom` generator also appears in the report |
| `g.Example(seed)` | `g.Example(seed)` |
| `g.Filter(f)` | `pbt.Filter(g, f)` |
| `g.AsAny()` | `pbt.AsAny(g)` widens a generator to `Gen[any]`; `pbt.Any()` is different: it generates values of common types |
| `rapid.Int()`, `IntRange`, `IntMin`, `IntMax` | the same names |
| `rapid.Int8Range` and the other sized variants | `pbt.IntegerRange[int8](lo, hi)`, `pbt.Integer[int8]()`; `Int32`, `Int64`, `Uint`, `Uint32`, `Uint64`, `Byte` keep their names |
| `rapid.Float64Range(lo, hi)` | `pbt.Float64Range(lo, hi)`; the full range is reachable, including the extremes |
| `rapid.Float64()` | `pbt.Float64()`, which is finite; `pbt.AnyFloat64()` adds NaN and the infinities |
| `rapid.Bool()`, `Rune()`, `String()`, `StringN`, `StringOf`, `StringOfN` | the same names |
| `rapid.StringMatching(regexp)` | not provided; build the string from its parts |
| `rapid.SliceOf`, `SliceOfN`, `SliceOfDistinct`, `SliceOfNDistinct` | the same names; `SliceOfDistinct` takes the key function last |
| `rapid.MapOf`, `MapOfN` | the same names |
| `rapid.MapOfValues` | `pbt.Map(pbt.SliceOfDistinct(val, key), ...)` |
| `rapid.ArrayOf` | `pbt.Make[[N]E]()` or a `Custom` loop |
| `rapid.Permutation(xs)` | `pbt.Permutation(xs)` |
| `rapid.SampledFrom(xs)`, `rapid.Just(v)` | the same names |
| `rapid.OneOf(gs...)` | `pbt.OneOf(gs...)`; `pbt.Frequency` adds weights |
| `rapid.Custom(func(t) V)` | `pbt.Custom("name", func(t) V)`; the name appears in reports |
| `rapid.Map(g, f)` | `pbt.Map(g, f)`; `pbt.Bind(g, f)` when f returns a generator |
| `rapid.Deferred(func() *Generator)` | `pbt.Lazy(func() pbt.Gen[V])`; `pbt.Recursive` and `pbt.Bounded` bound recursive types |
| `rapid.Ptr(elem, allowNil)` | `pbt.Ptr(elem, allowNil)` |
| `rapid.Make[V]()`, `rapid.MakeCustom[V](cfg)` | `pbt.Make[V](opts...)` with `pbt` struct tags, `pbt.WithType`, `pbt.WithField`; `cmd/pbtgen` writes the same as source |

pbt adds what rapid leaves to the engine: `Sized` and `Resize` expose the
size parameter, `Frequency` weights alternatives, `ShrinkTo`,
`IntRangeToward`, `LetShrink` and `NoShrink` steer shrinking, and
`SampleShrink` shows a shrink path. `Collect`, `Classify`, `Aggregate`,
`Measure` and `Cover` report and check the distribution, `Conjunction`,
`Deadline` and `OnFail` compose properties, and `Func` generates functions.

## Stateful tests

| rapid | pbt |
| --- | --- |
| `t.Repeat(map[string]func(*rapid.T))` with `""` as the invariant | `pbt.Commands(init, gen)` draws a `Program`; `prog.RunSequential(t)` runs it |
| `rapid.StateMachineActions(sm)` | a `pbt.CommandGen`, the generator of `pbt.Command` values for the current model state |
| actions that `t.SkipNow()` when not applicable | the `Pre` function of `pbt.Command` |
| checks inside the action | `Post`, which receives the real result and an `Env` |
| the Go struct that holds both model and system | the same: variables of the test that the commands close over, with an `init` function that resets the model |
| results the model needs before execution | `pbt.Var`, resolved with `pbt.Resolve` |
| no equivalent | `pbt.ParallelCommands` and `RunParallel` (an interleaving check, as in PropEr), package `fsm` for named states |

In rapid the actions run as they are generated; in pbt the whole sequence
is generated first against the model, then executed. That is what lets
shrinking remove a command and regenerate the following commands from the
new model state, checking their preconditions during generation, and
what makes parallel execution possible. It is also why the model may only
change in `init` and in the `Next` of a command: pbt replays a sequence by
calling them again. A false `Pre` retries a command during generation; if
no allowed command is found within the retry limit, the sequence ends or
the case is discarded when its minimum length has not been reached. During
`RunSequential`, a false `Pre` fails the case rather than skipping an action.

## Searching

`pbt.CheckExists` looks for a case on which the property holds, and
`t.Maximize` or `t.Minimize` turn a run into a targeted search; rapid has
no equivalent.
