# Shrinking
<!-- summary: How a failing case shrinks, writing generators that shrink well, steering the search with ShrinkTo and IntRangeToward, LetShrink and NoShrink, watching a shrink with SampleShrink, why the kind of failure stays the same, naming causes with Signature, and what a failure that does not recur means. -->

When a case fails, pbt looks for a simpler case that fails in the same way, and reports that one. The search is automatic for every generator, including the ones you write with `Custom`, `Map` and `Bind`. This chapter explains what "simpler" means, how to steer the search, and what it will not do.

## How a case shrinks

Every value a property draws is a function of a sequence of choices, and pbt records that sequence for each case. The shrinker edits the sequence: it deletes the choices that belong to one draw, lowers numbers, moves a group of choices toward an earlier alternative, reorders elements, and then regenerates the values from the edited sequence. A candidate replaces the current case when the property still fails and the sequence is shorter, or as long and lexicographically smaller. The shrinker also lowers the choice that picks an alternative, and a length that a generator draws before its elements. The new value can then take fewer choices than the old one. The shrinker reconnects the choices that followed the old value, so the draws after it keep their values. The passes repeat until none of them makes progress or the shrink time, 30 seconds by default, runs out.

Because the shrinker works on choices and not on values, nothing special is needed for a value built by `Map` or `Bind`: the shrinker edits the choices underneath and the function regenerates the value. The direction of the search follows from what each generator does with small choices.

| Generator | Shrinks toward |
| --- | --- |
| `Int`, `Float64` and the other numbers | 0, or the bound of the range nearest to 0 |
| `IntRangeToward(lo, hi, center)` | `center`, from either side |
| `Bool` | `false` |
| `String`, `SliceOf`, `MapOf` | fewer elements, then simpler elements |
| `SampledFrom`, `OneOf`, `Frequency` | the first value or alternative, when it takes no more choices than the current one |
| `Just` | nothing to shrink |
| `Custom`, `Map`, `Bind` | whatever the draws inside shrink toward |

## Writing generators that shrink well

The shrinker edits a span of choices as a unit: the choices of one draw, with the draws nested in it. Two habits keep the spans useful.

- Write an alternative as a generator that draws the whole branch, and choose between such generators with `OneOf` or `Frequency`. Avoid drawing an index first and then a `switch` on it. That puts the branch's choices outside the span of the index, and switching the branch disturbs the draws that follow.
- Draw a length first and the elements after it, inside one `Custom`. Lowering the length then removes the last elements and keeps what follows the generator.

<!-- snippet: shrinkwell_test.go#shrink-well -->

From every seed, the report shows `alice@example.com` and 4 retries, whichever contact the failing case had at first. An alternative wins only when it takes no more choices than the current one. Here the e-mail address takes one choice and the phone number two. A first alternative that draws more than the others can stay out of reach. The case then shrinks within the alternative it started from.

## Steering the search

Sometimes the simplest value by that table is not the most readable one. A date is easier to think about in 1970 than in the year 0, and an offset of a few minutes is more telling than one of 720. Two generators change the target.

<!-- snippet: shrinking_test.go#shrinkto -->

`ShrinkTo(g, alts...)` generates from `g` and, when shrinking, tries the alternatives in order. It is the equivalent of PropEr's `?SHRINK`. A property that fails for every year from 1990 on reports 1990, the lowest failing year inside the first alternative that still fails:

```text
[pbt] year (ShrinkTo(IntRange(0, 9999))) = 1990
```

`IntRangeToward` makes a range shrink toward a center instead of its lower bound. A property that fails outside ±30 minutes reports 31 or -31.

## Shrinking a composite to one of its parts

A value built from parts normally keeps its shape while the parts shrink: a concatenation of three slices stays a concatenation of three slices. `LetShrink` records one more choice, whether to combine the parts or to return one of them. Generation always combines; shrinking can flip the choice, so the composite shrinks to a single part. PropEr calls this `?LETSHRINK`.

<!-- snippet: shrinking_test.go#letshrink -->

A property that fails on two or more elements reports `[]int{1, 1}`, with the other two parts gone.

## Keeping a value as it is

`NoShrink` marks a value that the shrinker must leave alone. Use it for opaque tokens and for inputs whose shape is fixed by the system under test, where shrinking would only spend time.

A collection can still drop an element generated with `NoShrink`; the elements that remain keep their values.

<!-- snippet: shrinking_test.go#noshrink -->

## Watching a shrink

`SampleShrink` returns the path a value takes when it shrinks, as PropEr's `sampleshrink` does. It is the quickest way to check that a custom generator shrinks toward what you expect.

<!-- snippet: shrinking_test.go#sampleshrink -->

In a real run, `Verbose()` or `-pbt.v` logs every accepted shrink step with the values it produced.

## The failure stays the same

A case records how it first failed: the line of the assertion, or the type and location of the panic. During shrinking, a candidate is accepted only when it fails in the same way. The rule protects you from a common trap.

<!-- snippet: shrinking_test.go#identity -->

Suppose the first failing case is a quantity of 3000, which panics. Without the rule, the shrinker would lower the quantity past 1000, find that 101 fails the assertion, and report a wrong discount; the panic would be gone from the report. With the rule, the quantity shrinks to 1001 and the report shows the panic. When the first failure is the wrong discount, the case shrinks to 101 instead. Either way, the report describes the bug the run found. A stateful test keys its failures by command and check, so a failing postcondition of one command cannot turn into the panic of another.

One assertion site can cover several causes. `t.Signature(name)` assigns a stable name to the cause for the current case. This version of the [port property in chapter 4](04-properties.md#several-failures-in-one-run) uses one `Fatalf` for both bugs:

<!-- snippet: diagnostics_test.go#signature -->

Running it with `Seed(1)` and `MaxFailures(2)` produces:

```text
[pbt] failure 1 of 2: property failed after 6 passed checks and 9 shrink steps (size 4)
[pbt] failure was named "accepted invalid port"
[pbt] s (StringOf(SampledFrom(10 values))) = "0"
[pbt] error: parsePort("0"): accepted invalid port
[pbt] reproduce: go test -run 'TestNamedPort' -pbt.seed=0x1 -pbt.failures=2
[pbt] failure 2 of 2: property failed after 6 passed checks and 90 shrink steps (size 8)
[pbt] failure was named "rejected valid port"
[pbt] s (StringOf(SampledFrom(10 values))) = "7000"
[pbt] error: parsePort("7000"): rejected valid port
[pbt] reproduce: go test -run 'TestNamedPort' -pbt.seed=0x1 -pbt.failures=2
```

The name replaces the assertion site, panic type and site, or stateful command check as the failure identity. Shrinking keeps that name, and `MaxFailures` counts different names as different failures. The same identity determines saved-file suffixes when collecting several failures and the replay comparison for `HealthFlaky`.

Choose names for causes, without generated values. Otherwise each value can become a separate failure and prevent shrinking. Without `Signature`, the automatic identity still applies. A call before or after a failure works; the last call wins, including deferred calls and cleanup callbacks. `Conjunction` shares one case, so a signature in any member names the whole case. A passing case ignores the name.

## When a failure does not recur

Before shrinking, `Check` replays the failing case once with the same choices. A property that depends only on its input fails again, in the same way. The replay can pass, discard the case or fail at another place. Then the property or a generator depends on something besides the choices, such as a clock, a global variable or the order of goroutines. The report then carries a `HealthFlaky` note.

<!-- snippet: flaky_test.go#flaky -->

```text
[pbt] property failed after 55 passed checks and 3 shrink steps (size 42)
[pbt] key (IntRange(0, 9)) = 4
[pbt] error: cache miss for key 4
[pbt] reproduce: go test -run 'TestCache' -pbt.seed=0x1
[pbt] health: HealthFlaky: the failure did not recur when its input was replayed (the replay passed); the original failure was at flaky_test.go:21 (IgnoreHealth(HealthFlaky) hides it)
```

`Check` still shrinks and saves the case, since a failure that recurs only sometimes, such as a race in chapter 6, is still a failure. The note tells you that the shrunk case may not fail again when you replay it. Fix the property before you trust its reports. A parallel test of chapter 6 that catches a race can carry the note, and there it is expected.

## What shrinking will not do

The shrinker reaches a local minimum, not always the global one: it only accepts candidates that are simpler by its own measure, so a smaller case that needs a larger intermediate step stays out of reach. `LetShrink` and `ShrinkTo` exist for the cases where the structure of a value hides the simpler form. The size parameter stays fixed while a case shrinks, and the time budget is 30 seconds per failure; `ShrinkTime` or `-pbt.shrinktime` change it. Shrinking can stop at the budget or at its fixed limit of 200,000 steps. The report then carries a `HealthShrink` note, because a smaller case may exist.
