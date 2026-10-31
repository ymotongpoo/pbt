# Stateful testing

Many bugs only show up after a sequence of operations: a cache that returns a stale value after an eviction, a queue that miscounts after a close. A stateful test describes the operations as commands, keeps a model of what the system should contain, generates sequences of commands against the model, runs them against the real system and checks every result. When a sequence fails, pbt shrinks it to the shortest one that still fails.

## The system under test

The example is a queue that issues ticket numbers and closes them.

<!-- snippet: stateful_test.go#queue -->

## The model and the commands

The model is what the test believes about the system. Here it is the list of open tickets. The ticket numbers do not exist while the sequence is generated, because nothing has run yet, so the model holds them as `pbt.Var` values, symbolic names for results. PropEr does the same with `{var, N}`.

<!-- snippet: stateful_test.go#model -->

A command is a value of type `pbt.Command[S, Sys]`, with `S` the model and `Sys` the system. `pbt.Cmd` builds one from functions.

- `Name` and `Args` are what the report prints. Put the arguments of the call in `Args`, with the `Var` of a result where the call uses one, so that the report reads like a program.
- `Pre` says whether the command is allowed in a model state. It is checked when the sequence is generated and again when a shrunk sequence is replayed, so a command whose precondition fails after another command was removed is not a candidate.
- `Run` performs the call against the system. The `Env` maps every `Var` to the value its command returned, and `pbt.Resolve[T]` reads one with a type.
- `Next` returns the model after the command. It receives the result as a `Var`, during generation and during execution alike, which keeps the model free of real values and lets the shrinker regenerate the sequence.
- `Post` checks the real result against the model state before the command. It runs during execution only.

`pbt.Commands(init, gen)` is a generator of programs. The function `gen` receives the model state and returns a generator of the next command, which is where you decide what is possible and how likely it is: `close` is only offered while a ticket is open, and a `Frequency` sets the mix. `CommandsN` bounds the length, and a generator that returns `pbt.NoCommands()` ends the sequence. `RunSequential` executes the program, checks each postcondition, and records the name of every command for the statistics, so a passing run prints how often each command ran.

A type that implements the `Command` interface, with the methods `Precondition`, `Execute`, `NextState` and `Postcondition`, works in place of `Cmd`.

## Reading a failure

The next queue forgets to remove a closed ticket, so `Open` keeps counting it.

<!-- snippet: stateful_test.go#buggy-queue -->

The report shows the program, the history of the run and the model state before the step that failed.

```text
[pbt] property failed after 5 passed checks and 57 shrink steps (size 6)
[pbt] cmds (Commands) = Program{
[pbt]     var1 = issue()
[pbt]     var2 = close(var1)
[pbt]     var3 = open()
[pbt]   }
[pbt] log: history:
[pbt] log:   1. var1 = issue() -> 1
[pbt] log:   2. var2 = close(var1) -> <nil>
[pbt] log:   3. var3 = open() -> 1
[pbt] log: model state before step 3: {open:[]}
[pbt] error: step 3: postcondition of open() failed: 1 open tickets, want 0
[pbt] reproduce: go test -run 'TestQueue' -pbt.seed=0x1
```

The first failing sequence had several issues and closes. The shrinker removed commands one by one, re-checking the preconditions each time, until three remained: the shortest program that shows the leak. The failure is keyed by the check and the command, "postcondition of open", so shrinking does not trade it for a different failure.

## Writing good models

- Treat the model as immutable. `Next` returns a new value; copy slices and maps before changing them, as the example does with `slices.Clone`. A model that is modified in place breaks when the shrinker regenerates a sequence.
- Keep the model simpler than the system. The point is to state what the system should do in terms you can check, not to reimplement it.
- Choose commands from the state rather than rejecting them with `Pre` alone. A precondition that fails often wastes generated commands; a generator that offers the right commands for a state wastes none. Keep `Pre` as the guard that shrinking needs.
- Watch the statistics. `RunSequential` records every command name, so a passing run shows whether the mix of commands is the one you intended.
