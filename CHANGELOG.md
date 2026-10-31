# Changelog

The notable changes of pbt are recorded here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/). The project uses
[Semantic Versioning](https://semver.org/) with a major version of 0, so a
minor version may change the API.

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
