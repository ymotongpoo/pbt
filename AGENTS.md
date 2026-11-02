# AGENTS.md

pbt is a property-based testing library for Go, module `go.ymotongpoo.com/pbt`,
with the feature set of Erlang's PropEr. This file is for coding agents that
work on the repository. People who use the library read the
[user guide](https://go.ymotongpoo.com/pbt/guide/); agents that use the
library start from <https://go.ymotongpoo.com/pbt/llms.txt>, which indexes
the Markdown version of every guide page.

## Layout

- The root module `go.ymotongpoo.com/pbt` is the library: package `pbt`,
  `fsm/` (state machines), `cmd/pbtgen` (writes generators for types as
  source) and `docs/guide/snippets` (the listings of the guide, compiled and
  run as tests).
- `examples/` is a separate module. `chNN*` ports the chapters of the book
  *PropEr Testing*; `practical/` tests realistic systems (an HTTP API, a job
  queue, a rate limiter, a session store, a ledger); `discovery` holds the
  Go port of the examples of the Japanese book 『プロパティはどう思いつくのか』
  (work in progress), whose
  tests named `...Mutant` must fail and skip unless `PBT_MUTANTS` is set,
  checked by `discovery/run_mutants.sh`; `internal/expect` runs
  a property that must fail and returns its report, which is how the tests
  show a planted bug without failing the suite; `internal/clock` is a fake
  clock.
- `bench/` is a separate module that compares pbt with rapid.
- `site/` is a separate module that renders `docs/guide` into HTML, Markdown
  pages, `llms.txt` and `llms-full.txt`. Its output, `site/public`, is
  ignored by git and copied into the repository of the domain,
  <https://github.com/ymotongpoo/go.ymotongpoo.com>, which serves the
  vanity import paths and publishes the documentation of every module
  under go.ymotongpoo.com.
- `DESIGN.md` records the design decisions, the API listing, the comparison
  with PropEr, QuickCheck, Hypothesis and rapid, and the deferred items;
  `DESIGN.ja.md` is its Japanese translation. `MIGRATION.md` maps rapid's
  API onto pbt. `CHANGELOG.md` follows Keep a Changelog. `BENCHMARK.md`
  reports the comparison with rapid.

## Build and test

There is no CI. Run the checks yourself before every commit.

```sh
gofmt -l . && go vet ./... && go test ./...        # root module, includes the guide's listings
cd examples && go vet ./... && go test -race ./... # the examples module
cd bench && go vet ./...                           # the benchmark module
```

The parallel tests are meant to run under `-race`. Tests named
`...IsCaught` or `...AreCaught` run a planted bug through `internal/expect`
and assert on the report; they use `pbt.Seed(1)`, so a change to the
generators or the shrinker may change which counterexample they find.

## Conventions

- Every Go file starts with these two lines; `license_test.go` checks them,
  and generated files that contain `DO NOT EDIT.` are exempt:

  ```go
  // SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
  // SPDX-License-Identifier: Apache-2.0
  ```

- The library requires Go 1.25 or later; the examples module declares
  `go 1.26`. Code is `gofmt`-formatted and `go vet`-clean.
- The package is `pbt`. It does not use `testing.F`. Failing cases are
  saved under `testdata/pbt` and replayed by later runs.
- Generators are generic, `Gen[V]`, and the API leans on type inference:
  users never have to spell a type argument in ordinary code. The stateful
  API has no type parameters at all since v0.3.0: `Command` is a struct of
  functions that close over a model and a system held in variables of the
  test, `Commands(init, gen, opts...)` takes a function that resets the
  model, and pbt rewinds the model by calling `init` and replaying `Next`.
  Keep it that way: do not add type parameters that users would have to
  write, and prefer a function that infers (such as `W`) over a generic
  struct literal. A function that returns a command generator is
  declared as `CommandGen`, the alias of `Gen[Command]`, since Go does not
  infer the type argument in a declaration.
- In stateful tests, only `init` and the `Next` functions change the model;
  `Pre`, `Post` and the command generator read it. The examples keep the
  system and the model in a `harness` struct with the commands as closures
  over it, so that one program can run against several systems.
- English is the primary language of the documentation. A change to
  `DESIGN.md` is mirrored in `DESIGN.ja.md`, and a change to a chapter of
  `docs/guide` is mirrored in `docs/guide/ja`. Japanese prose is in polite
  form (です・ます), one sentence per line.
- The guide's code comes from `docs/guide/snippets`: a chapter includes a
  region with `<!-- snippet: FILE#NAME -->`, and the region is the lines
  between `// snippet: NAME` and `// snippet: end`. Reports quoted in the
  guide are the real output of those tests. The line after a chapter's
  title is `<!-- summary: ... -->`, which `llms.txt` uses as the
  description of the page.
- After an API change, update in the same change: the guide (both
  languages), `README.md`, `DESIGN.md` and `DESIGN.ja.md`, `MIGRATION.md`
  when the rapid mapping moves, `CHANGELOG.md`, the examples, and then
  re-render and publish the guide.

## Commits

Commit messages are in English and follow Conventional Commits:
`type(scope): summary`, with types `feat`, `fix`, `refactor`, `docs`,
`test`, `chore` and scopes such as `guide`, `site`, `examples`, `pages`,
`readme`, `release`. A breaking change is marked `feat!`. Keep commits
small and logical (library, tests, examples, docs apart), stage files
explicitly with `git add path`, and make sure every commit builds and
passes the tests on its own.

## Releasing

1. Add the version and date to `CHANGELOG.md` and commit it as
   `chore(release): vX.Y.Z`.
2. `git tag -a vX.Y.Z -m "..."`, push `main` and the tag.
3. `gh release create vX.Y.Z` with the changelog section as notes.
4. Check `go list -m -versions go.ymotongpoo.com/pbt` from outside the
   module.
5. Re-render the guide with the new version and publish it (below).

## Publishing the guide

The site lives in the sibling repository `../go.ymotongpoo.com` (clone
<https://github.com/ymotongpoo/go.ymotongpoo.com> if it is missing). Render
the guide, copy it in, regenerate the site's rules, commit there and deploy:

```sh
(cd site && go run . -version vX.Y.Z -analytics G-EC4QPYV95Y)
rm -rf ../go.ymotongpoo.com/public/guide/pbt && cp -R site/public ../go.ymotongpoo.com/public/guide/pbt
cd ../go.ymotongpoo.com && go run . && git add public && git commit -m "docs(pbt): publish the guide for vX.Y.Z" && ./deploy.sh && git push
```

The README of that repository explains its `_redirects` and `_headers`
rules and the pitfalls of the rule validator.
