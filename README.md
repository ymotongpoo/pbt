# pbt

Property-based testing for Go, aiming at the feature set of Erlang's
[PropEr](https://proper-testing.github.io/): typed generators with
automatic shrinking, user-controlled distributions with statistics to verify
them, stateful and parallel command testing with symbolic variables, state
machines, targeted search, and existential properties.

**Status: under development.** The API is not stable yet. The design, the
comparison with rapid, PropEr and QuickCheck, and the plan are in
[DESIGN.md](DESIGN.md) (a Japanese version is in [DESIGN.ja.md](DESIGN.ja.md)).
The `examples/` module ports every chapter of the book *PropEr Testing*
by Fred Hebert, which is the acceptance test of the design, and its
`practical` directory tests an HTTP API, a job queue, a rate limiter, a
session store and a ledger the way a project would; its `discovery`
directory holds the Go examples of the Japanese book on discovering
properties, 『プロパティはどう思いつくのか』. The
[user guide](https://go.ymotongpoo.com/pbt/guide/) explains the library
chapter by chapter, with a [Japanese version](https://go.ymotongpoo.com/pbt/guide/ja/);
its sources are under `docs/guide`. Coding agents start from
[llms.txt](https://go.ymotongpoo.com/pbt/llms.txt), which indexes the
Markdown version of every guide page, and from [AGENTS.md](AGENTS.md) when
they work on this repository.

Requires Go 1.25 or later.

```go
import "go.ymotongpoo.com/pbt"
```

## A stateless property

```go
func TestBiggest(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		xs := pbt.SliceOfN(pbt.Int(), 1, -1).Draw(t, "xs")
		sorted := slices.Clone(xs)
		slices.Sort(sorted)
		if got, want := biggest(xs), sorted[len(sorted)-1]; got != want {
			t.Fatalf("biggest(%v) = %d, want %d", xs, got, want)
		}
	})
}
```

When the property fails, `Check` shrinks the input and reports every value
the property drew:

```text
[pbt] property failed after 3 passed checks and 21 shrink steps (size 4)
[pbt] xs (SliceOfN(Int(), 1, -1)) = []int{0, 1}
[pbt] error: biggest([0 1]) = 0, want 1
[pbt] saved to testdata/pbt/TestBiggest/3f2a...; go test replays it until the file is deleted
[pbt] reproduce: go test -run 'TestBiggest' -pbt.seed=0x1c3
```

The saved file is replayed by every later `go test` run, so a fixed bug
stays fixed. Delete the file to drop the regression case, pass
`-pbt.replay=FILE` to replay one saved file in any test, or `pbt.NoReplay()`
to skip them. Shrinking keeps the kind of failure it started from, so a
panic does not shrink into an unrelated assertion that a smaller input
happens to trip.

## A stateful property

A command carries its arguments and knows how it changes the model
(`Next`) and what the real system must answer (`Post`). The model and the
system are variables of the test, and the commands are closures over them;
`init` puts the model back in its initial state whenever pbt replays a
sequence. `Commands` grows sequences from a state-dependent generator;
`RunSequential` executes them and checks every postcondition.
`ParallelCommands` and `RunParallel` do the same with concurrent branches,
accepting the results when some interleaving of the branches explains
them, as PropEr does.

```go
func TestCache(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		c := newCache()        // the system under test
		model := map[int]int{} // what the test believes it holds
		put := pbt.Custom("put", func(t *pbt.T) pbt.Command {
			k, v := pbt.IntRange(1, 10).Draw(t, ""), pbt.Int().Draw(t, "")
			return pbt.Command{Name: "put", Args: []any{k, v},
				Run:  func(pbt.Env) any { c.Put(k, v); return nil },
				Next: func(pbt.Var) { model[k] = v }}
		})
		get := pbt.Map(pbt.IntRange(1, 10), func(k int) pbt.Command {
			return pbt.Command{Name: "get", Args: []any{k},
				Run: func(pbt.Env) any { v, _ := c.Get(k); return v },
				Post: func(_ pbt.Env, got any) error {
					if want := model[k]; got != want {
						return fmt.Errorf("get(%d) = %v, want %v", k, got, want)
					}
					return nil
				}}
		})
		commands := func() pbt.CommandGen { return pbt.Frequency(pbt.W(3, put), pbt.W(1, get)) }
		prog := pbt.Commands(func() { clear(model) }, commands).Draw(t, "cmds")
		prog.RunSequential(t)
	})
}
```

On failure the report lists the program and the history of calls and
results, with the model state when `pbt.ModelState` names a function that
shows it. Results that only the system knows (ids, handles) are `pbt.Var`
values in the model and are resolved with `pbt.Resolve` at execution time.
Package `fsm` adds named states with weighted transitions.

## Distributions and statistics

Weights, sizes and shrinking targets are under your control: `Frequency`,
`Sized`, `Resize`, `ShrinkTo`, `LetShrink`, `NoShrink`, `IntRangeToward`.
`Make[T]` derives a generator from a struct type, with `pbt` tags for
ranges and lengths and `WithType` or `WithField` for overrides, and
`cmd/pbtgen` writes the same generators as source under `go generate`, where
they can fill unexported fields. `Recursive` and `Bounded` keep recursive
types, including mutually recursive ones, to about `size` nodes per value,
and `NoPanic` turns a function into a property over its parameter types.

Coming from rapid? See [MIGRATION.md](MIGRATION.md); [BENCHMARK.md](BENCHMARK.md)
compares the two on the same properties.
`Collect`, `Classify`, `Aggregate` and `Measure` print the distribution of a
passing run. `Cover` declares the share a class must reach; with the
`CheckCoverage` option, `Check` runs cases until the share is decided with
99.7% confidence and fails on a shortfall or on evidence that stays short.
A 100% requirement is checked directly: every passing case must hit the
label, and one miss makes the requirement unmet.
`Maximize` and `Minimize` turn a property into a targeted search;
`CheckExists` looks for a witness.

## Flags

| Flag | Default | Meaning |
| --- | --- | --- |
| `-pbt.checks` | 100 | test cases per property |
| `-pbt.seed` | random | seed of the run; printed on failure; reproducible with the same property code and pbt version |
| `-pbt.startsize` | 1 | size parameter of the first case |
| `-pbt.maxsize` | 42 | size the cases grow up to, one per passing case |
| `-pbt.shrinktime` | 30s | time budget for shrinking |
| `-pbt.searchsteps` | 1000 | steps of a targeted search |
| `-pbt.nosave` | false | do not write failing cases under `testdata/pbt` |
| `-pbt.replay` | none | a saved case to replay before searching |
| `-pbt.v` | false | verbose output |

The same settings are available per call as options: `pbt.Checks`,
`pbt.Seed`, `pbt.Sizes`, `pbt.ShrinkTime`, `pbt.SearchSteps`, `pbt.NoSave`,
`pbt.TestdataDir`, `pbt.ReplayFile`, `pbt.NoReplay`, `pbt.Verbose`, plus
`pbt.Setup`, `pbt.CheckCoverage`, `pbt.ReportLimit` and `pbt.ReportDepth`.
The changes between releases are listed in [CHANGELOG.md](CHANGELOG.md).

## License

[Apache License 2.0](LICENSE.md)
