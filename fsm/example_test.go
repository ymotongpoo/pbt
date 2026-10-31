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

// Commands turns a machine into a stateful property: the transitions of
// each state, with weights, and the commands that perform them. The
// statistics label each command with its source and target state.
func ExampleCommands() {
	var t *testing.T
	pbt.Check(t, func(t *pbt.T) {
		d := &door{}
		expect := func(want error) func(pbt.Env, any) error {
			return func(_ pbt.Env, got any) error {
				if err, _ := got.(error); !errors.Is(err, want) {
					return errors.New("unexpected result")
				}
				return nil
			}
		}
		open := func(want error) pbt.Gen[pbt.Command] {
			return pbt.Just(pbt.Command{Name: "open", Run: func(pbt.Env) any { return d.Open() }, Post: expect(want)})
		}
		closeDoor := func(want error) pbt.Gen[pbt.Command] {
			return pbt.Just(pbt.Command{Name: "close", Run: func(pbt.Env) any { return d.Close() }, Post: expect(want)})
		}
		transitions := func(state string) []fsm.Transition {
			if state == "closed" {
				return []fsm.Transition{
					{Weight: 3, To: "open", Cmd: open(nil)},
					{Weight: 1, To: "closed", Cmd: closeDoor(errAlready)},
				}
			}
			return []fsm.Transition{
				{Weight: 3, To: "closed", Cmd: closeDoor(nil)},
				{Weight: 1, To: "open", Cmd: open(errAlready)},
			}
		}
		prog := fsm.Commands("closed", nil, transitions).Draw(t, "cmds")
		prog.RunSequential(t)
	})
}
