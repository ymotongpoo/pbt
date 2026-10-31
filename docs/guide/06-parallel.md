# Parallel testing

A system that is correct under sequential use can still break under concurrent use. pbt tests for that with the same commands and model as a sequential test: it generates a sequential prefix followed by branches, runs the branches in goroutines, and then checks whether some interleaving of the branches explains the results that came back.

<!-- snippet: parallel_test.go#parallel -->

`ParallelCommands` generates programs with two branches of at most six commands each after the prefix. `ParallelCommandsN(init, gen, branches, maxBranchLen)` sets both numbers, with at most three branches and at most twelve commands across them; the check enumerates interleavings, and the count grows fast. `RunParallel` runs the prefix, then the branches concurrently, and finally searches for an order of the recorded calls and results that satisfies every postcondition in turn. When no order does, the case fails:

```text
[pbt] error: no interleaving of the 2 branches satisfies the postconditions
```

preceded by the program and the history of each branch. A command that panics in a branch fails the case as well.

## What the check means

The check keeps the order of the calls within each branch and tries every order between branches. It accepts the run when one of those orders is consistent with the model. That is what PropEr's `parallel_commands` does, and it is weaker than a linearizability check, which would also respect the real-time order in which calls from different branches completed. A system can pass this check and still violate linearizability in ways only a timing-aware checker sees. It does catch the common failures: lost updates, duplicate identifiers, reads of partial state.

Two consequences follow for how you write the commands.

- Every command must return a value that a postcondition can check. A command that returns nothing gives the check nothing to explain.
- A branch cannot depend on the results of another branch, since neither has run when the program is generated. Each branch resolves its own `Var` values and those of the prefix.

## Combine it with the race detector

The interleaving check finds behavior that the model rejects. The race detector finds unsynchronized memory access whether or not it changed a result. Run parallel tests with `go test -race`; the two tools catch different bugs, and a parallel property gives the race detector the concurrent workload it needs.

The `Counter` above holds a mutex. Remove it, and the test reports either duplicated results that no interleaving explains or a data race, depending on the run.
