# State machines

When a system moves between named states and allows different operations in each, a finite state machine is the natural model. The `fsm` package builds on the command machinery of chapter 5: you name the states and list the transitions out of each one, with weights, and the package generates command sequences that follow the machine. It is the counterpart of PropEr's `proper_fsm`.

## The system

<!-- snippet: fsm_test.go#account -->

## The machine

A machine implements `fsm.Machine[N, D, Sys]`: `N` is the type of the state names, `D` the state data carried alongside the name, and `Sys` the system under test. `Initial` returns the first state, and `Transitions` returns the transitions available in a state.

<!-- snippet: fsm_test.go#machine -->

Each `Transition` has a `Weight`, a target state `To`, and a `Cmd`, a generator of commands over the state data. The command's `Pre`, `Next` and `Post` see the data, not the name; the machine moves the name. A transition that stays in the same state, like a second `verify` in `verified`, is how a machine tests the operations that must be rejected.

`fsm.Commands(m)` returns a generator of programs whose model state is `fsm.State[N, D]`, the pair of name and data. The program runs with `RunSequential` as in chapter 5, and `fsm.ParallelCommands` gives the parallel form of chapter 6.

## Statistics by state

The commands are labeled with their source state, so the summary of a passing run shows which transitions the machine took:

```text
[pbt] OK, passed 100 checks (seed 0x1)
[pbt]  32.7% verified: lock
[pbt]  31.5% locked: unlock
[pbt]  17.2% verified: verify
[pbt]  15.1% locked: lock
[pbt]   2.8% new: verify
[pbt]   0.7% new: lock
```

The low share of `new` is expected: every sequence leaves that state after one or two commands and never returns. If a state matters and the statistics show that sequences rarely reach it, raise the weights of the transitions that lead there.
