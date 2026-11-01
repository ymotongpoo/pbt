// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package ch13

import (
	"fmt"
	"slices"
	"testing"

	"go.ymotongpoo.com/pbt"
	"go.ymotongpoo.com/pbt/examples/discovery/internal/mutant"
)

// Sessions はログインでトークンを発行し、ログアウトで無効化する。
type Sessions struct {
	next  int
	valid map[string]bool
}

func NewSessions() *Sessions { return &Sessions{valid: map[string]bool{}} }

func (s *Sessions) Login() string {
	s.next++
	tok := fmt.Sprintf("tok-%d", s.next)
	s.valid[tok] = true
	return tok
}

func (s *Sessions) Logout(tok string) { delete(s.valid, tok) }

func (s *Sessions) Validate(tok string) bool { return s.valid[tok] }

type sessionStore interface {
	Login() string
	Logout(tok string)
	Validate(tok string) bool
}

func sessionProperty(newStore func() sessionStore) pbt.Property {
	return func(t *pbt.T) {
		s := newStore()
		var active, dead []pbt.Var // モデル：有効なトークンと、無効化済みトークン

		login := pbt.Just(pbt.Command{
			Name: "login",
			Run:  func(pbt.Env) any { return s.Login() },
			Next: func(v pbt.Var) { active = append(active, v) },
		})
		logout := func() pbt.CommandGen {
			return pbt.Map(pbt.SampledFrom(active), func(v pbt.Var) pbt.Command {
				return pbt.Command{
					Name: "logout", Args: []any{v},
					Pre: func() bool { return slices.Contains(active, v) },
					Run: func(env pbt.Env) any {
						s.Logout(pbt.Resolve[string](env, v))
						return nil
					},
					Next: func(pbt.Var) { // 消すのではなく、過去側の集合へ移す
						active = slices.DeleteFunc(active, func(o pbt.Var) bool { return o == v })
						dead = append(dead, v)
					},
				}
			})
		}
		// validate は from の中のトークンを検証し、結果が want であることを確かめる。
		validate := func(name string, from *[]pbt.Var, want bool) pbt.CommandGen {
			return pbt.Map(pbt.SampledFrom(*from), func(v pbt.Var) pbt.Command {
				return pbt.Command{
					Name: name, Args: []any{v},
					Pre: func() bool { return slices.Contains(*from, v) },
					Run: func(env pbt.Env) any { return s.Validate(pbt.Resolve[string](env, v)) },
					Post: func(_ pbt.Env, got any) error {
						if got != want {
							return fmt.Errorf("%s(%v) = %v, want %v", name, v, got, want)
						}
						return nil
					},
				}
			})
		}
		commands := func() pbt.CommandGen {
			alts := []pbt.Weighted[pbt.Command]{pbt.W(3, login)}
			if len(active) > 0 {
				alts = append(alts, pbt.W(2, logout()), pbt.W(2, validate("validate_active", &active, true)))
			}
			if len(dead) > 0 {
				alts = append(alts, pbt.W(3, validate("validate_dead", &dead, false)))
			}
			return pbt.Frequency(alts...)
		}
		init := func() { active, dead = nil, nil }

		prog := pbt.Commands(init, commands).Draw(t, "cmds")
		prog.RunSequential(t)
	}
}

// noDeadProperty は dead を持たないモデル。logout はモデルから消すだけ。
func noDeadProperty(newStore func() sessionStore, checkAfterLogout bool) pbt.Property {
	return func(t *pbt.T) {
		s := newStore()
		var active []pbt.Var
		login := pbt.Just(pbt.Command{
			Name: "login",
			Run:  func(pbt.Env) any { return s.Login() },
			Next: func(v pbt.Var) { active = append(active, v) },
		})
		logout := func() pbt.CommandGen {
			return pbt.Map(pbt.SampledFrom(active), func(v pbt.Var) pbt.Command {
				return pbt.Command{
					Name: "logout", Args: []any{v},
					Pre: func() bool { return slices.Contains(active, v) },
					Run: func(env pbt.Env) any {
						tok := pbt.Resolve[string](env, v)
						s.Logout(tok)
						if checkAfterLogout {
							return s.Validate(tok)
						}
						return false
					},
					Next: func(pbt.Var) {
						active = slices.DeleteFunc(active, func(o pbt.Var) bool { return o == v })
					},
					Post: func(_ pbt.Env, got any) error {
						if got != false {
							return fmt.Errorf("token still valid right after logout")
						}
						return nil
					},
				}
			})
		}
		validateActive := func() pbt.CommandGen {
			return pbt.Map(pbt.SampledFrom(active), func(v pbt.Var) pbt.Command {
				return pbt.Command{
					Name: "validate_active", Args: []any{v},
					Pre: func() bool { return slices.Contains(active, v) },
					Run: func(env pbt.Env) any { return s.Validate(pbt.Resolve[string](env, v)) },
					Post: func(_ pbt.Env, got any) error {
						if got != true {
							return fmt.Errorf("active token rejected")
						}
						return nil
					},
				}
			})
		}
		commands := func() pbt.CommandGen {
			if len(active) == 0 {
				return login
			}
			return pbt.Frequency(pbt.W(3, login), pbt.W(2, logout()), pbt.W(2, validateActive()))
		}
		prog := pbt.Commands(func() { active = nil }, commands).Draw(t, "cmds")
		prog.RunSequential(t)
	}
}

// ---- 変異

// ignoreLogout はログアウトを無視する。
type ignoreLogout struct{ Sessions }

func (s *ignoreLogout) Logout(string) {}

// reuseToken はログアウト済みのトークンを次のログインで再利用する。
type reuseToken struct {
	Sessions
	freed []string
}

func (s *reuseToken) Logout(tok string) {
	s.Sessions.Logout(tok)
	s.freed = append(s.freed, tok)
}

func (s *reuseToken) Login() string {
	if len(s.freed) > 0 {
		tok := s.freed[0]
		s.freed = s.freed[1:]
		s.valid[tok] = true
		return tok
	}
	return s.Sessions.Login()
}

// resetDenylist は無効化済みのトークンを一覧で覚え、ログインのたびに一覧を初期化する。
type resetDenylist struct {
	Sessions
	revoked map[string]bool
	issued  map[string]bool
}

func (s *resetDenylist) Login() string {
	s.revoked = map[string]bool{} // バグ：一覧を初期化している
	tok := s.Sessions.Login()
	s.issued[tok] = true
	return tok
}
func (s *resetDenylist) Logout(tok string)        { s.revoked[tok] = true }
func (s *resetDenylist) Validate(tok string) bool { return s.issued[tok] && !s.revoked[tok] }

// logoutAll はログアウトで全トークンを無効化する。
type logoutAll struct{ Sessions }

func (s *logoutAll) Logout(string) { clear(s.valid) }

func mkOK() sessionStore     { return NewSessions() }
func mkIgnore() sessionStore { return &ignoreLogout{*NewSessions()} }
func mkReuse() sessionStore  { return &reuseToken{Sessions: *NewSessions()} }
func mkReset() sessionStore {
	return &resetDenylist{Sessions: *NewSessions(), revoked: map[string]bool{}, issued: map[string]bool{}}
}
func mkAll() sessionStore { return &logoutAll{*NewSessions()} }

var opts = []pbt.Option{pbt.NoSave(), pbt.NoReplay()}

func TestSessions(t *testing.T) { pbt.Check(t, sessionProperty(mkOK), opts...) }

func TestIgnoreLogoutMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, sessionProperty(mkIgnore), opts...)
}
func TestReuseTokenMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, sessionProperty(mkReuse), opts...)
}
func TestResetDenylistMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, sessionProperty(mkReset), opts...)
}
func TestLogoutAllMutant(t *testing.T) { mutant.Only(t); pbt.Check(t, sessionProperty(mkAll), opts...) }

func TestNoDeadIgnoreLogoutSurvives(t *testing.T) {
	pbt.Check(t, noDeadProperty(mkIgnore, false), append(opts, pbt.Checks(2000))...)
}
func TestNoDeadReuseTokenSurvives(t *testing.T) {
	pbt.Check(t, noDeadProperty(mkReuse, false), append(opts, pbt.Checks(2000))...)
}
func TestNoDeadResetDenylistSurvives(t *testing.T) {
	pbt.Check(t, noDeadProperty(mkReset, false), append(opts, pbt.Checks(2000))...)
}
func TestNoDeadLogoutAllMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, noDeadProperty(mkAll, false), opts...)
}

func TestAfterLogoutIgnoreLogoutMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, noDeadProperty(mkIgnore, true), opts...)
}
func TestAfterLogoutReuseTokenSurvives(t *testing.T) {
	pbt.Check(t, noDeadProperty(mkReuse, true), append(opts, pbt.Checks(2000))...)
}
func TestAfterLogoutResetDenylistSurvives(t *testing.T) {
	pbt.Check(t, noDeadProperty(mkReset, true), append(opts, pbt.Checks(2000))...)
}
func TestAfterLogoutLogoutAllMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, noDeadProperty(mkAll, true), opts...)
}
