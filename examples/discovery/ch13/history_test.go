// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package ch13

import (
	"fmt"
	"testing"

	"go.ymotongpoo.com/pbt"
	"go.ymotongpoo.com/pbt/examples/discovery/internal/mutant"
)

// LogSessions はイベントログに記録し、検証のたびにログをたどる実装である。
type event struct {
	kind string // "login" か "logout"
	tok  string
}

type LogSessions struct {
	next   int
	events []event
}

func (s *LogSessions) Login() string {
	s.next++
	tok := fmt.Sprintf("tok-%d", s.next)
	s.events = append(s.events, event{"login", tok})
	return tok
}

func (s *LogSessions) Logout(tok string) {
	s.events = append(s.events, event{"logout", tok})
}

// Validate にはバグがある。先頭から探して最初の記録で答えるので、
// ログアウトの記録まで届かない。
func (s *LogSessions) Validate(tok string) bool {
	for _, e := range s.events {
		if e.tok == tok {
			return e.kind == "login"
		}
	}
	return false
}

// modelEvent はモデルが持つ1操作の記録である。
type modelEvent struct {
	at   int // 操作の時刻の代わりの連番
	kind string
	tok  pbt.Var
}

// historyProperty は全操作の記録をモデルに持つ悪い例である。
func historyProperty(newStore func() sessionStore, predict func([]modelEvent, pbt.Var) bool) pbt.Property {
	return func(t *pbt.T) {
		s := newStore()
		var history []modelEvent
		var issued []pbt.Var

		login := pbt.Just(pbt.Command{
			Name: "login",
			Run:  func(pbt.Env) any { return s.Login() },
			Next: func(v pbt.Var) {
				history = append(history, modelEvent{len(history), "login", v})
				issued = append(issued, v)
			},
		})
		logout := func() pbt.CommandGen {
			return pbt.Map(pbt.SampledFrom(issued), func(v pbt.Var) pbt.Command {
				return pbt.Command{
					Name: "logout", Args: []any{v},
					Run: func(env pbt.Env) any {
						s.Logout(pbt.Resolve[string](env, v))
						return nil
					},
					Next: func(pbt.Var) {
						history = append(history, modelEvent{len(history), "logout", v})
					},
				}
			})
		}
		validate := func() pbt.CommandGen {
			return pbt.Map(pbt.SampledFrom(issued), func(v pbt.Var) pbt.Command {
				return pbt.Command{
					Name: "validate", Args: []any{v},
					Run: func(env pbt.Env) any { return s.Validate(pbt.Resolve[string](env, v)) },
					Post: func(_ pbt.Env, got any) error {
						if want := predict(history, v); got != want {
							return fmt.Errorf("validate(%v) = %v, want %v", v, got, want)
						}
						return nil
					},
				}
			})
		}
		commands := func() pbt.CommandGen {
			if len(issued) == 0 {
				return login
			}
			return pbt.Frequency(pbt.W(3, login), pbt.W(2, logout()), pbt.W(3, validate()))
		}
		init := func() { history, issued = nil, nil }

		prog := pbt.Commands(init, commands).Draw(t, "cmds")
		prog.RunSequential(t)
	}
}

// predictReplay は履歴を最初からたどり直し、最後の記録で答える。
func predictReplay(history []modelEvent, v pbt.Var) bool {
	valid := false
	for _, e := range history {
		if e.tok == v {
			valid = e.kind == "login"
		}
	}
	return valid
}

// predictCopy は対象の Validate をなぞって書いた予測で、同じバグを持つ。
func predictCopy(history []modelEvent, v pbt.Var) bool {
	for _, e := range history {
		if e.tok == v {
			return e.kind == "login"
		}
	}
	return false
}

func mkLog() sessionStore { return &LogSessions{} }

// 全履歴モデルは、正しい予測を書けば変異を捕まえる。
func TestHistoryReplayOK(t *testing.T) {
	pbt.Check(t, historyProperty(mkOK, predictReplay), opts...)
}
func TestHistoryReplayIgnoreLogoutMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, historyProperty(mkIgnore, predictReplay), opts...)
}
func TestHistoryReplayReuseTokenMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, historyProperty(mkReuse, predictReplay), opts...)
}
func TestHistoryReplayResetDenylistMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, historyProperty(mkReset, predictReplay), opts...)
}

// 対象をなぞった予測は、対象と同じバグを持つので通ってしまう。
func TestHistoryCopyLogBugSurvives(t *testing.T) {
	pbt.Check(t, historyProperty(mkLog, predictCopy), append(opts, pbt.Checks(2000))...)
}

// 過去側の集合のモデルは、同じ対象のバグを捕まえる。
func TestDeadModelLogBugMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, sessionProperty(mkLog), opts...)
}
