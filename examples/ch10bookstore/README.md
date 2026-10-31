# Chapter 10: the bookstore

`bookstore.go` is the inventory on SQLite; `bookstore_test.go` is the
state machine test with the shim intents of the chapter's precise version.
Questions 1, 2, 4 and 5 ask for explanations; question 3 is answered by
`openStore`, which opens a fresh database per test case and closes it in
`Cleanup`.

**Question 1.** During the abstract phase, when the command sequence is
generated and when it is shrunk, pbt calls the generator, `Precondition`
and `NextState`. During the real execution it calls `Precondition` again,
`Execute`, `Postcondition` and `NextState`. `NextState` receives the
result only as a `pbt.Var` in both phases, so the model never depends on
real results.

**Question 2.** Pattern matching in the generator is not enough because
the generator runs once, with the state at generation time. When shrinking
removes an earlier command, the commands after it are regenerated from a
different state, and the parallel test executes branches in orders the
generator never saw. The precondition is checked in every one of those
situations, so it must state what the command needs on its own.

**Question 3.** The three placements of start and stop: inside the
property, around `RunSequential`, start and stop the system once per
test case, which is what `openStore` does. Starting it once per test
function shares state between cases, so a failure in one case corrupts
the next. Starting it inside a command makes the model responsible for
the system's lifetime, which a shrunk sequence can break.

**Question 4.** Using the real result in `NextState` makes the model
depend on values that exist only during execution, so the state at
generation time and at execution time diverge, and shrinking, which
regenerates the sequence, can no longer reproduce it. pbt therefore
passes a `pbt.Var` and resolves it with `pbt.Resolve` in the
postcondition. The case where the real result is needed is a system that
assigns identifiers, such as the registry in the library's own tests: the
model stores the `Var` and the next command refers to it.

**Question 5.** Raw calls on the real system keep the test close to the
API and find bugs the author did not think of, but the preconditions and
postconditions have to work out the intent of every call. Shim calls with
model-aware generators, such as `borrow_copy:unavail`, name the intent, so
the statistics show which paths ran and the postconditions stay simple,
at the cost of a layer that must itself be right.

What differs from the chapter: SQLite replaces PostgreSQL by default, so
the driver-specific bugs (Unicode code-point lists, the zero byte) do not
reproduce, while the `LIKE` wildcards and the `return_copy` and
`borrow_copy` fixes do. `LIKE` runs with `PRAGMA case_sensitive_like` so
that the database and the model agree on substring matching. To run on
PostgreSQL as the chapter does, point `PBT_BOOKSTORE_DSN` at a database,
for example after `docker run -e POSTGRES_PASSWORD=secret -p 5432:5432
postgres`:

```sh
PBT_BOOKSTORE_DSN='postgres://postgres:secret@localhost:5432/postgres' go test ./ch10bookstore/
```

Each test case drops and recreates the `books` table.

The PostgreSQL run found one more difference: a title containing a
backslash was not found by `find_by_title`, because PostgreSQL treats the
backslash as the escape character of `LIKE` by default, while SQLite does
not. The query now escapes the backslash and the two wildcards with an
explicit `ESCAPE` clause, so the term is always literal; the generators
still leave the wildcards out, as in the chapter, so that the story of the
chapter holds. The same run also rejected titles longer than the 256
characters of the column, which SQLite never enforces; the generators now
stay within the schema.
