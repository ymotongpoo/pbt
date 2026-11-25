# Getting started
<!-- summary: What a property is, how to install pbt, a first property with ForAll and Check, and what happens when it fails: shrinking, the saved case and the replay. -->

pbt is a property-based testing library for Go. A property is a test that must hold for every input of some shape. pbt generates the inputs, shrinks a failing one to a small case, and saves that case so that the next `go test` run replays it. The feature set follows Erlang's [PropEr](https://proper-testing.github.io/): generators whose distribution you control, statistics to check that control, stateful and parallel command testing, state machines, targeted search, and existential properties.

This guide describes pbt v0.4.0 and the additions on the main branch that the next release will carry: causality analysis, named failure identities, two more health checks, time budgets, machine-readable output and a coverage-guided search. `go get go.ymotongpoo.com/pbt@main` installs them until then. It assumes that you write tests with `go test`. The [package documentation](https://pkg.go.dev/go.ymotongpoo.com/pbt) is the reference for every function; the chapters here explain how the pieces fit together and which one to reach for.

## Install

```sh
go get go.ymotongpoo.com/pbt@v0.4.0
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

The guide's test runs `biggestProperty` with `Seed(1)` and `NoSave()` to capture this report:

```text
[pbt] property failed after 12 passed checks and 20 shrink steps (size 13)
[pbt] xs (SliceOfN(Int(), 1, -1)) = []int{-1}
[pbt] error: biggest([-1]) = 0, want -1
[pbt] reproduce: go test -run 'TestBiggest' -pbt.seed=0x1
```

Read it line by line.

- The first line says how many cases passed before the failure, and how many shrink steps it took to reach the case shown. The failing case was found at size 13.
- Every value the property drew is listed with its label, its generator and its value. The slice that failed first had 13 elements; the shrinker removed all but one and reduced that one to the smallest negative number. `[-1]` is the smallest input that still fails.
- The `error:` line is the message passed to `Fatalf`. Log lines, panics and the output of `OnFail` callbacks appear here as well.
- The last line is the command that reproduces the run, seed included.

Without `NoSave`, pbt also saves the case under `testdata/pbt`, in a directory named after the test. The next run replays it before generating new cases. Keep that file as a regression case after fixing the bug.

## What the rest of the guide covers

The chapters build on each other. Chapters 1 to 4 cover generators, the size and distribution of the data, shrinking, and the options of a run. Chapters 5 to 7 cover stateful testing against a model, parallel execution and state machines. Chapter 8 covers targeted search and existential properties, and chapter 9 the generators derived from types. Chapter 10 covers the strings in the formats that standards define. Chapter 11 points to the examples, the migration guide for rapid users and the reference material. [Chapter 12](12-agents.md) gives coding agents a method for testing a project, from choosing properties to reporting a reproducible bug.
