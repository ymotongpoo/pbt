# Practical examples

Each package here tests a small but realistic system the way a project
would: through its public API, against a model of what the system should
do, with a planted bug that shows what the failure report looks like. The
examples build on the stateful chapters of the
[user guide](https://go.ymotongpoo.com/pbt/guide/05-stateful) and are the
place to look when the book's examples feel far from your own code.

| Package | System | What it shows |
| --- | --- | --- |
| `httpapi` | A REST API for notes with optimistic concurrency, served by `net/http/httptest` | Server-assigned ids kept as `Var`s, status codes and bodies as postconditions, a stale `If-Match`, and the same commands run in parallel against a server that allocates ids outside its lock |
| `jobqueue` | A worker queue with leases, a visibility timeout, retries and dead letters | A fake clock advanced by a command, every lease ever issued kept in the model so that stale leases are presented too, and an ack that does not check which lease it was given |
| `ratelimit` | A sliding-window limiter per client that forgets idle clients | The rule "at most N requests in any window" as the model, in place of the pruned logs the limiter keeps, and a sweep that forgets clients too early |
| `sessions` | A session store with expiry, refresh and logout | The `fsm` package with state data and a clock, time as a transition split by where it leads, and a refresh that revives an expired session |
| `ledger` | Transfers between accounts | A model that predicts every result next to an audit command that checks only the invariants, a transfer that creates money, and a parallel test that catches an overdraft race |

Every package has a test that runs against the correct system and one or
two that run against the planted bug through `internal/expect`, which
records the failure report instead of failing the suite. Run

```sh
go test -v -run Caught ./practical/...
```

to read the reports, and add `-race` to let the race detector watch the
parallel runs.

## Patterns the examples share

- **Fake time.** `internal/clock` is a clock that moves only when a command
  says so. The system takes `clock.Now` as its time source, and the model
  keeps the elapsed time as a number, so a lease expires or a session times
  out in the model and in the system at the same step.
- **Results as `Var`s.** The ids a server chooses, the tokens a store issues
  and the leases a queue hands out are results of earlier commands. The
  model holds them as `pbt.Var` and the commands resolve them when they
  run, so a shrunk program still refers to the right ones.
- **Expectations derived from the model.** The update of `httpapi` sends
  the version it chose when it was generated, and its postcondition decides
  from the model's current version whether the server must answer 200 or
  412. The command stays valid after the shrinker removes the commands
  around it, which a captured expectation would not.
- **Stale handles stay in the model.** `jobqueue` keeps every lease and
  `httpapi` every deleted note, so that commands keep presenting them and
  the test checks the rejections as carefully as the successes.
- **Planted bugs behind options.** Each system has an `Options` struct
  whose zero value is the correct system; the bugs are documented there,
  and the tests that catch them are named `...IsCaught` or `...AreCaught`.
