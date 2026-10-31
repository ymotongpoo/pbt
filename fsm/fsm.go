// Package fsm tests state machines with named states, the equivalent of
// PropEr's proper_fsm. A Machine lists, for each state, the transitions that
// may fire with their weights; the package turns it into a stateful property
// on top of package pbt, where the model state is the pair of the state name
// and the state data.
package fsm

import (
	"fmt"

	"go.ymotongpoo.com/pbt"
)

// State is the model state of a machine: the name of the current state and
// the data that goes with it.
type State[N comparable, D any] struct {
	Name N
	Data D
}

// Transition is one way to leave a state. Weight controls how often the
// transition is generated relative to the others of the same state, as
// weight/3 does in proper_fsm; a weight of 0 is reachable only by shrinking.
// Cmd generates the command that performs the transition, with the
// precondition, state update and postcondition applying to the state data.
type Transition[N comparable, D, Sys any] struct {
	Weight int
	To     N
	Cmd    pbt.Gen[pbt.Command[D, Sys]]
}

// Machine describes a state machine. Transitions returns the transitions
// that may fire from a state with the given data; an empty list makes the
// state final and ends the command sequence.
type Machine[N comparable, D, Sys any] interface {
	Initial() (N, D)
	Transitions(name N, data D) []Transition[N, D, Sys]
}

// Commands generates command sequences that follow the machine. The commands
// carry the source and target state in their names, so the statistics show
// which transitions ran, as zip(state_names(H), command_names(Cmds)) does in
// the PropEr Testing book.
func Commands[N comparable, D, Sys any](m Machine[N, D, Sys]) pbt.Gen[pbt.Program[State[N, D], Sys]] {
	return pbt.Commands(initial(m), commandGen(m))
}

// ParallelCommands generates parallel programs that follow the machine; see
// pbt.ParallelCommands.
func ParallelCommands[N comparable, D, Sys any](m Machine[N, D, Sys]) pbt.Gen[pbt.ParallelProgram[State[N, D], Sys]] {
	return pbt.ParallelCommands(initial(m), commandGen(m))
}

func initial[N comparable, D, Sys any](m Machine[N, D, Sys]) State[N, D] {
	name, data := m.Initial()
	return State[N, D]{Name: name, Data: data}
}

func commandGen[N comparable, D, Sys any](m Machine[N, D, Sys]) func(State[N, D]) pbt.Gen[pbt.Command[State[N, D], Sys]] {
	return func(s State[N, D]) pbt.Gen[pbt.Command[State[N, D], Sys]] {
		trs := m.Transitions(s.Name, s.Data)
		alts := make([]pbt.Weighted[pbt.Command[State[N, D], Sys]], 0, len(trs))
		total := 0
		for _, tr := range trs {
			total += tr.Weight
			from, to := s.Name, tr.To
			alts = append(alts, pbt.Weighted[pbt.Command[State[N, D], Sys]]{
				Weight: tr.Weight,
				Gen: pbt.Map(tr.Cmd, func(c pbt.Command[D, Sys]) pbt.Command[State[N, D], Sys] {
					return transition[N, D, Sys]{from: from, to: to, inner: c}
				}),
			})
		}
		if len(alts) == 0 || total == 0 {
			return pbt.NoCommands[State[N, D], Sys]()
		}
		return pbt.Frequency(alts...)
	}
}

// transition adapts a command on the state data to a command on the full
// machine state.
type transition[N comparable, D, Sys any] struct {
	from, to N
	inner    pbt.Command[D, Sys]
}

func (t transition[N, D, Sys]) Precondition(s State[N, D]) bool {
	return s.Name == t.from && t.inner.Precondition(s.Data)
}

func (t transition[N, D, Sys]) Execute(sys Sys, env pbt.Env) any {
	return t.inner.Execute(sys, env)
}

func (t transition[N, D, Sys]) NextState(s State[N, D], result pbt.Var) State[N, D] {
	return State[N, D]{Name: t.to, Data: t.inner.NextState(s.Data, result)}
}

func (t transition[N, D, Sys]) Postcondition(s State[N, D], env pbt.Env, result any) error {
	return t.inner.Postcondition(s.Data, env, result)
}

// CommandName labels the command with its source state, for the statistics.
func (t transition[N, D, Sys]) CommandName() string {
	return fmt.Sprintf("%v: %s", t.from, pbt.CommandName(t.inner))
}

func (t transition[N, D, Sys]) String() string {
	return fmt.Sprintf("%v -> %v: %v", t.from, t.to, t.inner)
}
