# Testing a project with an agent
<!-- summary: A workflow for coding agents: derive properties from the project's contract, choose oracles and generators, inspect distributions, run longer searches, replay and triage failures, and report bugs with evidence. -->

Start with a small set of properties whose failures would matter to the project. Keep each property, its source of expected behaviour, and its results together. A useful result is a reproducible disagreement with the project's contract, supported by a test that preserves the evidence.

## Establish the contract

Read the documentation, public types, existing tests and fixtures for the code under test. Look for standards that the project promises to follow. These sources define valid inputs, allowed errors and expected results. Record disagreements between them before choosing an oracle.

Basic rules can also supply properties: valid requests should not panic, stored data should not disappear, and operations should finish. State the limits of each rule. For example, a documented panic on invalid input does not violate a promise about valid input.

Choose a few places where a bug would cause the most harm. For each, write down the input domain, the expected result, and the source that requires it. Quote the relevant sentence when one exists. If the contract remains ambiguous, identify the missing decision instead of inventing a requirement.

## Choose an oracle

An oracle decides whether the observed result is correct. Start with an inexpensive one, then add an independent check where it would catch a different kind of error.

| Oracle, roughly by effort | Property and pbt tools |
| --- | --- |
| No panic | Use [NoPanic](04-properties.md#panics) to generate function arguments from types, or call the function inside `Check`; a panic fails the case. |
| Round trip | Generate a value with `ForAll` or `Draw`, encode it, decode it, and compare with `Fatalf`. Account for documented normalization. See [generators](01-generators.md) and [formats](10-formats.md). |
| Reference model | Keep a small model of expected state and compare operations through [Commands and postconditions](05-stateful.md). Reset the system for each case. |
| Invariants after every step | Check conservation, ordering or ownership in each command's `Post`. Remember that `Post` sees the model before `Next`. |
| Metamorphic check | Draw one input, transform it, and compare the related results in one property. For example, a documented order-independent operation should survive a permutation. |
| Differential check | Send the same drawn input to two independent implementations and compare results with `Fatalf`. First establish that their contracts agree on that input domain. |

For a possible hang, set `go test -timeout`. The [Deadline combinator](04-properties.md#combining-properties) checks elapsed time only after the call returns. A difference between two implementations is evidence to investigate, not proof that either particular implementation is wrong.

## Build inputs that exercise the contract

Prefer [generators that construct valid inputs](01-generators.md) over a broad generator followed by `Filter` or `Assume`. Use `Bind` for dependent fields and `Frequency` for the mix of alternatives. Include boundaries from existing tests and fixtures alongside generated values. Keep invalid-input properties separate so that their expected errors remain explicit.

Use the [format package](10-formats.md) for standard text inputs. Check whether the project accepts the full standard grammar or only its common form. Use [Make or pbtgen](09-types.md) for Go types, with field overrides for constraints that types cannot express.

Generate [command sequences](05-stateful.md) for stateful code. Independent calls rarely reach states such as expired leases or a closed queue with pending work. Change the model only in `init` and `Next`; let the generator and checks read it. Reset external state, clocks and resources for every execution, including shrinking and diagnostic replays.

Measure the distribution with [Classify, Cover and CheckCoverage](02-distribution.md). Label important operations and boundary cases. Require a minimum share when missing that class would invalidate the experiment. Inspect health warnings before increasing the number of checks. Enable `WarnHealth(HealthDuplicate)` when repeated choices would waste a large search.

## Run in stages

First compile and run the affected package with the default 100 checks. Inspect the statistics and any warnings. Then increase the case count and try several recorded seeds. The following commands assume a property named `TestQueue` in the current package:

```sh
go test -count=1 -run '^TestQueue$' -pbt.v
go test -count=1 -run '^TestQueue$' -pbt.checks=10000 -pbt.seed=1
go test -count=1 -run '^TestQueue$' -pbt.checks=10000 -pbt.seed=2
go test -count=1 -run '^TestQueue$' -pbt.checks=1000000 -pbt.budget=30s -pbt.seed=3
```

[Budget](04-properties.md#time-budgets) caps search time, while the case limit can end it earlier. For targeted properties, increase `-pbt.searchsteps`. Neither setting interrupts a blocked call.

Use `-pbt.failures=3` to collect several causes in one run. Assign stable [Signature names](03-shrinking.md#the-failure-stays-the-same) when causes share an assertion site. Read `-pbt.json=runs.jsonl` for structured run results instead of parsing console text. Use `-pbt.observe=observations` for [OpenPBTStats records that Tyche reads](04-properties.md#machine-readable-output).

Try [CoverageGuided](08-targeted.md#searching-with-code-coverage) when nested comparisons make random progress difficult:

```sh
go test -count=1 -run '^TestQueue$' -cover -coverpkg=./... -pbt.coverage -pbt.checks=10000
```

Select the packages under test with `-coverpkg`. Run the property alone and avoid parallel tests because coverage counters belong to the whole process. Coverage guidance does not supply an oracle; the property still needs assertions.

## Investigate a failure

Keep the saved case under `testdata/pbt` and run the report's `reproduce:` command. Use `-pbt.replay=FILE` to replay a particular saved case. Record the Go, pbt and project versions: generator changes can change the meaning of saved choices. Inspect a `HealthStale` warning before treating replay as the original input.

Add the minimal input to an ordinary regression test when that makes the defect easier to inspect. Confirm that it fails against the unchanged code. For a sequential stateful failure, [Causality](05-stateful.md#finding-the-step-that-caused-a-failure) can suggest an earlier command that made the failure likely. Its sampled rates guide investigation; they do not establish the cause by themselves.

| Verdict | Signs | Next action |
| --- | --- | --- |
| Real bug | A valid input produces a result that contradicts the recorded contract. | Preserve the reproducer and report the mismatch. Fix the implementation only within the requested scope. |
| Test bug | The generator violates a precondition, the model leaks state, or the comparison assumes an undocumented rule. | Correct that specific mistake, record the reason, and rerun the affected properties. |
| Intended behaviour | Documentation or an existing test explicitly permits this result. | Adjust the expectation to the contract and keep a test of the documented behaviour. |
| Environment | The result depends on a clock, platform, service, missing tool or resource limit. | Record the dependency, control it where possible, and rerun before assigning the defect to the library. |

Check related existing findings and issues to avoid reporting the same cause twice. Preserve the original observation when a controlled rerun behaves differently. State what remains uncertain.

## Extend a passing test

A passing run covers the generated cases and the assertions that ran. Use its evidence to choose the next change:

- Inspect labels and command statistics for missing shapes or states. Change weights or add a generator branch, then measure again.
- Inspect code coverage with `go test -cover -coverprofile=coverage.out`, then `go tool cover -func=coverage.out`. Trace uncovered paths back to missing inputs or operations.
- Add a new oracle on the same code. A round trip can pass while both directions share a bug; an independent model may reveal it.
- Count what `Assume`, `Filter` and explicit exclusions remove. Check whether an exclusion hides the cases that matter.
- Raise `-pbt.maxsize` for longer inputs or programs, while watching generation cost. Try another seed and record the result.
- Move to another part of the project when further changes no longer improve the tested inputs or assertions.

## Preserve the evidence

Never weaken or delete a property to make a failure disappear. Correct a mistaken oracle only with evidence from the contract. Never delete a saved case to make a run pass. Do not change the code under test until a failing test demonstrates the defect.

A bug report includes the tested versions, minimal input, exact commands, expected result and actual result. Include the documentation sentence that requires the expected behaviour. If a type or standard supplies the rule, identify that source instead. Report how many cases ran, which properties failed, the triage verdict, and any untested scope. Keep the reproducer with the report so that another person can check the finding.
