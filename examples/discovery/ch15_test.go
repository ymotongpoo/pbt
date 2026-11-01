// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"errors"
	"fmt"
	"testing"

	"go.ymotongpoo.com/pbt"
	"go.ymotongpoo.com/pbt/fsm"
)

type Account struct{ state string }

var errState = errors.New("invalid state")

func (a *Account) Verify() error {
	if a.state != "new" {
		return errState
	}
	a.state = "verified"
	return nil
}
func (a *Account) Lock() error {
	if a.state != "verified" {
		return errState
	}
	a.state = "locked"
	return nil
}
func (a *Account) Unlock() error {
	if a.state != "locked" {
		return errState
	}
	a.state = "verified"
	return nil
}

func TestAccountFSM(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		a := &Account{state: "new"}
		command := func(name string, op func() error, wantErr error) pbt.CommandGen {
			return pbt.Just(pbt.Command{
				Name: name,
				Run:  func(pbt.Env) any { return op() },
				Post: func(_ pbt.Env, got any) error {
					if wantErr == nil {
						if got != nil {
							return fmt.Errorf("%s failed: %v", name, got)
						}
						return nil
					}
					if got == nil {
						return fmt.Errorf("%s succeeded, want %v", name, wantErr)
					}
					return nil
				},
			})
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
	}, pbt.Verbose())
}
