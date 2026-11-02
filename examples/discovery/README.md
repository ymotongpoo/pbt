# Examples: 『プロパティはどう思いつくのか』 (work in progress)

This directory holds a Go port of the examples of the Japanese book
『プロパティはどう思いつくのか　プロパティベーステスト発見ガイド』 (work in
progress; the title is not final) by Yoshi Yamaguchi. The book's own
listings are written in TypeScript with [fast-check](https://fast-check.dev/)
and Vitest; this directory is their port to `go.ymotongpoo.com/pbt`. The
book is about finding properties rather than about a library: each chapter
starts from a specification, derives properties from it, and then checks
that the properties catch planted bugs. The Japanese README is
[README.ja.md](README.ja.md).

## Layout

| Chapter | Topic | Where |
| --- | --- | --- |
| 1 | Why properties do not come to mind | `ch01_test.go` |
| 2 | The five steps of discovery | `ch02_test.go`, `ch02` |
| 3 | Generalizing example tests | `ch03_test.go`, `ch03` |
| 4 | Decomposing a specification | `ch04_test.go`, `ch04` |
| 5 | Algebraic laws | `ch05_test.go`, `ch05` |
| 6 | Round trips | `ch06_test.go`, `ch06` |
| 7 | Comparing with a simple model | `ch07_test.go`, `ch07` |
| 8 | Metamorphic relations | `ch08_test.go`, `ch08` |
| 9 | Invariants and robustness | `ch09_test.go`, `ch09` |
| 10 | What a model keeps and what it drops | `ch10_test.go`, `ch10` |
| 11 | Five questions for each operation | `ch11_test.go`, `ch11` |
| 12 | Invariants of the whole sequence | `ch12_test.go`, `ch12` |
| 13 | History and lifecycle | `ch13` |
| 14 | Equivalent sequences and order independence | `ch14_test.go`, `ch14` |
| 15 | Rewriting as a state machine | `ch15_test.go`, `ch15` |
| 16 | Moving on to concurrent tests | `ch16_test.go`, `ch16` |
| 17 | Telling weak properties apart | `ch17_test.go`, `ch17` |
| 18 | Checking reachability with statistics | `ch18_test.go`, `ch18` |
| 19 | Properties from past bugs | `ch19` |

The files `chNN_test.go` belong to the top-level package `discovery`;
`exp_test.go` holds the measurements behind some of the numbers in the
book. The directories `chNN` are one package per chapter.

## Running

Run the tests from the `examples` directory:

```sh
go test -race ./discovery/...
```

A test whose name ends in `Mutant` runs a property against a planted bug,
and it is supposed to fail. These tests skip themselves unless the
environment variable `PBT_MUTANTS` is set, so that the suite stays green.
`run_mutants.sh` runs the ordinary tests, then every mutant test on its
own, and reports a mutant that passes:

```sh
./discovery/run_mutants.sh
```

To read the report of one planted bug, run it directly:

```sh
PBT_MUTANTS=1 go test -run '^TestRuneExactEmptyMutant$' ./discovery/ch02
```

A few tests depend on scheduling or measure detection rates over many
runs. They skip themselves unless `PBT_FLAKY=1` or `PBT_MEASURE=1` is set.
The race-only mutant of chapter 16 is built with `-race`.

Comments and messages in the code are in Japanese, as in the book.

## Where the port differs from the book

The properties, the planted bugs and the detected/passed verdicts of the
tables are the same in most chapters. The numbers that depend on the
generators (shares, case counts, shrunk counterexamples) differ, since
fast-check and pbt generate differently. The commands follow pbt's model,
in which the whole sequence is generated against the model first and
results are named by `Var`s; the book describes fast-check's commands,
which run against the model as they go. Five places of the book were
rewritten for fast-check and no longer correspond to this code:

1. **Chapter 2, step 5** (`TestTruncateCoverage` in `ch02_test.go`). Here
   the property passes `CheckCoverage` with `pbt.String()` and
   `IntRange(0, 20)`. In the book, `fc.string()` makes strings of at most
   10 characters, so the same requirement stays undecided and fails; the
   book quotes that report and fixes the generator (a character set with
   surrogate pairs, `n` in 0..10).
2. **Chapter 9, the percentage** (`Percent` in `ch09/`). Here `done*100`
   overflows `int`. JavaScript numbers are doubles and do not overflow, so
   the book truncates to 32 bits with `Math.imul(done, 100)` to produce the
   overflow.
3. **Chapter 15, tuning the weights** (`ch15/`). Here the self-transitions
   of `new` are below 1%, so the weights leaving `new` become verify 1,
   lock 2, unlock 2. In the book the walks are at most 10 steps long and
   those transitions are around 8%; the lowest are the self-transitions of
   `locked`, so the book changes the weights leaving `locked` to unlock 1,
   verify 2, lock 2.
4. **Chapter 16, concurrency** (`ch16/`). Here the chapter is built on
   `pbt.ParallelCommands` (a sequential prefix and parallel branches,
   checked for linearizability) and `go test -race`. JavaScript has no
   threads, so the book interleaves asynchronous operations across `await`
   with `fc.scheduler` and checks linearizability with a helper of its own.
   As a result:
   - the sections on data races and the race detector (`StatCounter` in
     `stat_race_test.go` and `stat_norace_test.go`, the version without
     locks, the table against `-race`) are not in the book;
   - here the counter takes its lock twice, once to read and once to write,
     and another goroutine runs in between; in the book the counter loses
     updates across an `await` between its read and its write;
   - `StickyRegister` survives the linearizability check here, and the
     chapter concludes that another means is needed; the book records the
     order of calls and completions and catches it with a check that
     respects real time;
   - the section on failures that are hard to reproduce covers replaying
     saved cases and `-race` here; the book covers the interleaving as part
     of the generated value, replay with `seed` and `path`, and
     `schedulerFor`.
5. **Chapter 19, past bugs** (`ch19/`). The book dropped the paragraph
   that used pbt's own fix in v0.3.2 (`Rune` now includes U+10FFFF) and
   the paragraph that pointed to `examples/practical`. Neither paragraph has
   code in this directory. The book also keeps a found counterexample with
   fast-check's `examples` option instead of the failure files that pbt
   saves under `testdata/pbt`.
