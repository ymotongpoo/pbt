// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

// Package fsm tests state machines with named states, the equivalent of
// PropEr's proper_fsm. A machine lists, for each state, the transitions
// that may fire with their weights; the package turns it into a stateful
// property on top of package pbt, which tracks the name of the current
// state while the state data lives in the variables of the test, like the
// model of pbt.Commands.
package fsm

import (
	"fmt"

	"go.ymotongpoo.com/pbt"
)

// Transition is one way to leave a state. Weight controls how often the
// transition is generated relative to the others of the same state, as
// weight/3 does in proper_fsm; a weight of 0 is reachable only by
// shrinking. Cmd generates the command that performs the transition, with
// the precondition, state update and postcondition applying to the state
// data; the package moves the state name.
type Transition struct {
	Weight int
	To     string
	Cmd    pbt.CommandGen
}

// Commands generates command sequences that follow a machine. initial is
// the name of the first state, init puts the state data in its initial
// state, and transitions returns the transitions that may fire from a
// state; an empty list makes the state final and ends the sequence. The
// commands carry the source and target state in their names, so the
// program and the statistics show which transitions ran, as
// zip(state_names(H), command_names(Cmds)) does in the PropEr Testing
// book. The report shows the state name before the failing step, and any
// pbt.ModelState option next to it.
func Commands(initial string, init func(), transitions func(state string) []Transition, opts ...pbt.StatefulOption) pbt.Gen[pbt.Program] {
	m := newMachine(initial, init, transitions)
	return pbt.Commands(m.reset, m.gen, m.options(opts)...)
}

// ParallelCommands generates parallel programs that follow a machine; see
// pbt.ParallelCommands.
func ParallelCommands(initial string, init func(), transitions func(state string) []Transition, opts ...pbt.StatefulOption) pbt.Gen[pbt.ParallelProgram] {
	m := newMachine(initial, init, transitions)
	return pbt.ParallelCommands(m.reset, m.gen, m.options(opts)...)
}

// machine tracks the name of the current state alongside the test's own
// state data. pbt resets and replays it like the rest of the model.
type machine struct {
	initial     string
	init        func()
	transitions func(string) []Transition
	state       string
}

func newMachine(initial string, init func(), transitions func(string) []Transition) *machine {
	return &machine{initial: initial, init: init, transitions: transitions}
}

func (m *machine) reset() {
	m.state = m.initial
	if m.init != nil {
		m.init()
	}
}

func (m *machine) options(opts []pbt.StatefulOption) []pbt.StatefulOption {
	return append([]pbt.StatefulOption{pbt.ModelState(func() any { return m.state })}, opts...)
}

// gen returns the generator of the next command in the current state: a
// weighted choice among the transitions, each adapted to move the state.
func (m *machine) gen() pbt.CommandGen {
	trs := m.transitions(m.state)
	alts := make([]pbt.Weighted[pbt.Command], 0, len(trs))
	total := 0
	for _, tr := range trs {
		total += tr.Weight
		from, to := m.state, tr.To
		alts = append(alts, pbt.W(tr.Weight, pbt.Map(tr.Cmd, func(c pbt.Command) pbt.Command {
			return m.adapt(from, to, c)
		})))
	}
	if len(alts) == 0 || total == 0 {
		return pbt.NoCommands()
	}
	return pbt.Frequency(alts...)
}

// adapt turns a command on the state data into a command on the machine:
// it is allowed only in its source state, and moves the name to the target
// state after the data update.
func (m *machine) adapt(from, to string, c pbt.Command) pbt.Command {
	return pbt.Command{
		Name: fmt.Sprintf("%s -> %s: %s", from, to, c.Name),
		Args: c.Args,
		Pre: func() bool {
			return m.state == from && (c.Pre == nil || c.Pre())
		},
		Run: c.Run,
		Next: func(result pbt.Var) {
			if c.Next != nil {
				c.Next(result)
			}
			m.state = to
		},
		Post: c.Post,
	}
}
