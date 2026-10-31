# Where to go next

## The examples

The `examples` module of the repository ports every chapter of the book [PropEr Testing](https://propertesting.com/) by Fred Hebert to Go. The book is the best long-form introduction to the way of thinking behind pbt, and the ports show how each of its Erlang examples reads in Go.

| Book chapter | Package | Shows |
| --- | --- | --- |
| 1 to 4 | `ch01cash`, `ch04stateless` | first properties, modeling, the exercises on lists |
| 5 | `ch05generators` | custom generators, `Collect` and `Aggregate`, recursive and `Sized` generators |
| 6 | `ch06birthday` | a CSV round trip, templates, example tests next to properties |
| 7 | `ch07checkout` | layered `Bind`, negative tests, discovering a specification bug |
| 8 | `ch08shrinking` | `ShrinkTo`, `LetShrink`, `SampleShrink` |
| 9 | `ch09cache` | a sequential and a parallel stateful test of a cache |
| 10 | `ch10bookstore` | a stateful test against a database, with SQLite and PostgreSQL |
| 11 | `ch11breaker` | a state machine with the `fsm` package |

Each package has a README that answers the exercises of its chapter. Run them with `go test ./...` inside `examples`.

## Coming from rapid

[MIGRATION.md](https://github.com/ymotongpoo/pbt/blob/main/MIGRATION.md) maps rapid's API onto pbt, table by table, and lists what pbt adds. Most rapid tests translate line by line; the stateful tests change shape, because pbt generates the whole sequence against the model before running it. [BENCHMARK.md](https://github.com/ymotongpoo/pbt/blob/main/BENCHMARK.md) compares the two libraries on the same properties.

## Reference material

- The [package documentation](https://pkg.go.dev/go.ymotongpoo.com/pbt) on pkg.go.dev describes every function and option, with runnable examples.
- [DESIGN.md](https://github.com/ymotongpoo/pbt/blob/main/DESIGN.md) records the design decisions and the comparison with PropEr, QuickCheck, Hypothesis and rapid, with a Japanese version in DESIGN.ja.md.
- [CHANGELOG.md](https://github.com/ymotongpoo/pbt/blob/main/CHANGELOG.md) lists the changes of each release. The API may still change before v1.0.
- Issues and questions go to the [GitHub repository](https://github.com/ymotongpoo/pbt).

## Flags at a glance

```text
go test ./... -pbt.checks=1000          # more cases
go test ./... -pbt.seed=0x1c3           # repeat a run
go test ./... -pbt.startsize=10 -pbt.maxsize=100
go test ./... -pbt.shrinktime=2m
go test ./... -pbt.searchsteps=5000     # longer targeted searches
go test ./... -pbt.nosave               # keep testdata/pbt untouched
go test ./... -pbt.replay=testdata/pbt/TestQueue/3f2a… -run TestQueue
go test ./... -pbt.v                    # summaries and shrink steps
```
