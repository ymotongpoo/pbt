# pbt

Property-based testing for Go, aiming at the feature set of Erlang's
[PropEr](https://proper-testing.github.io/): typed generators with
automatic shrinking, user-controlled distributions with statistics to verify
them, stateful and parallel command testing with symbolic variables, and
targeted properties.

**Status: under development.** The API is not stable yet. The core engine
(generators, shrinking, replay of saved counterexamples) is in place; the
features above are being added in the order laid out in
[DESIGN.md](DESIGN.md) (a Japanese version is in [DESIGN.ja.md](DESIGN.ja.md)).

Requires Go 1.25 or later.

```go
import "go.ymotongpoo.com/pbt"
```

## Example

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

## Flags

| Flag | Default | Meaning |
| --- | --- | --- |
| `-pbt.checks` | 100 | test cases per property |
| `-pbt.seed` | random | seed of the run; printed on failure |
| `-pbt.startsize` | 1 | size parameter of the first case |
| `-pbt.maxsize` | 42 | size the cases grow up to, one per passing case |
| `-pbt.shrinktime` | 30s | time budget for shrinking |
| `-pbt.nosave` | false | do not write failing cases under `testdata/pbt` |
| `-pbt.v` | false | verbose output |

The same settings are available per call as options: `pbt.Checks`,
`pbt.Seed`, `pbt.Sizes`, `pbt.ShrinkTime`, `pbt.NoSave`, `pbt.TestdataDir`.

## License

[Apache License 2.0](LICENSE.md)
