# Shrinking
<!-- summary: How a failing case shrinks, steering the search with ShrinkTo and IntRangeToward, LetShrink and NoShrink, watching a shrink with SampleShrink, and why the kind of failure stays the same. -->

When a case fails, pbt looks for a simpler case that fails in the same way, and reports that one. The search is automatic for every generator, including the ones you write with `Custom`, `Map` and `Bind`. This chapter explains what "simpler" means, how to steer the search, and what it will not do.

## How a case shrinks

Every value a property draws is a function of a sequence of choices, and pbt records that sequence for each case. The shrinker edits the sequence: it deletes the choices that belong to one draw, lowers numbers, moves a group of choices toward an earlier alternative, reorders elements, and then regenerates the values from the edited sequence. A candidate replaces the current case when the property still fails and the sequence is shorter, or as long and lexicographically smaller. The passes repeat until none of them makes progress or the shrink time, 30 seconds by default, runs out.

Because the shrinker works on choices and not on values, nothing special is needed for a value built by `Map` or `Bind`: the shrinker edits the choices underneath and the function regenerates the value. The direction of the search follows from what each generator does with small choices.

| Generator | Shrinks toward |
| --- | --- |
| `Int`, `Float64` and the other numbers | 0, or the bound of the range nearest to 0 |
| `IntRangeToward(lo, hi, center)` | `center`, from either side |
| `Bool` | `false` |
| `String`, `SliceOf`, `MapOf` | fewer elements, then simpler elements |
| `SampledFrom`, `OneOf`, `Frequency` | the first value or alternative |
| `Just` | nothing to shrink |
| `Custom`, `Map`, `Bind` | whatever the draws inside shrink toward |

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

<!-- snippet: shrinking_test.go#noshrink -->

## Watching a shrink

`SampleShrink` returns the path a value takes when it shrinks, as PropEr's `sampleshrink` does. It is the quickest way to check that a custom generator shrinks toward what you expect.

<!-- snippet: shrinking_test.go#sampleshrink -->

In a real run, `Verbose()` or `-pbt.v` logs every accepted shrink step with the values it produced.

## The failure stays the same

A case records how it first failed: the line of the assertion, or the type and location of the panic. During shrinking, a candidate is accepted only when it fails in the same way. The rule protects you from a common trap.

<!-- snippet: shrinking_test.go#identity -->

Suppose the first failing case is a quantity of 3000, which panics. Without the rule, the shrinker would lower the quantity past 1000, find that 101 fails the assertion, and report a wrong discount; the panic would be gone from the report. With the rule, the quantity shrinks to 1001 and the report shows the panic. When the first failure is the wrong discount, the case shrinks to 101 instead. Either way, the report describes the bug the run found. A stateful test keys its failures by command and check, so a failing postcondition of one command cannot turn into the panic of another.

## What shrinking will not do

The shrinker reaches a local minimum, not always the global one: it only accepts candidates that are simpler by its own measure, so a smaller case that needs a larger intermediate step stays out of reach. `LetShrink` and `ShrinkTo` exist for the cases where the structure of a value hides the simpler form. The size parameter stays fixed while a case shrinks, and the time budget is 30 seconds per failure; `ShrinkTime` or `-pbt.shrinktime` change it.
