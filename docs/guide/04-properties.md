# Properties and runs
<!-- summary: The options and flags of a run, Assume and discards, Setup and Cleanup, Conjunction, Deadline, OnFail, Recover and NoPanic, and how seeds, saved cases and replay work. -->

This chapter covers the shape of a run: how many cases, which sizes, how discards are handled, what runs before and after a case, how properties combine, and how a failure is reproduced.

## Options and flags

Every setting has a flag for the whole `go test` run and an option for one `Check` call. Options win over flags.

| Flag | Option | Default | Meaning |
| --- | --- | --- | --- |
| `-pbt.checks` | `Checks(n)` | 100 | passing cases required |
| `-pbt.seed` | `Seed(s)` | random | seed of the run, printed in every report |
| `-pbt.startsize`, `-pbt.maxsize` | `Sizes(start, max)` | 1, 42 | size of the first case and the size the cases grow to |
| `-pbt.shrinktime` | `ShrinkTime(d)` | 30s | time budget for shrinking one failure |
| `-pbt.searchsteps` | `SearchSteps(n)` | 1000 | cases of a targeted search |
| `-pbt.nosave` | `NoSave()` | false | do not save failing cases |
| `-pbt.replay` | `ReplayFile(path)` | none | a saved case to replay before searching |
| `-pbt.v` | `Verbose()` | false | log the summary of a passing run and every shrink step |
| | `TestdataDir(dir)` | `testdata/pbt` | where failing cases are saved |
| | `NoReplay()` | false | skip the saved cases |
| | `Setup(f)` | none | run `f` before every case |
| | `CheckCoverage()` | false | fail on an unmet or undecided `Cover` requirement |
| | `ReportLimit(chars)`, `ReportDepth(levels)` | 2000, 3 | size of a reported value and depth of nested draws |

<!-- snippet: properties_test.go#options -->

`MakeCheck` wraps a property as a `func(*testing.T)`, which is convenient for a table of properties or for a subtest.

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

## Seeds, saved cases and replay

Every report ends with a `reproduce:` line that carries the seed of the run. Passing that seed with `-pbt.seed` repeats the same sequence of cases, including the failing one, on the same version of both the property code and pbt. Updating pbt can change the generated cases for a seed. In v0.3.2 the integer generation fix changes some seeded runs; existing saved choice sequences still replay as before.

The failing case is also saved under `testdata/pbt/<TestName>/<hash>` as a text file with the size, the drawn values as comments, and the choices. Every later `Check` of the same test replays the saved cases before generating new ones and fails if one still fails, so the regression case follows the code into version control. Delete the file to drop the case. Three options adjust this behavior: `NoSave` keeps a run from writing, `NoReplay` skips the saved cases, and `ReplayFile` or `-pbt.replay=FILE` replays one saved file in any test, which is how you hand a case from one test to another or reproduce a failure from a CI log.
