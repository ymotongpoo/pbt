# Getting started

pbt is a property-based testing library for Go. A property is a test that must hold for every input of some shape. pbt generates the inputs, shrinks a failing one to a small case, and saves that case so that the next `go test` run replays it. The feature set follows Erlang's [PropEr](https://proper-testing.github.io/): generators whose distribution you control, statistics to check that control, stateful and parallel command testing, state machines, targeted search, and existential properties.

This guide describes pbt v0.2.0. It assumes that you write tests with `go test`. The [package documentation](https://pkg.go.dev/go.ymotongpoo.com/pbt) is the reference for every function; the chapters here explain how the pieces fit together and which one to reach for.

## Install

```sh
go get go.ymotongpoo.com/pbt@v0.2.0
```

pbt needs Go 1.25 or later and has no dependencies outside the standard library. Import it as `go.ymotongpoo.com/pbt`.

## A first property

A property is a function that receives a `*pbt.T`, draws values from generators, and reports a failure through the methods it shares with `*testing.T`. `pbt.Check` runs it.

<!-- snippet: intro_test.go#largest -->

Three things happen in this test.

- `pbt.SliceOfN(pbt.Int(), 1, -1)` is a generator of slices with at least one element and no upper bound. A generator is a value of type `pbt.Gen[V]`; you can store it in a variable and reuse it.
- `Draw(t, "xs")` produces one value for the current test case. The label names the value in the report of a failure.
- `pbt.Check(t, prop)` runs the property 100 times. The size parameter grows from 1 to 42 over the run, so the first cases are small and later ones larger. Chapter 3 explains the size.

`*pbt.T` has `Errorf`, `Fatalf`, `Logf`, `Cleanup` and the other reporting methods of `*testing.T`, so a property reads like an ordinary test. A panic inside the property counts as a failure too.

## When a property fails

The next implementation has a bug: it starts the maximum at 0, which is wrong when every element is negative.

<!-- snippet: intro_test.go#biggest -->

A run of `pbt.Check(t, biggestProperty)` fails with this report.

```text
[pbt] property failed after 12 passed checks and 16 shrink steps (size 13)
[pbt] xs (SliceOfN(Int(), 1, -1)) = []int{-1}
[pbt] error: biggest([-1]) = 0, want -1
[pbt] saved to testdata/pbt/TestBiggest/5a0c…; go test replays it until the file is deleted
[pbt] reproduce: go test -run 'TestBiggest' -pbt.seed=0x1
```

Read it line by line.

- The first line says how many cases passed before the failure, and how many shrink steps it took to reach the case shown. The failing case was found at size 13.
- Every value the property drew is listed with its label, its generator and its value. The slice that failed first had 13 elements; the shrinker removed all but one and reduced that one to the smallest negative number. `[-1]` is the smallest input that still fails.
- The `error:` line is the message passed to `Fatalf`. Log lines, panics and the output of `OnFail` callbacks appear here as well.
- The case was saved under `testdata/pbt`, in a directory named after the test. The next `go test` replays it before generating anything new, so the bug stays visible until it is fixed, and the file is yours to delete or to keep as a regression case.
- The last line is the command that reproduces the run, seed included.

## What the rest of the guide covers

The chapters build on each other. Chapters 1 to 4 cover generators, the size and distribution of the data, shrinking, and the options of a run. Chapters 5 to 7 cover stateful testing against a model, parallel execution and state machines. Chapter 8 covers targeted search and existential properties, chapter 9 the generators derived from types, and chapter 10 points to the examples, the migration guide for rapid users and the reference material.
