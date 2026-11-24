# Examples: the PropEr Testing book in Go

Each package ports the examples and exercises of one chapter of
*PropEr Testing* by Fred Hebert (the edition at
[propertesting.com](https://propertesting.com/)) to `go.ymotongpoo.com/pbt`.
Tests that demonstrate how pbt catches a planted bug run the property against
a recording test handle (`internal/expect`) and assert on the report, so the
suite stays green.

| Chapter | Package | Contents |
| --- | --- | --- |
| 1 | `ch01cash` | The cash register and its two properties |
| 2, 3, 4 | `ch04stateless` | biggest, sort invariants, last, exercises 3 to 7, and the tooling map of chapter 2 in the README |
| 5 | `ch05generators` | Collect, Aggregate, Resize, Sized, Bind, Filter and Map, Lazy recursion, readable reports, and exercises 4 to 6 |
| 6 | `ch06birthday` | The birthday greetings kata: CSV codec, employees, the birthday filter, the mail template |
| 7 | `ch07checkout` | The checkout kata with specials and the negative tests |
| 8 | `ch08shrinking` | ShrinkTo for date-times, LetShrink for trees and concatenations, the meal exercise |
| 9 | `ch09cache` | The bounded cache as a state machine, sequential and parallel, with a planted race |
| 10 | `ch10bookstore` | The bookstore inventory on SQLite as a state machine, with the shim intents in the command names |
| 11 | `ch11breaker` | The circuit breaker as a finite state machine on package fsm |

The exercises that ask for an explanation rather than code are answered
in the README of the chapter's package.

## Practical examples

The `practical` directory holds examples that are not from the book: a
REST API, a worker queue with leases, a rate limiter, a session store and
a ledger, each tested through its public API against a model, with a
planted bug whose report the tests check. See
[practical/README.md](practical/README.md).

These examples use the v0.4.0 generators: `format.Email` and `format.URL`
for note titles, `format.Email` for session users, and `format.UUID` for
unknown session tokens. `pbt.DurationRange` generates waits around session
expiry, including differences smaller than a minute.

## The discovery guide

The `discovery` directory holds the Go examples of the Japanese book
『プロパティはどう思いつくのか　プロパティベーステスト発見ガイド』 (work in
progress), which
derives properties from specifications and checks them against planted
bugs. Its mutant tests skip themselves unless `PBT_MUTANTS` is set; see
[discovery/README.md](discovery/README.md).

Run everything with `go test -race ./...` from this directory.

## Health diagnostics

Use `go test -race -count=1 -v ./...` to see health warnings as well as
the reports of planted bugs. The book ports retain generators whose
structure is part of the chapter:

- `ch06birthday` shares the size across CSV columns, rows and field
  lengths. Its nested generators can produce `HealthLarge` and `HealthSlow`
  warnings in the round-trip and employee tests.
- `ch08shrinking` filters dates while demonstrating `ShrinkTo` for their
  separate components. Its July counterexample can include a `HealthFilter`
  note from rejected dates during shrinking.
- `discovery` chapter 6 keeps the idempotence tests of ID lists and paths
  as the book writes them, discarding inputs that do not decode, which can
  trigger `HealthFilter`.
- `discovery/ch08` states the odd-length premise of median negation with
  `Assume`, which rejects about half the cases and can trigger `HealthFilter`.
- `discovery/ch17` deliberately uses `Assume` on arbitrary lists to show
  how discards hide a bug. Its `TestAssumeOK` and `TestAssumeBadSurvives`
  tests retain the `HealthFilter` warnings.

Planted races can report `HealthFlaky` when replay schedules operations
differently. `HealthSlow` measures elapsed generation time, so the race
detector and competing test processes can also trigger it in other examples.

These diagnostics remain visible; the examples do not silence them with
`IgnoreHealth`. The `-pbt.checkhealth` flag makes selected warnings fail a
passing property, so choose the package and test before enabling it.
