# Examples: プロパティはどう思いつくのか

This directory holds the Go examples of the Japanese book
『プロパティはどう思いつくのか　プロパティベーステスト発見ガイド』
(*How do you come up with properties? A guide to discovering properties for
property-based testing*) by Yoshi Yamaguchi, written with
`go.ymotongpoo.com/pbt`. The book is about finding properties rather than
about a library: each chapter starts from a specification, derives
properties from it, and then checks that the properties catch planted bugs.
The Japanese README is [README.ja.md](README.ja.md).

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
