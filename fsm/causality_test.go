// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package fsm_test

import (
	"fmt"
	"strings"
	"testing"

	"go.ymotongpoo.com/pbt"
	"go.ymotongpoo.com/pbt/fsm"
)

func TestCausalityNamesTransitionAndArguments(t *testing.T) {
	broken := false
	transitions := func(state string) []fsm.Transition {
		switch state {
		case "start":
			return []fsm.Transition{
				{Weight: 1, To: "waiting", Cmd: pbt.Just(pbt.Command{Name: "Put", Args: []any{1, 2}, Run: func(pbt.Env) any {
					broken = true
					return nil
				}})},
				{Weight: 15, To: "waiting", Cmd: pbt.Just(pbt.Command{Name: "Nop"})},
			}
		case "waiting":
			return []fsm.Transition{{Weight: 1, To: "checking", Cmd: pbt.Just(pbt.Command{Name: "Wait"})}}
		case "checking":
			return []fsm.Transition{{Weight: 1, To: "done", Cmd: pbt.Just(pbt.Command{Name: "Get", Post: func(pbt.Env, any) error {
				if broken {
					return fmt.Errorf("broken")
				}
				return nil
			}})}}
		}
		return nil
	}
	tb := &fakeTB{name: "causalFSM"}
	pbt.Check(tb, func(pt *pbt.T) {
		broken = false
		fsm.Commands("start", nil, transitions, pbt.Length(3, 3)).Draw(pt, "program").RunSequential(pt)
	}, pbt.Seed(1), pbt.Causality(64), pbt.Checks(1000), pbt.NoSave(), pbt.NoReplay())
	if len(tb.errors) != 1 || !strings.Contains(tb.errors[0], "largest measured rise at step 1: start -> waiting: Put(1, 2)") {
		t.Fatal(tb.errors)
	}
}
