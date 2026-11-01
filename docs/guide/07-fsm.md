# State machines
<!-- summary: The fsm package: named states, weighted transitions, state data in the test's variables, and statistics by transition. -->

When a system moves between named states and allows different operations in each, a finite state machine is the natural model. The `fsm` package builds on the command machinery of chapter 5: you name the states and list the transitions out of each one, with weights, and the package generates command sequences that follow the machine. It is the counterpart of PropEr's `proper_fsm`.

## The system

<!-- snippet: fsm_test.go#account -->

## The machine

A machine is a function from a state name to the transitions available in that state. `fsm.Commands(initial, init, transitions)` takes the name of the first state, a function that puts the state data in its initial state, and that function. The state data, when a machine has some, lives in variables of the property like the model of chapter 5; `init` may be nil when there is none.

<!-- snippet: fsm_test.go#machine -->

Each `Transition` has a `Weight`, a target state `To`, and a `Cmd`, the generator of the command, of type `pbt.CommandGen`. The command's `Pre`, `Next` and `Post` see the state data; the package moves the name. A transition that stays in the same state, like a second `verify` in `verified`, is how a machine tests the operations that must be rejected.

The program runs with `RunSequential` as in chapter 5, and `fsm.ParallelCommands` gives the parallel form of chapter 6. The report shows the state name before the failing step, next to whatever `pbt.ModelState` adds.

## Statistics by state

The commands carry their source and target state, so the summary of a passing run shows which transitions the machine took:

```text
[pbt] OK, passed 100 checks (seed 0x1)
[pbt]  32.7% verified -> locked: lock
[pbt]  31.5% locked -> verified: unlock
[pbt]  17.2% verified -> verified: verify
[pbt]  15.1% locked -> locked: lock
[pbt]   2.8% new -> verified: verify
[pbt]   0.7% new -> new: lock
```

The low share of `new` is expected: every sequence leaves that state after one or two commands and never returns. If a state matters and the statistics show that sequences rarely reach it, raise the weights of the transitions that lead there.
