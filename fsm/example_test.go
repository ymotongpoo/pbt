// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package fsm_test

import (
	"errors"
	"testing"

	"go.ymotongpoo.com/pbt"
	"go.ymotongpoo.com/pbt/fsm"
)

// A door that opens and closes.
type door struct{ open bool }

var errAlready = errors.New("already in that state")

func (d *door) Open() error {
	if d.open {
		return errAlready
	}
	d.open = true
	return nil
}

func (d *door) Close() error {
	if !d.open {
		return errAlready
	}
	d.open = false
	return nil
}

// machine names the states and lists the transitions of each one, with
// weights.
type doorMachine struct{}

func (doorMachine) Initial() (string, struct{}) { return "closed", struct{}{} }

func (doorMachine) Transitions(state string, _ struct{}) []fsm.Transition[string, struct{}, *door] {
	type cmd = pbt.Cmd[struct{}, *door]
	expect := func(want error) func(struct{}, pbt.Env, any) error {
		return func(_ struct{}, _ pbt.Env, got any) error {
			if err, _ := got.(error); !errors.Is(err, want) && !(want == nil && err == nil) {
				return errors.New("unexpected result")
			}
			return nil
		}
	}
	open := pbt.Just[pbt.Command[struct{}, *door]](cmd{Name: "open", Run: func(d *door, _ pbt.Env) any { return d.Open() }, Post: expect(nil)})
	openAgain := pbt.Just[pbt.Command[struct{}, *door]](cmd{Name: "open", Run: func(d *door, _ pbt.Env) any { return d.Open() }, Post: expect(errAlready)})
	closeDoor := pbt.Just[pbt.Command[struct{}, *door]](cmd{Name: "close", Run: func(d *door, _ pbt.Env) any { return d.Close() }, Post: expect(nil)})
	closeAgain := pbt.Just[pbt.Command[struct{}, *door]](cmd{Name: "close", Run: func(d *door, _ pbt.Env) any { return d.Close() }, Post: expect(errAlready)})
	if state == "closed" {
		return []fsm.Transition[string, struct{}, *door]{
			{Weight: 3, To: "open", Cmd: open},
			{Weight: 1, To: "closed", Cmd: closeAgain},
		}
	}
	return []fsm.Transition[string, struct{}, *door]{
		{Weight: 3, To: "closed", Cmd: closeDoor},
		{Weight: 1, To: "open", Cmd: openAgain},
	}
}

// Commands turns the machine into a stateful property; the statistics
// label each command with its source state.
func ExampleCommands() {
	var t *testing.T
	pbt.Check(t, func(t *pbt.T) {
		prog := fsm.Commands[string, struct{}, *door](doorMachine{}).Draw(t, "cmds")
		prog.RunSequential(t, &door{})
	})
}
