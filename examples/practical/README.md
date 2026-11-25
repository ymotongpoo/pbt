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
| `jobqueue` | A worker queue with leases, a visibility timeout, retries and dead letters | A fake clock advanced by a command, every lease ever issued kept in the model so that stale leases are presented too, an ack that does not check which lease it was given, and `Causality` on a handoff between two workers |
| `ratelimit` | A sliding-window limiter per client that forgets idle clients | The rule "at most N requests in any window" as the model, in place of the pruned logs the limiter keeps, a sweep that forgets clients too early, and a budgeted run whose `ReportJSON` and `Observations` files a test reads the way a CI job or an agent would |
| `sessions` | A session store with expiry, refresh and logout | The `fsm` package with state data and a clock, time as a transition split by where it leads, and a refresh that revives an expired session |
| `ledger` | Transfers between accounts | A model that predicts every result next to an audit command that checks only the invariants, a transfer that creates money, a parallel test that catches an overdraft race, and `Signature` with `MaxFailures` to report money creation and overdrafts apart |

Every package has tests that run against the correct system and tests
that run against planted bugs through `internal/expect`, which
records the failure report instead of failing the suite. Run

```sh
go test -v -run Caught ./practical/...
```

to read the reports, and add `-race` to let the race detector watch the
parallel runs.

## Diagnosing failures and reading run results

`jobqueue.TestStaleAckCausalityIsCaught` models two workers polling one job.
It generates a wait, lets the second worker poll, then acknowledges with
the first lease. `Causality(8)` samples continuations before shrinking.
With `Seed(1)`, the largest measured rise occurs at the wait that expires
the first lease; the ack detects the error two commands later.
This identifies a candidate cause, not proof of causation.
The fixed five-command scenario keeps the handoff and bounds diagnostic work.
`TestLeaseHandoff` runs the same property against the correct queue.

`ledger.TestAuditCausesAreCaught` enables both `AllowSelfTransfer` and
`AllowOverdraft`. The audit names its violated invariant with `Signature`.
`MaxFailures(2)` then reports "money not conserved" and "overdrawn account"
separately, although both failures share the audit postcondition.
`expect.Failures` returns every report; `expect.Failure` keeps returning
only the first. The names omit generated values so shrinking can reduce
balances and transfer amounts without changing the failure's identity.
`TestLedgerNamedInvariants` checks the same audit against the correct ledger.

`ratelimit.TestLimiterRunArtifacts` runs the existing limiter property with
10,000 checks and a 100 ms search budget. It decodes the `ReportJSON` file
with `encoding/json` and checks the outcome, completed checks and command
statistics. It also reads `Observations` as OpenPBTStats records: one
passing `test_case` per completed check, followed by an `info` record.
Both outputs live under `t.TempDir()`; a CI job can use artifact paths
instead. The completed count depends on machine speed, and the consumer
ignores unknown JSON fields. A budget ends search between cases; replay
and shrinking have separate limits, and a blocked call needs a test timeout.

Run these tests from `examples`:

```sh
go test -race -count=1 -v ./practical/jobqueue -run 'Test(LeaseHandoff|StaleAckCausalityIsCaught)$'
go test -race -count=1 -v ./practical/ledger -run 'Test(LedgerNamedInvariants|AuditCausesAreCaught)$'
go test -race -count=1 -v ./practical/ratelimit -run '^TestLimiterRunArtifacts$'
```

`HealthDuplicate` stays off: the small client, account and timeout domains
are intentional. Enable `pbt.WarnHealth(pbt.HealthDuplicate)` when inspecting
repeated choices in a larger search.
`HealthStale` already warns about changed draw text during saved-case replay;
inspect that warning after changing a generator.

These examples use state models rather than a decoder with a chain of exact
byte comparisons. They do not add a separate protocol just for
`CoverageGuided`; the [coverage example in the guide](../../docs/guide/08-targeted.md#searching-with-code-coverage)
demonstrates that search with an instrumented header recognizer.

## Patterns the examples share

- **One struct for the system and the model.** Each test keeps both in a
  `harness` struct and writes the commands as closures over it, with a
  `reset` method as the `init` of `pbt.Commands`. The same program can then
  run against a fresh system several times, as the parallel tests do by
  replacing the system between runs.
- **Fake time.** `internal/clock` is a clock that moves only when a command
  says so. The system takes `clock.Now` as its time source, and the model
  keeps the elapsed time as a number, so a lease expires or a session times
  out in the model and in the system at the same step.
- **Generated text formats.** `httpapi` mixes non-empty text with
  `format.Email()` and `format.URL()` as note titles, checking their JSON
  round trip through create, update, get and list. `sessions` uses
  `format.Email()` for user names and `format.UUID()` for unknown tokens.
  The store issues tokens such as `s1`, so generated UUIDs cannot identify
  an existing session.
  The HTTP tests bound sequential programs and parallel prefixes to 20
  commands so that shrinking stays within the time budget.
  The planted ID race needs no setup, so that test uses an empty prefix.
- **Bounded interleavings.** The HTTP and ledger tests use two parallel
  branches with at most three commands each. Generation checks every
  interleaving, so shorter branches reduce the cost of replaying the model.
- **Expiry boundaries.** `sessions` uses `pbt.DurationRange(lo, hi)` for
  waits, with inclusive bounds that separate active and expired states.
  Waits can stop one nanosecond before expiry or reach the deadline exactly.
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
