# Stateful testing
<!-- summary: Commands as closures over a model and a system kept in the test's variables, Vars for results that do not exist yet, Commands and RunSequential, reading a stateful failure, locating an earlier cause with Causality, and writing good models. -->

Many bugs only show up after a sequence of operations: a cache that returns a stale value after an eviction, a queue that miscounts after a close. A stateful test describes the operations as commands, keeps a model of what the system should contain, generates sequences of commands against the model, runs them against the real system and checks every result. When a sequence fails, pbt shrinks it to the shortest one that still fails.

## The system under test

The example is a queue that issues ticket numbers and closes them.

<!-- snippet: stateful_test.go#queue -->

## The model and the commands

The model is what the test believes about the system. Here it is the list of open tickets. The ticket numbers do not exist while the sequence is generated, because nothing has run yet, so the model holds them as `pbt.Var` values, symbolic names for results. PropEr does the same with `{var, N}`.

The model and the system are variables of the property, and the commands are closures over them: nothing is passed around, and no type parameter appears. The property takes a constructor for the queue, so that the same commands run against the correct queue here and against a buggy one below.

<!-- snippet: stateful_test.go#model -->

A command is a `pbt.Command` value, a struct of functions.

- `Name` and `Args` are what the report prints. Put the arguments of the call in `Args`, with the `Var` of a result where the call uses one, so that the report reads like a program.
- `Pre` says whether the command is allowed in the current state of the model. It is checked whenever a command is generated, and a command that fails it is drawn again. Shrinking also generates commands: removing a command regenerates the ones after it from the new state. If no allowed command is found within the retry limit, the sequence ends, or the case is discarded if its minimum length has not been reached. `RunSequential` checks `Pre` once more before each call; a false result fails the case and usually means that the model was not reset or was changed outside `init` and `Next`.
- `Run` performs the call against the system. The `Env` maps every `Var` to the value its command returned, and `pbt.Resolve[T]` reads one with a type.
- `Next` updates the model for the command. It receives the result as a `Var`, during generation and during execution alike, which keeps the model free of real values and lets the shrinker regenerate the sequence.
- `Post` checks the real result against the model state before the command. It runs during execution only.

`pbt.Commands(init, gen)` is a generator of programs. `init` puts the model in its initial state, and pbt calls it before every generation and every run, so the variables of the model need no copying: `Next` changes them in place, and a replay starts again from `init`. The function `gen` returns the generator of the next command for the current state of the model, which is where you decide what is possible and how likely it is: `close` is only offered while a ticket is open, and a `Frequency` sets the mix, with `pbt.W` pairing each weight with its generator. The type of a command generator is `pbt.CommandGen`, an alias of `pbt.Gen[pbt.Command]`: Go infers the type argument in a call to `Map` or `Frequency` but not in the signature of a function you declare, and the alias is the name to write there. The option `pbt.Length(min, max)` bounds the length of the sequence, a generator that returns `pbt.NoCommands()` ends it, and `pbt.ModelState(f)` gives the report a function that shows the model. `RunSequential` executes the program, checks each postcondition, and records the name of every command for the statistics, so a passing run prints how often each command ran.

## Reading a failure

The next queue forgets to remove a closed ticket, so `Open` keeps counting it.

<!-- snippet: stateful_test.go#buggy-queue -->

The report shows the program, the history of the run and the model state before the step that failed.

```text
[pbt] property failed after 5 passed checks and 81 shrink steps (size 6)
[pbt] cmds (Commands) = Program{
[pbt]     var1 = issue()
[pbt]     var2 = close(var1)
[pbt]     var3 = open()
[pbt]   }
[pbt] log: history:
[pbt] log:   1. var1 = issue() -> 1
[pbt] log:   2. var2 = close(var1) -> <nil>
[pbt] log:   3. var3 = open() -> 1
[pbt] log: model state before step 3: []
[pbt] error: step 3: postcondition of open() failed: 1 open tickets, want 0
[pbt] reproduce: go test -run 'TestQueue' -pbt.seed=0x1
```

The first failing sequence had several issues and closes. The shrinker removed commands one by one, regenerating the following commands from the new model state and checking their preconditions, until three remained: the shortest program that shows the leak. The failure is keyed by the check and the command, "postcondition of open", so shrinking does not trade it for a different failure.

## Finding the step that caused a failure

The command that detects a failure can run long after the command that caused it. `Causality(n)` or `-pbt.causality=n` samples continuations of the first failing program before shrinking. The default, zero, disables this analysis.

For each sampled prefix, pbt keeps the choices through its last command and generates `n` fresh continuations at the original size. The report gives the fraction that fail with the original failure identity. Discarded continuations do not enter that fraction. A rise suggests that a command in the retained prefix made the failure more likely.

This property loses a value during `write(7)`, then detects the loss in `read()` after four unrelated commands:

<!-- snippet: causality_test.go#causality -->

`TestCausalityFindsEarlyWrite` runs it with `Seed(1)` and `Causality(32)`. The report contains these lines; the program, history and later prefix rates are omitted here:

```text
[pbt] property failed after 38 passed checks and 16 shrink steps (size 39)
[pbt] error: step 6: postcondition of read() failed: read 0, want 7
[pbt] reproduce: go test -run 'TestDelayedWrite' -pbt.seed=0x1
[pbt] causality (32 continuations per prefix, seed 0x1):
[pbt]   steps are generated commands of the first failing case, before shrinking in this run
[pbt]   after 0 of 6 steps: 0/32 fail with the same identity
[pbt]   after 1 of 6 steps: 32/32 fail with the same identity <- largest measured rise at step 1: write(7)
```

Keeping the first command makes every sampled continuation fail, although the postcondition fails at step 6. Here `Length(6, 6)` preserves the delayed check during shrinking. In a variable-length program, shrinking can remove commands and change step numbers. The causality report refers to the first failing program before that shrink.

Analysis costs at most `16*n` extra property executions, beyond the usual stability replay. It samples up to eight spaced prefixes and bisects the largest measured rise up to eight times. These executions run setup and cleanup, so reset the system for every property invocation. They do not count toward checks or statistics, call `OnFail`, or save cases.

The analysis applies to sequential `Commands` programs, including `fsm.Commands`. It skips parallel programs because generation order is not execution order. It also skips a failure that the stability replay cannot reproduce. `MaxFailures` analyzes only the first failure; `CheckExists` ignores the option. For a saved case, pbt analyzes the replayed program, since the original unshrunk program is unavailable.

A rise identifies a candidate cause, not proof. Sampling noise can hide a rise, rates can fall again, and a long trace may leave only a range of candidate steps.

## Writing good models

- Change the model only in `init` and `Next`; `Pre`, `Post` and `gen` read it. pbt replays a sequence by calling `init` and then the `Next` of every command, so a change made anywhere else is lost on the replay, and a value that `init` forgets to reset leaks from one case into the next.
- Keep the model simpler than the system. The point is to state what the system should do in terms you can check, not to reimplement it.
- Choose commands from the state rather than rejecting them with `Pre` alone. A precondition that fails often wastes generated commands; a generator that offers the right commands for a state wastes none. Still state the condition in `Pre`. If the generator produces only commands that satisfy it in the current model state, `Pre` rejects nothing during sequential generation, including shrinking. It filters candidates from generators that do not enforce the condition, and `ParallelCommands` uses it to drop commands from branch ends until every interleaving satisfies the preconditions.
- Watch the statistics. `RunSequential` records every command name, so a passing run shows whether the mix of commands is the one you intended.
