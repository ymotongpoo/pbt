# Targeted search and existence
<!-- summary: Maximize and Minimize to steer a run toward a utility, and CheckExists for properties that ask whether a witness exists. -->

Random inputs reach the common states of a system quickly and the rare ones slowly or never. Targeted property-based testing adds a utility function: the property reports how close a case came to something interesting, and pbt searches for cases with a higher utility instead of drawing fresh ones. PropEr calls it `?MAXIMIZE`.

## Maximize

<!-- snippet: targeted_test.go#maximize -->

A random walk of `n` steps ends about `sqrt(n)` away from the origin, so 100 random walks almost never reach 50. With `t.Maximize("position", pos)`, the run turns into a search once the first case reports a utility. pbt mutates the choices of the best case so far, keeps a mutation when its utility is higher, sometimes keeps a worse one early in the search to escape a local maximum, and occasionally restarts from a fresh case. The search runs for the number of steps set by `SearchSteps` or `-pbt.searchsteps`, 1000 by default, instead of the number of checks.

```text
[pbt] property failed after 246 passed checks and 1835 shrink steps (size 33)
[pbt] utility position = 51
[pbt] steps (SliceOf(SampledFrom(2 values))) = []int{1, 1, 1, 1, …}
[pbt] error: the walk reached 51
```

The search found a walk past 50 after 246 cases, and the shrinker then reduced it to 51 steps to the right, the smallest input that fails. When the property passes, the summary reports the best utility seen. `t.Minimize` searches in the other direction. A case has one utility: a second `Maximize` with the same name replaces the value, and one with another name is an error.

Use the search when a property has a notion of "closer": the depth of a tree, the length of a queue, the number of retries, the distance to a boundary. A utility that is flat over most inputs gives the search nothing to climb.

## Existence

`CheckExists` inverts the question: instead of a property that must hold for every case, it looks for one case where the property holds. A case that fails is a rejected candidate, and the first case that passes is the witness, which pbt shrinks and logs.

<!-- snippet: targeted_test.go#exists -->

```text
[pbt] witness found after 2748 rejected cases and 14 shrink steps (seed 0x83d3334683123f8d) (size 42)
[pbt] a (IntRange(1, 50)) = 3
[pbt] b (IntRange(1, 50)) = 4
[pbt] c (IntRange(1, 70)) = 5
```

The test fails when no witness turns up within the number of checks. `Maximize` works inside `CheckExists` too: a utility that measures the distance to a passing case guides the search toward one. Nothing is saved under `testdata/pbt` for an existential property, since a witness is not a regression.
