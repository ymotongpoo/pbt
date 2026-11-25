# Targeted search and existence
<!-- summary: Maximize and Minimize to steer a run toward utilities, searching for slow inputs through measured work or duration, experimental CoverageGuided search under go test -cover, and CheckExists for witnesses. -->

Random inputs reach the common states of a system quickly and the rare ones slowly or never. Targeted property-based testing adds a utility function: the property reports how close a case came to something interesting, and pbt searches for cases with a higher utility instead of drawing fresh ones. PropEr calls it `?MAXIMIZE`.

## Maximize

<!-- snippet: targeted_test.go#maximize -->

A random walk of `n` steps ends about `sqrt(n)` away from the origin, so 100 random walks almost never reach 50. With `t.Maximize("position", pos)`, the run turns into a search once the first case reports a utility. pbt mutates the choices of the best case so far, keeps a mutation when its utility is higher, sometimes keeps a worse one early in the search to escape a local maximum, and occasionally restarts from a fresh case. The search runs for the number of steps set by `SearchSteps` or `-pbt.searchsteps`, 1000 by default, instead of the number of checks.

The report below shortens the list of steps with `…` and omits the reproduction line.

```text
[pbt] property failed after 246 passed checks and 2039 shrink steps (size 33)
[pbt] utility position = 51
[pbt] steps (SliceOf(SampledFrom(2 values))) = []int{1, 1, 1, 1, …}
[pbt] error: the walk reached 51
```

The search found a walk past 50 after 246 cases, and the shrinker then reduced it to 51 steps to the right, the smallest input that fails. When the property passes, the summary reports the best utility seen. `t.Minimize` searches in the other direction. A second `Maximize` with the same name replaces the value of the case.

Use the search when a property has a notion of "closer": the depth of a tree, the length of a queue, the number of retries, the distance to a boundary. A utility that is flat over most inputs gives the search nothing to climb.

## Several objectives

A case can report several utilities under different names. The search then keeps a current and a best case for each objective, and the objectives take turns. Each step mutates the current case of one objective, in the order in which the objectives first appeared. It judges the mutation by that objective's utility alone.

<!-- snippet: targeted_test.go#objectives -->

```text
[pbt] property failed after 41 passed checks and 3005 shrink steps (size 26)
[pbt] utility queue = 1
[pbt] utility wait = 101
[pbt] jobs (SliceOf(IntRange(1, 10))) = []int{3, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 10, 1}
[pbt] workers (IntRange(1, 4)) = 1
[pbt] error: a job waited 101
[pbt] reproduce: go test -run 'TestPool' -pbt.seed=0x1
```

The report lists every utility of the failing case. A passing run lists the best value of each objective in its summary. The objectives share the search steps, so give a run with several of them more steps with `SearchSteps`. A property with a single objective searches exactly as before.

## Searching for slow inputs

A performance property can maximize the elapsed duration of the call under test. Draw the input first, record `time.Now()` immediately before the call, then pass `time.Since(start).Seconds()` to `t.Maximize("seconds", elapsed)`. This keeps generation time out of the utility.

Elapsed time is noisy. Scheduling, garbage collection and caches can change it for the same input. Repeat a slow case before treating it as evidence of a performance bug. A deterministic work count often makes a better search objective.

The guide measures comparisons inside a quadratic scan. Inputs with an early duplicate finish quickly; inputs with many distinct values need more comparisons.

<!-- snippet: search_test.go#slow-input -->

`TestSearchFindsExpensiveScan` runs this property with `Seed(1)`. Its report is:

```text
[pbt] property failed after 194 passed checks and 213 shrink steps (size 42)
[pbt] utility comparisons = 21
[pbt] xs (SliceOfN(IntRange(0, 9), 0, 10)) = []int{0, 1, 2, 3, 5, 4, 4}
[pbt] error: scan used 21 comparisons, limit 20
[pbt] reproduce: go test -run 'TestScanWork' -pbt.seed=0x1
```

Elapsed time works the same way in a real project: maximize the measured duration and assert the acceptable bound. `Maximize` alone does not fail a case. A passing search reports its best utility but saves no counterexample. The bound makes a slow case fail so that normal runs shrink and save it. The guide's test uses `NoSave` through its recorder to avoid writing the planted failure into the suite.

## Searching with code coverage

`CoverageGuided()` or `-pbt.coverage` enables an experimental search based on `testing.Coverage`. It is off by default. Build with `go test -cover`, and use `-coverpkg` to select the packages whose coverage matters. Without instrumentation, pbt logs a hint and uses ordinary search.

Passing generated cases that increase live block coverage enter a pool of up to 64 seeds. Cases without choices cannot seed mutations. Once the pool has a seed, attempts alternate between a fresh case and a mutation of a randomly selected seed. The pool drops the oldest seed when full and lasts for one `Check` call. Mutations use the seed's size and count toward `Checks` like fresh cases.

This can help with deep chains of comparisons. A case that passes one more comparison reaches another block and can seed attempts at the next comparison. The example recognizes a four-byte header:

<!-- snippet: coverage_fixture.go#coverage-fixture -->

<!-- snippet: search_test.go#coverage-guided -->

The fixture lives in `coverage_fixture.go`, because Go does not instrument `_test.go` files. The test skips itself when the binary has no instrumentation. Run it alone from the repository root:

```sh
go test -count=1 -v -cover -coverpkg=go.ymotongpoo.com/pbt/docs/guide/snippets ./docs/guide/snippets -run '^TestCoverageGuidedHeader$'
```

The captured output includes:

```text
[pbt] coverage-guided: 4 of 200 passing cases reached new code, 4 seeds kept, coverage 0.0% -> 60.0%
[pbt] OK, passed 200 checks (seed 0x1)
```

The percentages are the live estimate from `testing.Coverage`. They differ from the statement-weighted percentage that `go test` prints at the end. Compiler instrumentation and earlier tests can change the output, so the test checks the report's shape rather than fixed percentages.

Coverage guidance has limits:

- Counters cover the whole process. Earlier tests can exhaust the useful signal, while parallel tests and other goroutines can attribute unrelated coverage to a case.
- The signal describes covered blocks, without edges or execution counts. Comparisons within an already covered block give no new signal.
- Each reading scans the counters, so its cost grows with the instrumented code. Select the packages under test; instrumenting pbt also includes its own work in the signal.
- A seed alone cannot repeat the schedule across different coverage histories. Saved counterexamples still replay their choices normally.

Run a property alone when using this signal, and avoid `t.Parallel`. `Maximize` or `Minimize` takes priority once targeted search starts, with its own `SearchSteps` limit. `CheckExists` ignores coverage guidance.

## Existence

`CheckExists` inverts the question: instead of a property that must hold for every case, it looks for one case where the property holds. A case that fails is a rejected candidate, and the first case that passes is the witness, which pbt shrinks and logs.

<!-- snippet: targeted_test.go#exists -->

```text
[pbt] witness found after 98 rejected cases and 29 shrink steps (seed 0x1) (size 42)
[pbt] a (IntRange(1, 50)) = 3
[pbt] b (IntRange(1, 50)) = 4
[pbt] c (IntRange(1, 70)) = 5
```

The test fails when no witness turns up within the number of checks. `Maximize` works inside `CheckExists` too: a utility that measures the distance to a passing case guides the search toward one. Nothing is saved under `testdata/pbt` for an existential property, since a witness is not a regression.
