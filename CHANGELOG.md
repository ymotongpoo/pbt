# Changelog

The notable changes of pbt are recorded here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/). The project uses
[Semantic Versioning](https://semver.org/) with a major version of 0, so a
minor version may change the API.

## Unreleased

### Fixed

- `TestGoURLDivergences` in the package `format` failed on Go 1.25.0 and
  1.25.1, which the module supports. It expected `net/url.Parse` to reject
  IPvFuture literals such as `http://[v1.x]/`, which Go rejects only from
  Go 1.25.2, where `url.Parse` started to accept only an IPv6 address in
  brackets. The test now expects the behaviour of the running toolchain,
  and the documentation of `RFC3986URI` names the release. The generators
  are unchanged.

## 0.5.0 - 2026-11-25

### Added

- `Causality(n)` and `-pbt.causality=N`, which explain a failing
  stateful test. The first failing program is rewound to prefixes of its
  commands before shrinking, each prefix is continued `n` times with
  fresh choices, and the report lists how often the same failure recurs
  after each prefix. The largest rise is bisected down to one step, which
  names the command after which the failure became likely, even when a
  later command detected it. Sequential `Commands` and `fsm` programs are
  analysed; parallel programs and properties without commands are
  skipped with a note. The analysis costs at most `16*n` runs.
- `T.Signature(name)`, which names the identity of a case's failure. The
  name replaces the site of the assertion or the panic for shrinking, for
  grouping under `MaxFailures`, for the names of saved files and for
  `HealthFlaky`, so that one assertion that covers several causes can
  report them apart.
- `HealthStale`: a saved case whose recorded draws differ from the draws
  of its replay is reported, because the generator changed and the replay
  tests another input. It warns by default.
- `HealthDuplicate`, which warns when fewer than half of at least 20
  generated cases are distinct, and `WarnHealth` and `-pbt.warnhealth`,
  which turn it on as a warning. It is off by default, because a property
  over a small domain is often intended.
- `Budget(d)` and `-pbt.budget`, which stop generating cases once `d` has
  passed since the search began. A run cut short passes with the cases it
  ran and says so in its summary; replay and shrinking are outside the
  budget.
- `ReportJSON(path)` and `-pbt.json`, which append one JSON line per
  `Check` or `CheckExists` under the schema
  `go.ymotongpoo.com/pbt/report/v1`: the status, the counts and
  durations, each failure with its identity, draws, saved file and
  reproduce command, the health notes, the statistics and the objectives.
- `Observations(dir)` and `-pbt.observe`, which write one record per
  evaluated case in the OpenPBTStats format that
  [Tyche](https://github.com/tyche-pbt/tyche-extension) and Hypothesis's
  observability use, one file per test.
- `CoverageGuided()` and `-pbt.coverage`, an experimental search for
  `go test -cover`: a passing case that raises `testing.Coverage()`
  becomes a seed, and later cases alternate between fresh cases and
  mutations of the seeds. Without instrumentation the option logs a hint
  and the search is unchanged.

### Changed

- A saved case that has gone stale logs a `HealthStale` warning when it
  is replayed. Nothing else changes for a run that uses none of the new
  options: the cases, their order and the reports are the same as in
  0.4.0 for the same seed.

## 0.4.0 - 2026-11-24

### Added

- The package `go.ymotongpoo.com/pbt/format`, which generates strings in
  the formats that standards define: timestamps and durations of RFC 3339,
  ISO 8601, RFC 9110 and RFC 5322, IP, MAC and host name text, e-mail
  addresses of RFC 5321, RFC 5322, RFC 6531 and the HTML standard, URIs of
  RFC 3986, HTTP tokens, fields, media types and cookies, UUIDs, JSON and
  I-JSON, JSON Pointers, CSV, the encodings of RFC 4648, Semantic
  Versioning, BCP 47 language tags, E.164 numbers, Luhn-checked card
  numbers and ISBNs. A function named after a standard, such as
  `RFC5321Mailbox`, covers its production for testing parsers; a plain
  name, such as `Email`, generates the common form. `Time(layout)` takes
  the layout constants of package time. Every value is built from the
  grammar, so shrinking keeps it valid.
- Generators of standard library types: `Time`, `TimeRange`, `Duration`,
  `DurationRange`, `FixedZone` and `Zone` for package time, which lean
  toward the edges of the calendar and the changes of a zone's offset;
  `IPv4`, `IPv6`, `Addr`, `AddrIn`, `Prefix` and `AddrPort` for
  `net/netip`, with the special-purpose blocks of RFC 6890; and
  `HardwareAddr`.
- `RuneIn`, which draws runes from the range tables of package unicode,
  and `StringMatching`, which builds strings that match a regular
  expression as a whole.
- Health checks: `HealthCheck` with `HealthFilter`, `HealthLarge`,
  `HealthSlow`, `HealthFlaky` and `HealthShrink`. A passing run warns
  about wasted, oversized or slowly generated input; a failing report
  notes a failure that did not recur on replay and a shrink that ran out
  of budget. `CheckHealth` and `-pbt.checkhealth` turn warnings into
  failures, and `IgnoreHealth` and `-pbt.ignorehealth` hide them.
- `MaxFailures` and `-pbt.failures`, which collect, shrink, save and
  report up to n failures that fail in different ways.
- `Maximize` and `Minimize` accept several names per case; the targeted
  search takes turns between the objectives.
- `BoundedGrowth` and `UnboundedGrowth`, which share the growth allowance
  of `Make` among the collections of a hand-written generator, or exempt
  a part from it.
- `Make` and `cmd/pbtgen` generate `time.Time`, `*time.Location`,
  `netip.Addr`, `netip.Prefix`, `netip.AddrPort`, `net.IP` and
  `net.HardwareAddr` with the new generators, and take `min` and `max`
  RFC 3339 tags on `time.Time` fields.

### Changed

- A report names an unlabeled draw at the top level of a property after
  the variable it is assigned to, as in `start (Time()) = ...`, or after
  its file and line, as in `Int() at prop_test.go:12 = 5`. `Verbose` adds
  the line to every draw. Tests that match the text of reports may need
  updating.
- A failing `Check` replays the failing case once before shrinking, so a
  property and its cleanups run one more time on a failure.
- The shrinker can switch an alternative of `OneOf` or `Frequency`, or
  shorten a length drawn first, and reconnect the choices that follow.
  Counterexamples for an existing seed can be smaller than in 0.3.4.
- `Make` bounds the total work of a value: one value spends at most
  `8*max(size, 1)` units of growth across all its collections, strings,
  map entries and pointers, and recursive references stay nil or empty
  once the allowance is spent. Values that used more than that, and some
  deeply recursive values, differ for an existing seed; other values stay
  as they were. `Make[time.Time]` and the other standard types above are
  no longer zero.
- `Maximize` and `Minimize` no longer panic when a case reports a second
  utility name.

To upgrade from 0.3: tests that assert on the text of a report with an
unlabeled draw, such as `Int() = 5`, now see the variable name or the file
and line; a property with side effects runs once more when it fails;
`WithType` generators for `time.Time` and the netip types can be dropped.

## 0.3.4 - 2026-11-14

### Fixed

- The shrinker tries the same candidates in the same order for the same
  seed. Two passes ranged over a map: the pass that sorts sibling spans
  took the groups of a parent in Go's random map order, and the pass that
  lowers duplicate choices sorted its groups with ties left in map order.
  When a counterexample had more than one sortable group, two runs with
  the same seed could shrink along different paths and stop at different
  counterexamples. Both passes now use a fixed order: groups in the order
  of their first span, and duplicate groups by value, kind and the rest of
  their metadata. Generation is unchanged, so the first failure for a seed
  is the same as before; the shrink path can differ from v0.3.3.

## 0.3.3 - 2026-11-01

### Fixed

- `Float64Range` and `Float32Range` stay within their bounds when the
  whole range lies beyond ±2^53, such as `Float64Range(1e308,
  math.MaxFloat64)`. The bounds were converted to `int64` before they were
  clipped, which is implementation-defined for values outside the `int64`
  range, so on amd64 the integral form could draw values such as 0. Other
  ranges generate the same values for an existing seed as before.

## 0.3.2 - 2026-11-01

### Fixed

- `Conjunction` preserves an earlier failure when a later sub-property
  discards the case.
- Cleanup callbacks cannot replace a panic from the property with a
  discard or another panic. All callbacks still run, and a cleanup failure
  takes precedence over a discard.
- Wide integer generation uses value bits independent of the width
  selector, restoring access to previously unreachable high-bit patterns.
  This changes some generated cases for an existing seed; saved choice
  sequences still replay as before.
- `NoShrink` validates protected draws after replay, so deleting choices
  cannot change a retained value. Collections can still drop protected
  elements.
- `Rune` includes the maximum Unicode scalar value, `U+10FFFF`.

## 0.3.1 - 2026-11-01

### Added

- `CommandGen`, an alias of `Gen[Command]`, for the signature of a function
  that returns a command generator: Go infers the type argument in a call
  to `Map` or `Frequency` but not in a declaration. `Commands`,
  `ParallelCommands`, `NoCommands` and `fsm.Transition` are written with
  it, which changes no type.

## 0.3.0 - 2026-10-31

### Changed

- The stateful API has no type parameters. `Command` is a struct of
  functions that close over the model and the system under test, which
  live in variables of the test; `Commands(init, gen)` and
  `ParallelCommands(init, gen)` take a function that puts the model in its
  initial state and a generator of the next command for the current state;
  `RunSequential(t)` and `RunParallel(t)` take no system. pbt replays a
  sequence by calling `init` and then the `Next` of every command, so the
  model is changed in place instead of being copied.
- Package `fsm` follows: `fsm.Commands(initial, init, transitions)` takes
  the name of the first state, an init function for the state data and a
  function from a state name to its transitions. `Transition` has no type
  parameters and state names are strings. Commands are named
  "from -> to: name" in the program and in the statistics.

### Added

- `W(weight, gen)`, which pairs a weight with a generator for `Frequency`
  and infers the type a `Weighted` literal would spell out.
- `Length(min, max)`, `Branches(n, maxLen)` and `ModelState(f)`, options of
  `Commands` and `ParallelCommands`, which replace `CommandsN` and
  `ParallelCommandsN` and let the report show the model.

### Removed

- `Cmd`, the `Command` interface, `CommandName`, `CommandsN`,
  `ParallelCommandsN`, `fsm.State` and `fsm.Machine`.

To upgrade a stateful test from 0.2: move the model and the system into
variables of the property, drop the type arguments and the `state` and
`sys` parameters of the command functions, make `Next` update the model in
place, write an `init` function that resets it, and pass neither the
initial state to `Commands` nor the system to `RunSequential`. Chapters 5
to 7 of the guide show the result.

## 0.2.0 - 2026-10-31

### Added

- `AnyFloat64`, which generates NaN and the two infinities alongside the
  finite values of `Float64`.
- `Deadline`, the new name of `Timeout`; `Timeout` stays as a deprecated alias.
- `ReportDepth`, which sets how many levels of nested labeled draws a report
  lists; the default stays at three.
- `ReplayFile` and the `-pbt.replay` flag, which replay one saved test case
  before searching, whatever test saved it, and `NoReplay`, which skips the
  cases saved under `testdata/pbt`.
- `-prefix` in `cmd/pbtgen`, which names the generated functions.
- This changelog.

### Changed

- Shrinking keeps the identity of the failure it started from: the site of
  the assertion, the type and site of the panic, or the check and command of
  a stateful run. A candidate that fails in another way is not adopted, so a
  panic no longer shrinks into an unrelated assertion that a smaller input
  trips. A `Conjunction` keeps the identity of the first sub-property that
  failed.
- `Cover` requirements are decided from the 99.7% Wilson interval of the
  observed share. Under `CheckCoverage`, `Check` keeps running cases, up to
  twenty times the configured number, until every requirement is decided, and
  fails when one is still undecided. A case counts once per label however
  often it calls `Cover`, the highest required share wins, and a share
  outside (0, 100] panics.
- A report lists a labeled draw after the value it contributes to and nests
  it by labeled draws only, so an unlabeled generator in between no longer
  pushes it past the depth limit. A draw that did not finish is marked.
- Size 0 gives empty collections, and `Sizes` accepts a start size of 0.
- `Maximize` and `Minimize` panic when a case reports a second utility name.
- `SampledFrom` copies its slice; `Just`, `Fn` and `Make` document what they
  share between cases and when they may be called.
- `Resolve` names the wanted type in its message and resolves a nil result
  to the zero value.
- `cmd/pbtgen` reads the files that `go build` compiles on the current
  platform, leaving out files excluded by build constraints, and rejects a
  generated name the package already declares.

## 0.1.0 - 2026-10-31

The first release: generators with shrinking over a choice sequence, size
and distribution control with statistics, stateful and parallel command
testing with symbolic variables, the `fsm` package, targeted search and
existential properties, `Make` and `cmd/pbtgen` for generators derived from
types, failures saved under `testdata/pbt` and replayed, and a port of every
chapter of *PropEr Testing* under `examples/`.
