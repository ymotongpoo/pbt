# Size, distribution and statistics
<!-- summary: The size parameter, weights with Frequency, Sized and Resize, the statistics Collect, Classify, Aggregate, Measure and Cover, CheckCoverage, which turns coverage into a check, and the health checks that warn about wasted or oversized input. -->

pbt lets you decide what the data looks like, and gives you the tools to check that it looks the way you intended. Both halves matter: a property that never reaches the interesting states of a system proves nothing about them.

## The size parameter

Every test case runs at a size. `Check` starts at size 1 and raises it by one for each passing case, up to 42, so a run begins with small inputs and ends with large ones. The lengths of strings, slices and maps average the size; the magnitude of integers grows with it too. `t.Size()` returns the size of the current case, and the `Sizes(start, max)` option or the `-pbt.startsize` and `-pbt.maxsize` flags change the range.

`Sized` builds a generator from the size, which is how PropEr's `?SIZED` works, and `Resize` runs a generator at a fixed size. Together they bound a recursive structure.

<!-- snippet: distribution_test.go#sized -->

Halving the size at every level gives a tree whose depth is at most the logarithm of the size, whatever the branching. For size-sensitive collection generators such as `SliceOf`, `Resize(0, g)` produces an empty collection unless a positive minimum length is required. `Resize` fixes only the size parameter, so the length bounds stated by `SliceOfN` still hold: `Resize(0, SliceOfN(Int(), 1, 3))` gives one element.

## Weights

`Frequency` draws one of its alternatives with a probability proportional to its weight.

<!-- snippet: distribution_test.go#frequency -->

Weights only affect generation. Shrinking always moves toward the first alternative, whatever the weights are, so the order of the alternatives is the shrinking priority. An alternative with weight 0 is never generated and is reachable only by shrinking, which is what `ShrinkTo` in chapter 3 is built on. `OneOf` is `Frequency` with equal weights.

## Statistics

Four methods on `*pbt.T` describe the cases of a run. `Check` prints the result when the property passes.

<!-- snippet: distribution_test.go#statistics -->

- `Classify(cond, label)` records a label when the condition holds, and `Collect(v)` records the text of a value as a label. Both report a share of passing cases as long as a case records each label once. They do not remove repeats, so a case that records a label twice counts twice and a share can exceed 100%. Use `Aggregate` for what happens several times in a case. `Cover` counts a case once per label.
- `Aggregate(values...)` counts samples, several per case, and reports their share of all samples. It is the right call for the names of the commands a stateful test ran.
- `Measure(name, value)` records a number and reports its minimum, average and maximum.

```text
[pbt] OK, passed 100 checks (seed 0x1)
[pbt]  22.0% read-only batch
[pbt]  81.1% get
[pbt]  18.9% put
[pbt] batch length: min 1, avg 12.43, max 20 (100 samples)
```

The shares confirm the four-to-one weighting, and the length line confirms that batches of the full length occur. `Verbose()` or `-pbt.v` prints the summary even when the property records nothing.

## Checking coverage

`Cover(cond, pct, label)` turns an observation into a requirement: the cases for which `cond` holds must make up at least `pct` percent of all passing cases. The example above requires a write in 60% of the batches.

For requirements below 100%, the decision is statistical, since a run of 100 cases can miss a 60% share by chance. pbt computes the 99.7% confidence interval of the observed share, the Wilson score interval. The requirement is met when the interval's lower bound reaches `pct`, unmet when its upper bound is below `pct`, and undecided otherwise. A requirement of 100% is not statistical: it is met while every passing case records a hit for the label and unmet as soon as one does not. Without the `CheckCoverage` option, `Check` logs a shortfall and passes. With it, `Check` keeps running cases past the configured number, up to twenty times as many, until every requirement is decided. It then fails on a shortfall, and it also fails when a requirement is still undecided, so a pass means that the data supports the share.

<!-- snippet: distribution_test.go#cover-fails -->

```text
[pbt] insufficient coverage:
[pbt]   round hundred: 17.6% of 125 cases, required 30.0%
```

The run stopped after 125 cases, once the interval excluded 30%. The observed share also says something about `IntRange`: 0 is a boundary value, so it occurs far more often than one case in a thousand. Statistics are how you find that out, and `Frequency` or `SampledFrom` are how you change it when a uniform spread is what the property needs.

## Health checks

Statistics answer the questions you ask. Health checks answer the ones you might not think of: whether the run spends its cases well. `Check` watches every run and warns about wasted input, expensive generation and changes to saved inputs. An optional check also detects repeated choices.

| Check | Warns when |
| --- | --- |
| `HealthFilter` | half of the cases, and at least 20, are discarded by `Assume`, `Skip` or a `Filter` that gave up; or a `Filter` rejects nine values in ten over at least 100 |
| `HealthLarge` | a fifth of the cases draw more than 10,000 choices |
| `HealthSlow` | drawing the values takes 10 ms per case and a second in all, not counting the property's own work |
| `HealthStale` | a saved case's draw comments differ from the draws produced by replay; warns by default |
| `HealthDuplicate` | fewer than half the inputs have distinct choice sequences over at least 20 non-discarded random cases; off by default |

A property that keeps only long strings shows the first one.

<!-- snippet: health_test.go#health -->

```text
[pbt] health: HealthFilter: 629 of 729 cases were discarded; most by Assume at health_test.go:18 (629) (CheckHealth(HealthFilter) makes this fail; IgnoreHealth(HealthFilter) hides it)
```

The run passed, but it ran 729 cases to get its 100, and the warning names the line that threw the rest away. A generator that builds long strings, `StringN(8, 40)`, removes the waste. The warning names the busiest sources: the description of a `Filter`, or the line of an `Assume` or a `Skip`. `HealthLarge` names the draws that consumed the most choices.

All health checks except `HealthDuplicate` warn by default, the way `Cover` does, and the run passes. `CheckHealth()` makes `HealthFilter` and `HealthLarge` fail a passing run. `CheckHealth(HealthSlow)` adds the timing check, which is left out by default because a loaded CI machine can trip it. `CheckHealth(HealthStale)` makes stale replay fail a passing run; `CheckHealth()` leaves it as a warning because generators can change intentionally. `IgnoreHealth(checks...)` hides a check and wins over both `CheckHealth` and `WarnHealth`.

The flags `-pbt.checkhealth` and `-pbt.ignorehealth` take the same names for every `Check` of a `go test` run, for example `-pbt.checkhealth=default,slow`. Two more checks, `HealthFlaky` and `HealthShrink`, apply to failing runs; chapter 3 describes them.

`HealthStale` means that the same saved choices now describe different draw text. It does not mean that a fixed bug still fails. The warning identifies the file and the first changed draw. [Chapter 4](04-properties.md#seeds-saved-cases-and-replay) explains what to inspect before trusting the replay.

`WarnHealth(HealthDuplicate)` or `-pbt.warnhealth=duplicate` enables the duplicate warning. `CheckHealth(HealthDuplicate)` enables it as an error. `WarnHealth()` with no arguments changes nothing. Health flags and options add up; `WarnHealth` cannot downgrade a check that `CheckHealth` makes an error.

<!-- snippet: diagnostics_test.go#duplicate -->

The Boolean domain has only two values. This run intentionally repeats them:

```text
[pbt] health: HealthDuplicate: only 2 of 20 generated inputs were distinct by their choices; possible causes include a small domain such as Bool or SampledFrom, a low maximum size, or more Checks than the input space has values (CheckHealth(HealthDuplicate) makes this fail; IgnoreHealth(HealthDuplicate) hides it)
```

`HealthDuplicate` counts passing and failing cases from ordinary random generation. It excludes discards, saved-case replay, shrinking and targeted search. Coverage-guided mutations also do not count. The key hashes the consumed choices, without the size. Sizes grow during a run, so including size would hide repeated choices in most early cases.

This count is a proxy for distinct values. Different choices can map to the same value, and values derived only from size can count as duplicates. Tracking stops after more than 4096 distinct keys. A small domain can be intentional, which is why this check needs an explicit option.
