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
// accountMachine names the states and lists the transitions out of each
// one. The state data is empty here; a machine can carry a model value too.
type accountMachine struct{}

func (accountMachine) Initial() (string, struct{}) { return "new", struct{}{} }

func (accountMachine) Transitions(state string, _ struct{}) []fsm.Transition[string, struct{}, *Account] {
	type cmd = pbt.Cmd[struct{}, *Account]
	expect := func(want error) func(struct{}, pbt.Env, any) error {
		return func(_ struct{}, _ pbt.Env, got any) error {
			err, _ := got.(error)
			if !errors.Is(err, want) {
				return errors.New("unexpected result")
			}
			return nil
		}
	}
	command := func(name string, call func(*Account) error, want error) pbt.Gen[pbt.Command[struct{}, *Account]] {
		return pbt.Just[pbt.Command[struct{}, *Account]](cmd{Name: name, Run: func(a *Account, _ pbt.Env) any { return call(a) }, Post: expect(want)})
	}
	type transition = fsm.Transition[string, struct{}, *Account]
	switch state {
	case "new":
		return []transition{
			{Weight: 3, To: "verified", Cmd: command("verify", (*Account).Verify, nil)},
			{Weight: 1, To: "new", Cmd: command("lock", (*Account).Lock, errState)},
		}
	case "verified":
		return []transition{
			{Weight: 2, To: "locked", Cmd: command("lock", (*Account).Lock, nil)},
			{Weight: 1, To: "verified", Cmd: command("verify", (*Account).Verify, errState)},
		}
	default: // locked
		return []transition{
			{Weight: 2, To: "verified", Cmd: command("unlock", (*Account).Unlock, nil)},
			{Weight: 1, To: "locked", Cmd: command("lock", (*Account).Lock, errState)},
		}
	}
}

func TestAccountMachine(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		prog := fsm.Commands[string, struct{}, *Account](accountMachine{}).Draw(t, "cmds")
		prog.RunSequential(t, &Account{state: "new"})
	})
}

// snippet: end

func TestAccountMachineLogs(t *testing.T) {
	logs := passingLogs(t, "TestAccountMachine", func(t *pbt.T) {
		prog := fsm.Commands[string, struct{}, *Account](accountMachine{}).Draw(t, "cmds")
		prog.RunSequential(t, &Account{state: "new"})
	}, pbt.Seed(1))
	mustContain(t, logs, "% new: verify", "% verified: lock")
}
