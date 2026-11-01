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

// Expiring はトークンごとに有効期限を持つセッションストアである。
// 時刻は now から受け取るので、実時計に依存しない。
type Expiring struct {
	now     func() int
	ttl     int
	next    int
	expires map[string]int
}

func NewExpiring(now func() int, ttl int) *Expiring {
	return &Expiring{now: now, ttl: ttl, expires: map[string]int{}}
}

func (s *Expiring) Login() string {
	s.next++
	tok := fmt.Sprintf("tok-%d", s.next)
	s.expires[tok] = s.now() + s.ttl
	return tok
}

func (s *Expiring) Logout(tok string) { delete(s.expires, tok) }

// Validate は、有効期限に達する前のトークンだけを受け付ける。
func (s *Expiring) Validate(tok string) bool {
	exp, ok := s.expires[tok]
	return ok && s.now() < exp
}

type expiringStore interface {
	Login() string
	Logout(tok string)
	Validate(tok string) bool
}

const ttl = 5

// session はモデルが持つ有効なトークンと、その有効期限である。
type session struct {
	tok     pbt.Var
	expires int
}

func expiringProperty(newStore func(now func() int) expiringStore, steps pbt.Gen[int]) pbt.Property {
	return func(t *pbt.T) {
		clock := 0 // 対象が読む時計。Run の中でだけ進める
		s := newStore(func() int { return clock })

		now := 0 // モデルの時計。Next の中でだけ進める
		var active []session
		var dead []pbt.Var

		login := pbt.Just(pbt.Command{
			Name: "login",
			Run:  func(pbt.Env) any { return s.Login() },
			Next: func(v pbt.Var) { active = append(active, session{v, now + ttl}) },
		})
		advance := pbt.Map(steps, func(d int) pbt.Command {
			return pbt.Command{
				Name: "advance", Args: []any{d},
				Run: func(pbt.Env) any { clock += d; return nil },
				Next: func(pbt.Var) {
					now += d
					for _, a := range active {
						if a.expires <= now { // 仕様：有効期限に達した時点で無効になる
							dead = append(dead, a.tok)
						}
					}
					active = slices.DeleteFunc(active, func(a session) bool { return a.expires <= now })
				},
			}
		})
		logout := func() pbt.CommandGen {
			return pbt.Map(pbt.SampledFrom(active), func(a session) pbt.Command {
				return pbt.Command{
					Name: "logout", Args: []any{a.tok},
					Pre: func() bool { return slices.Contains(active, a) },
					Run: func(env pbt.Env) any {
						s.Logout(pbt.Resolve[string](env, a.tok))
						return nil
					},
					Next: func(pbt.Var) {
						active = slices.DeleteFunc(active, func(o session) bool { return o == a })
						dead = append(dead, a.tok)
					},
				}
			})
		}
		validateActive := func() pbt.CommandGen {
			return pbt.Map(pbt.SampledFrom(active), func(a session) pbt.Command {
				return pbt.Command{
					Name: "validate_active", Args: []any{a.tok},
					Pre: func() bool { return slices.Contains(active, a) },
					Run: func(env pbt.Env) any { return s.Validate(pbt.Resolve[string](env, a.tok)) },
					Post: func(_ pbt.Env, got any) error {
						if got != true {
							return fmt.Errorf("token expiring at %d rejected at %d", a.expires, now)
						}
						return nil
					},
				}
			})
		}
		validateDead := func() pbt.CommandGen {
			return pbt.Map(pbt.SampledFrom(dead), func(v pbt.Var) pbt.Command {
				return pbt.Command{
					Name: "validate_dead", Args: []any{v},
					Pre: func() bool { return slices.Contains(dead, v) },
					Run: func(env pbt.Env) any { return s.Validate(pbt.Resolve[string](env, v)) },
					Post: func(_ pbt.Env, got any) error {
						if got != false {
							return fmt.Errorf("dead token accepted at %d", now)
						}
						return nil
					},
				}
			})
		}
		commands := func() pbt.CommandGen {
			alts := []pbt.Weighted[pbt.Command]{pbt.W(3, login), pbt.W(3, advance)}
			if len(active) > 0 {
				alts = append(alts, pbt.W(1, logout()), pbt.W(2, validateActive()))
			}
			if len(dead) > 0 {
				alts = append(alts, pbt.W(3, validateDead()))
			}
			return pbt.Frequency(alts...)
		}
		init := func() { now, active, dead = 0, nil, nil }

		prog := pbt.Commands(init, commands).Draw(t, "cmds")
		prog.RunSequential(t)
	}
}

// ---- 変異

// inclusiveExpiry は有効期限ちょうどの時刻でも受け付けてしまう。
type inclusiveExpiry struct{ Expiring }

func (s *inclusiveExpiry) Validate(tok string) bool {
	exp, ok := s.expires[tok]
	return ok && s.now() <= exp // バグ：< のはずが <= になっている
}

// earlyExpiry は期限の1手前で無効にする。
type earlyExpiry struct{ Expiring }

func (s *earlyExpiry) Validate(tok string) bool {
	exp, ok := s.expires[tok]
	return ok && s.now() < exp-1
}

// slidingExpiry は検証のたびに期限を延ばしてしまう。
type slidingExpiry struct{ Expiring }

func (s *slidingExpiry) Validate(tok string) bool {
	ok := s.Expiring.Validate(tok)
	if ok {
		s.expires[tok] = s.now() + s.ttl
	}
	return ok
}

var fine = pbt.IntRange(1, 3)
var coarse = pbt.SampledFrom([]int{10, 20})

func okStore(now func() int) expiringStore { return NewExpiring(now, ttl) }

func TestExpiring(t *testing.T) { pbt.Check(t, expiringProperty(okStore, fine), opts...) }
func TestInclusiveExpiryMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, expiringProperty(func(now func() int) expiringStore { return &inclusiveExpiry{*NewExpiring(now, ttl)} }, fine), opts...)
}
func TestEarlyExpiryMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, expiringProperty(func(now func() int) expiringStore { return &earlyExpiry{*NewExpiring(now, ttl)} }, fine), opts...)
}
func TestSlidingExpiryMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, expiringProperty(func(now func() int) expiringStore { return &slidingExpiry{*NewExpiring(now, ttl)} }, fine), opts...)
}

// 時計の進み方が荒いと、有効期限ちょうどの時刻に届かない。
func TestInclusiveExpiryCoarseSurvives(t *testing.T) {
	pbt.Check(t, expiringProperty(func(now func() int) expiringStore { return &inclusiveExpiry{*NewExpiring(now, ttl)} }, coarse), append(opts, pbt.Checks(2000))...)
}
