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
by Fred Hebert, which is the acceptance test of the design.

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
stays fixed. Delete the file to drop the regression case.

## A stateful property

A command carries its arguments and knows how it changes the model
(`Next`) and what the real system must answer (`Post`). `Commands` grows
sequences from a state-dependent generator; `RunSequential` executes them and
checks every postcondition. `ParallelCommands` and `RunParallel` do the same
with concurrent branches, accepting the results when some interleaving of
the branches explains them, as PropEr does.

```go
type model struct{ entries map[int]int }

func commands(m model) pbt.Gen[pbt.Command[model, *cache]] {
	return pbt.Frequency(
		pbt.Weighted[pbt.Command[model, *cache]]{Weight: 3, Gen: pbt.Custom("put", func(t *pbt.T) pbt.Command[model, *cache] {
			k, v := pbt.IntRange(1, 10).Draw(t, ""), pbt.Int().Draw(t, "")
			return pbt.Cmd[model, *cache]{Name: "put", Args: []any{k, v},
				Run:  func(c *cache, _ pbt.Env) any { c.Put(k, v); return nil },
				Next: func(m model, _ pbt.Var) model { return m.with(k, v) }}
		})},
		pbt.Weighted[pbt.Command[model, *cache]]{Weight: 1, Gen: pbt.Map(pbt.IntRange(1, 10), func(k int) pbt.Command[model, *cache] {
			return pbt.Cmd[model, *cache]{Name: "get", Args: []any{k},
				Run: func(c *cache, _ pbt.Env) any { v, _ := c.Get(k); return v },
				Post: func(m model, _ pbt.Env, got any) error {
					if want := m.entries[k]; got != want {
						return fmt.Errorf("get(%d) = %v, want %v", k, got, want)
					}
					return nil
				}}
		})},
	)
}

func TestCache(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		prog := pbt.Commands(model{}, commands).Draw(t, "cmds")
		prog.RunSequential(t, newCache())
	})
}
```

On failure the report lists the program and the history of calls, results
and the model state. Results that only the system knows (ids, handles) are
`pbt.Var` values in the model and are resolved with `pbt.Resolve` at
execution time. Package `fsm` adds named states with weighted transitions.

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
passing run, and `Cover` with the `CheckCoverage` option fails the test when
a declared share of cases is not reached. `Maximize` and `Minimize` turn a
property into a targeted search; `CheckExists` looks for a witness.

## Flags

| Flag | Default | Meaning |
| --- | --- | --- |
| `-pbt.checks` | 100 | test cases per property |
| `-pbt.seed` | random | seed of the run; printed on failure |
| `-pbt.startsize` | 1 | size parameter of the first case |
| `-pbt.maxsize` | 42 | size the cases grow up to, one per passing case |
| `-pbt.shrinktime` | 30s | time budget for shrinking |
| `-pbt.searchsteps` | 1000 | steps of a targeted search |
| `-pbt.nosave` | false | do not write failing cases under `testdata/pbt` |
| `-pbt.v` | false | verbose output |

The same settings are available per call as options: `pbt.Checks`,
`pbt.Seed`, `pbt.Sizes`, `pbt.ShrinkTime`, `pbt.SearchSteps`, `pbt.NoSave`,
`pbt.TestdataDir`, plus `pbt.Setup` and `pbt.CheckCoverage`.

## License

[Apache License 2.0](LICENSE.md)
