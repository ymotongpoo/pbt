// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package snippets

import (
	"errors"
	"testing"

	"go.ymotongpoo.com/pbt"
	"go.ymotongpoo.com/pbt/fsm"
)

// snippet: account
// Account moves between the states new, verified and locked.
type Account struct{ state string }

var errState = errors.New("not allowed in this state")

func (a *Account) Verify() error { return a.move("new", "verified") }
func (a *Account) Lock() error   { return a.move("verified", "locked") }
func (a *Account) Unlock() error { return a.move("locked", "verified") }

func (a *Account) move(from, to string) error {
	if a.state != from {
		return errState
	}
	a.state = to
	return nil
}

// snippet: end

// snippet: machine
// accountProperty names the states and lists the transitions out of each
// one. The machine carries no state data here; a machine that needs some
// keeps it in variables of the property, like the model of chapter 5.
func accountProperty(t *pbt.T) {
	a := &Account{state: "new"}
	expect := func(want error) func(pbt.Env, any) error {
		return func(_ pbt.Env, got any) error {
			err, _ := got.(error)
			if !errors.Is(err, want) {
				return errors.New("unexpected result")
			}
			return nil
		}
	}
	command := func(name string, call func() error, want error) pbt.Gen[pbt.Command] {
		return pbt.Just(pbt.Command{Name: name, Run: func(pbt.Env) any { return call() }, Post: expect(want)})
	}
	transitions := func(state string) []fsm.Transition {
		switch state {
		case "new":
			return []fsm.Transition{
				{Weight: 3, To: "verified", Cmd: command("verify", a.Verify, nil)},
				{Weight: 1, To: "new", Cmd: command("lock", a.Lock, errState)},
			}
		case "verified":
			return []fsm.Transition{
				{Weight: 2, To: "locked", Cmd: command("lock", a.Lock, nil)},
				{Weight: 1, To: "verified", Cmd: command("verify", a.Verify, errState)},
			}
		default: // locked
			return []fsm.Transition{
				{Weight: 2, To: "verified", Cmd: command("unlock", a.Unlock, nil)},
				{Weight: 1, To: "locked", Cmd: command("lock", a.Lock, errState)},
			}
		}
	}
	prog := fsm.Commands("new", nil, transitions).Draw(t, "cmds")
	prog.RunSequential(t)
}

func TestAccountMachine(t *testing.T) {
	pbt.Check(t, accountProperty)
}

// snippet: end

func TestAccountMachineLogs(t *testing.T) {
	logs := passingLogs(t, "TestAccountMachine", accountProperty, pbt.Seed(1))
	mustContain(t, logs, "% new -> verified: verify", "% verified -> locked: lock")
}
