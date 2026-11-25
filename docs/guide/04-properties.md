# Properties and runs
<!-- summary: Options and flags, discards, setup and cleanup, property combinators, several failures with MaxFailures and Signature, seeds and stale replay, search time budgets, JSON run reports and OpenPBTStats observations for Tyche. -->

This chapter covers the shape of a run: how many cases, which sizes, how discards are handled, what runs before and after a case, how properties combine, and how a failure is reproduced.

## Options and flags

Most settings have a flag for the whole `go test` run and an option for one `Check` call. Options win over flags, except for the health checks, where the flags and the options add up.

| Flag | Option | Default | Meaning |
| --- | --- | --- | --- |
| `-pbt.checks` | `Checks(n)` | 100 | passing cases required |
| `-pbt.seed` | `Seed(s)` | random | seed of the run, printed in every report |
| `-pbt.startsize`, `-pbt.maxsize` | `Sizes(start, max)` | 1, 42 | size of the first case and the size the cases grow to |
| `-pbt.budget` | `Budget(d)` | 0 (unlimited) | elapsed time allowed for search, excluding replay and shrinking |
| `-pbt.shrinktime` | `ShrinkTime(d)` | 30s | time budget for shrinking one failure |
| `-pbt.failures` | `MaxFailures(n)` | 1 | distinct failures to collect, shrink and report |
| `-pbt.causality` | `Causality(n)` | 0 (off) | continuations per prefix to locate a possible cause in a sequential program (chapter 5) |
| `-pbt.coverage` | `CoverageGuided()` | false | use live block coverage to guide search (chapter 8) |
| `-pbt.json` | `ReportJSON(path)` | empty (off) | append one JSON report per run |
| `-pbt.observe` | `Observations(dir)` | empty (off) | append OpenPBTStats records, one file per test |
| `-pbt.searchsteps` | `SearchSteps(n)` | 1000 | cases of a targeted search |
| `-pbt.nosave` | `NoSave()` | false | do not save failing cases |
| `-pbt.replay` | `ReplayFile(path)` | none | a saved case to replay before searching |
| `-pbt.v` | `Verbose()` | false | log the summary of a passing run and every shrink step |
| | `TestdataDir(dir)` | `testdata/pbt` | where failing cases are saved |
| | `NoReplay()` | false | skip the saved cases |
| | `Setup(f)` | none | run `f` before every case |
| | `CheckCoverage()` | false | fail on an unmet or undecided `Cover` requirement |
| `-pbt.warnhealth` | `WarnHealth(checks...)` | none | enable optional health warnings, currently `HealthDuplicate` |
| `-pbt.checkhealth` | `CheckHealth(checks...)` | none | health checks that fail a passing run (chapter 2) |
| `-pbt.ignorehealth` | `IgnoreHealth(checks...)` | none | health checks to hide |
| | `ReportLimit(chars)`, `ReportDepth(levels)` | 2000, 3 | size of a reported value and depth of nested draws |

<!-- snippet: properties_test.go#options -->

`MakeCheck` wraps a property as a `func(*testing.T)`, which is convenient for a table of properties or for a subtest.

## Time budgets

`Budget(d)` or `-pbt.budget=d` limits the elapsed search time. The default, zero, leaves search unlimited by time. The clock starts after saved-case replay. pbt checks it before each new case, including discarded cases, targeted search steps and extended `CheckCoverage` runs.

The case limit still applies, so `Budget` does not promise a minimum runtime. Raise `Checks` to run for roughly the requested duration. For a property that calls `Maximize` or `Minimize`, raise `SearchSteps` instead.

```sh
go test -count=1 -run '^TestQueue$' -pbt.checks=1000000 -pbt.budget=30s
```

A running case finishes before the budget takes effect. Use `go test -timeout` for a property that can hang. Replay and shrinking fall outside this budget; `ShrinkTime` separately bounds shrinking each failure or witness.

A `Check` can pass with fewer than `Checks` cases when time runs out. Collected failures still fail, shrink and appear in the report. Enabled health checks and `CheckCoverage` can also fail the run; an undecided coverage requirement remains a failure. `CheckExists` fails if it finds no witness.

A passing run that exhausts its budget always prints a summary, even without `Verbose`. `TestSearchBudget` uses `testing/synctest` to run three one-second cases without a real wait:

```text
[pbt] OK, passed 3 checks in the 3s budget (seed 0x1)
```

## Discarding cases

`t.Assume(cond)` discards the current case when the condition is false, as PropEr's `?IMPLIES` does. A discarded case counts neither as a pass nor as a failure, and `Check` draws another. `Skip`, `Skipf` and `SkipNow` do the same. A failure already recorded in the case takes precedence over a later discard.

<!-- snippet: properties_test.go#assume -->

Discards are a budget. When they exceed ten times the number of checks, the run fails with the reason of the last discard, because a property that discards most of its cases is testing less than it claims. Prefer a generator that produces valid input, `IntMin(1)` over `Assume(b > 0)`, whenever one exists.

## Setup and cleanup

The `Setup` option runs a function before every case. It may return a teardown function, which runs after the case. `t.Cleanup` registers a function from inside the property for the same purpose, and the cleanups run in reverse order after the case, failed or not.

Every cleanup runs even if another cleanup panics or discards. A cleanup failure fails the case even if the property discarded it; a cleanup discard cannot erase a failure or panic from the property. If both the property and a cleanup panic, the report keeps the property's panic.

<!-- snippet: properties_test.go#setup -->

## Combining properties

`Conjunction` runs several named properties on the same case and names the ones that fail. A later sub-property that discards the case does not erase an earlier sub-property's failure. `Deadline` fails a case that takes longer than a duration.

<!-- snippet: properties_test.go#combinators -->

A failure of a conjunction names its members:

```text
[pbt] error: non-empty: empty
[pbt] error: conjunction: [non-empty] failed
```

Go cannot interrupt a running function, so `Deadline` checks the elapsed time after the property returns. A property that never returns is caught by `go test -timeout`. `Timeout` is the former name of `Deadline` and still works.

## Explaining a failure

`t.OnFail(f)` registers a function that runs only for the case that is reported, after shrinking. Anything it logs appears in the report, which makes it the place to print the state of the system under test or the intermediate values of a computation, as PropEr's `?WHENFAIL` does.

<!-- snippet: properties_test.go#onfail -->

## Panics

A panic inside a property fails the case. The report shows the panic value and the frames of your code, and the shrinker looks for the smallest case that panics at the same place. Two helpers cover the properties that are about panics themselves.

<!-- snippet: properties_test.go#panics -->

`Recover` runs a function and returns what it panicked with, or nil, so a property can assert that invalid input panics. `NoPanic` takes a function and generates its arguments with `Make`, which chapter 9 describes; the `WithType` and `WithField` options narrow the arguments.

## Several failures in one run

A run normally stops at the first failure, shrinks it and reports it. A function with two bugs then shows them one at a time: you fix the first and run again to meet the second. `MaxFailures(n)` or `-pbt.failures=n` keeps the run going after a failure. It collects up to `n` failures that fail in different ways, shrinks each one, saves each one and reports each one.

<!-- snippet: failures_test.go#failures -->

```text
[pbt] failure 1 of 2: property failed after 6 passed checks and 9 shrink steps (size 4)
[pbt] s (StringOf(SampledFrom(10 values))) = "0"
[pbt] error: parsePort("0") accepted 0
[pbt] reproduce: go test -run 'TestPort' -pbt.seed=0x1 -pbt.failures=2
[pbt] failure 2 of 2: property failed after 6 passed checks and 90 shrink steps (size 8)
[pbt] s (StringOf(SampledFrom(10 values))) = "7000"
[pbt] error: parsePort("7000") rejected a valid port
[pbt] reproduce: go test -run 'TestPort' -pbt.seed=0x1 -pbt.failures=2
```

By default, two failures are different when they fail at different lines, or panic with different types or at different places. That is the same identity that keeps a shrink on one bug (chapter 3). A failure that the shrinker meets on the way, failing in a new way, becomes one of the `n` as well. Each failure gets the full `ShrinkTime` budget. The second minimum, 7000, is a local one. The true boundary is 6554, but the path to it needs a larger digit in a later place, and the shrinker does not try that. Saved failures replay together, and a run that replays a failing saved case does not search for more. `CheckExists` ignores the option.

When several causes share one assertion, call `t.Signature` with a stable name for each cause. `MaxFailures` then collects the names separately, even though every failure reaches the same line. The [named port example in chapter 3](03-shrinking.md#the-failure-stays-the-same) separates an invalid port that the parser accepts from a valid port that it rejects.

## Seeds, saved cases and replay

Every report ends with a `reproduce:` line that carries the seed of the run. Passing that seed with `-pbt.seed` repeats the same sequence of cases, including the failing one, on the same version of both the property code and pbt. Updating pbt can change the generated cases for a seed. In v0.3.2 the integer generation fix changes some seeded runs. In v0.4.0, `Make` generates different values for some recursive types, and the shrinker can reach other, smaller cases. A saved case replays its choices through the generators of the installed version. After an upgrade, a saved case of a property that uses `Make` on a recursive type can produce a different value.

The failing case is also saved under `testdata/pbt/<TestName>/<hash>` as a text file with the size, the drawn values as comments, and the choices. Every later `Check` of the same test replays the saved cases before generating new ones and fails if one still fails, so the regression case follows the code into version control. Keep the file as a regression case after fixing the bug. Removing it drops that replay; do not delete it just to make a run pass. Three options adjust this behavior: `NoSave` keeps a run from writing, `NoReplay` skips the saved cases, and `ReplayFile` or `-pbt.replay=FILE` replays one saved file in any test, which is how you hand a case from one test to another or reproduce a failure from a CI log.

`HealthStale` warns by default when saved draw comments differ from the current replay's draws. A changed generator can turn the same choices into another input. Compare the warning's saved and current text, then test the original value explicitly if it still matters. A passing stale replay does not establish that the original regression is fixed.

The comparison uses rendered text, not deep equality. Truncation can hide changes, and pbt skips draws with unstable representations such as pointers or custom formatters. `CheckHealth(HealthStale)` or `-pbt.checkhealth=stale` makes the warning fail a passing run. `IgnoreHealth(HealthStale)` hides it. Replay does not rewrite the saved comments.

## Machine-readable output

`ReportJSON(path)` or `-pbt.json=path` appends one JSON line after each `Check` or `CheckExists`. The default empty path disables output. Create the parent directory first. The schema name is `go.ymotongpoo.com/pbt/report/v1`; readers should ignore unknown fields.

| Fields | Contents |
| --- | --- |
| `test`, `seed`, `kind`, `status`, `errors` | test name, hexadecimal seed, `check` or `exists`, outcome and error messages |
| `checks_passed`, `rejected`, `discards` | search counts, excluding replay and shrinking; `rejected` counts failing candidates |
| `budget_exhausted`, `durations_seconds` | whether time ended the search, plus total, search and shrink durations |
| `failures`, `witness` | failure identities, rendered draws, saved paths and reproduction commands; `witness` appears only after successful `CheckExists` |
| `health`, `statistics`, `objectives` | diagnostics, labels, coverage requirements, measurements and best utilities |

`status` is `passed`, `failed`, `gave_up`, `no_witness` or `error`. Draw values are rendered strings and follow `ReportLimit` and `ReportDepth`. Non-finite numbers use the strings `NaN`, `+Inf` and `-Inf`.

<!-- snippet: run_output_test.go#machine-output -->

`TestMachineReadableOutput` checks both files. This is its JSON report with durations, statistics and empty collections omitted; all shown fields retain their captured values:

```json
{"schema":"go.ymotongpoo.com/pbt/report/v1","test":"TestRecordedRun","seed":"0x1","kind":"check","status":"passed","checks_passed":3,"rejected":0,"discards":0,"budget_exhausted":false}
```

`Observations(dir)` or `-pbt.observe=dir` appends OpenPBTStats records that Tyche reads. The default empty directory disables observations. pbt creates the directory and writes one file per sanitized test name, such as `observations/TestRecordedRun.jsonl`. Subtest names create subdirectories. Sanitized names can collide; `property` and `run_start` distinguish records in a shared file.

Each `test_case` record includes the input, outcome, generation phase, features and timings. Features include labels, measurements, coverage-requirement hits and target utilities. Timings separate input generation from test execution. Replay and shrink attempts also produce records. A `minimal` record with `metadata.final=true` is a snapshot, not another execution. An `info` record ends the run. The `coverage` field is currently null, even with `CoverageGuided`.

```sh
go test -count=1 ./... -pbt.json=runs.jsonl -pbt.observe=observations
```

A log shipper can tail these files and forward records or derived counts to a log or metrics backend. pbt writes files locally and does not send them over the network. Observations use buffering and flush at the end of a run; abrupt termination can lose buffered records. A write error fails the test, because the requested output was not delivered.
