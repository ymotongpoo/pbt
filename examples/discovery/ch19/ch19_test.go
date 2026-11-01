// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

// Package ch19 は第19章のバグの昇格の実例を実行する。
package ch19

import (
	"fmt"
	"runtime"
	"slices"
	"strconv"
	"sync"
	"testing"

	"go.ymotongpoo.com/pbt"
	"go.ymotongpoo.com/pbt/examples/discovery/internal/mutant"
)

var noSave = []pbt.Option{pbt.NoSave(), pbt.NoReplay()}

// ---- 実例1：値の範囲の話（割り勘）

// Split は total を n 人で分け、各人の額を返す。元のバグ入りの版。
func splitOriginal(total, n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = total / n
	}
	return out
}

// 変異：余りを先頭の1人に寄せる
func splitFirst(total, n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = total / n
	}
	out[0] += total % n
	return out
}

// 変異：余りを1人1単位ずつ配るが、負の余りを考えていない
func splitPositive(total, n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = total / n
	}
	for i := 0; i < total%n; i++ {
		out[i]++
	}
	return out
}

// Split は修正後の版である。
func Split(total, n int) []int {
	out := make([]int, n)
	q, r := total/n, total%n
	step := 1
	if r < 0 {
		step, r = -1, -r
	}
	for i := range out {
		out[i] = q
		if i < r {
			out[i] += step
		}
	}
	return out
}

func sum(xs []int) int {
	s := 0
	for _, x := range xs {
		s += x
	}
	return s
}

func regressionSplit(t testing.TB, Split func(int, int) []int) {
	got := Split(1000, 3)
	if s := sum(got); s != 1000 {
		t.Fatalf("sum(Split(1000, 3)) = %d, want 1000", s)
	}
}

func splitProperty(Split func(int, int) []int) pbt.Property {
	return func(t *pbt.T) {
		total := pbt.IntRange(-10000, 10000).Draw(t, "total")
		n := pbt.IntRange(1, 20).Draw(t, "n")

		got := Split(total, n)
		if len(got) != n {
			t.Fatalf("Split(%d, %d) has %d shares", total, n, len(got))
		}
		if s := sum(got); s != total {
			t.Fatalf("Split(%d, %d) = %v, which sums to %d", total, n, got, s)
		}
		if d := slices.Max(got) - slices.Min(got); d > 1 {
			t.Fatalf("Split(%d, %d) = %v, shares differ by %d", total, n, got, d)
		}
	}
}

func TestSplit1000By3(t *testing.T)               { regressionSplit(t, Split) }
func TestRegressionOriginalMutant(t *testing.T)   { mutant.Only(t); regressionSplit(t, splitOriginal) }
func TestRegressionFirstSurvives(t *testing.T)    { regressionSplit(t, splitFirst) }
func TestRegressionPositiveSurvives(t *testing.T) { regressionSplit(t, splitPositive) }

func TestSplitConserves(t *testing.T) { pbt.Check(t, splitProperty(Split)) }
func TestSplitOriginalMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, splitProperty(splitOriginal), noSave...)
}
func TestSplitFirstMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, splitProperty(splitFirst), noSave...)
}
func TestSplitPositiveMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, splitProperty(splitPositive), noSave...)
}

// ---- 実例2：履歴の話（トークンの再利用）

type sessions interface {
	Login() string
	Logout(string)
	Validate(string) bool
}

type Store struct {
	n     int
	valid map[string]bool
}

func NewStore() *Store { return &Store{valid: map[string]bool{}} }
func (s *Store) Login() string {
	s.n++
	k := strconv.Itoa(s.n)
	s.valid[k] = true
	return k
}
func (s *Store) Logout(k string)        { delete(s.valid, k) }
func (s *Store) Validate(k string) bool { return s.valid[k] }

// 元のバグ：解放したトークンを直後のログインで再利用する
type reuseStore struct {
	Store
	freed []string
}

func (s *reuseStore) Logout(k string) { s.Store.Logout(k); s.freed = append(s.freed, k) }
func (s *reuseStore) Login() string {
	if len(s.freed) > 0 {
		k := s.freed[0]
		s.freed = s.freed[1:]
		s.valid[k] = true
		return k
	}
	return s.Store.Login()
}

// 同系統の変異：解放したトークンが2つたまるまでは再利用しない
type poolStore struct {
	Store
	freed []string
}

func (s *poolStore) Logout(k string) { s.Store.Logout(k); s.freed = append(s.freed, k) }
func (s *poolStore) Login() string {
	if len(s.freed) >= 2 {
		k := s.freed[0]
		s.freed = s.freed[1:]
		s.valid[k] = true
		return k
	}
	return s.Store.Login()
}

// 同系統の変異：2回目のログアウトを無視する
type secondLogoutStore struct {
	Store
	logouts int
}

func (s *secondLogoutStore) Logout(k string) {
	s.logouts++
	if s.logouts%2 == 0 {
		return
	}
	s.Store.Logout(k)
}

func regressionToken(t testing.TB, s sessions) {
	tok := s.Login()
	s.Logout(tok)
	s.Login()
	if s.Validate(tok) {
		t.Fatalf("token %q is valid after logout", tok)
	}
}

func TestLogoutThenLogin(t *testing.T) { regressionToken(t, NewStore()) }
func TestRegressionReuseMutant(t *testing.T) {
	mutant.Only(t)
	regressionToken(t, &reuseStore{Store: *NewStore()})
}
func TestRegressionPoolSurvives(t *testing.T) { regressionToken(t, &poolStore{Store: *NewStore()}) }
func TestRegressionSecondLogoutSurvives(t *testing.T) {
	regressionToken(t, &secondLogoutStore{Store: *NewStore()})
}

func sessionProperty(mk func() sessions) pbt.Property {
	return func(t *pbt.T) {
		s := mk()
		var active, dead []pbt.Var

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
					Run: func(env pbt.Env) any { s.Logout(pbt.Resolve[string](env, v)); return nil },
					Next: func(pbt.Var) {
						active = slices.DeleteFunc(active, func(o pbt.Var) bool { return o == v })
						dead = append(dead, v)
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
							return fmt.Errorf("logged-out token is still valid")
						}
						return nil
					},
				}
			})
		}
		commands := func() pbt.CommandGen {
			alts := []pbt.Weighted[pbt.Command]{pbt.W(3, login)}
			if len(active) > 0 {
				alts = append(alts, pbt.W(2, logout()))
			}
			if len(dead) > 0 {
				alts = append(alts, pbt.W(3, validateDead()))
			}
			return pbt.Frequency(alts...)
		}
		init := func() { active, dead = nil, nil }
		pbt.Commands(init, commands).Draw(t, "cmds").RunSequential(t)
	}
}

func TestSessionsOK(t *testing.T) {
	pbt.Check(t, sessionProperty(func() sessions { return NewStore() }), noSave...)
}
func TestSessionsReuseMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, sessionProperty(func() sessions { return &reuseStore{Store: *NewStore()} }), noSave...)
}
func TestSessionsPoolMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, sessionProperty(func() sessions { return &poolStore{Store: *NewStore()} }), noSave...)
}
func TestSessionsSecondLogoutMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, sessionProperty(func() sessions { return &secondLogoutStore{Store: *NewStore()} }), noSave...)
}

// ---- 実例3：交互配置の話（更新の消失）

type adder interface {
	Add(int) int
	Value() int
}

type Counter struct {
	mu sync.Mutex
	n  int
}

func (c *Counter) Add(d int) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.n += d
	return c.n
}
func (c *Counter) Value() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.n
}

// 元のバグ：読みと書きの間でロックを手放す
type lostCounter struct{ Counter }

func (c *lostCounter) Add(d int) int {
	c.mu.Lock()
	v := c.n
	c.mu.Unlock()
	runtime.Gosched()
	c.mu.Lock()
	defer c.mu.Unlock()
	c.n = v + d
	return c.n
}

// 同系統の変異：更新は失われないが、返す値を別のロック区間で読む
type lateReadCounter struct{ Counter }

func (c *lateReadCounter) Add(d int) int {
	c.mu.Lock()
	c.n += d
	c.mu.Unlock()
	runtime.Gosched()
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.n
}

func regressionCounter(t testing.TB, c adder) {
	var wg sync.WaitGroup
	for range 1000 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.Add(1)
		}()
	}
	wg.Wait()
	if got := c.Value(); got != 1000 {
		t.Fatalf("counter = %d after 1000 adds", got)
	}
}

func TestConcurrentAdds(t *testing.T)                 { regressionCounter(t, &Counter{}) }
func TestConcurrentAddsLostMutant(t *testing.T)       { mutant.Only(t); regressionCounter(t, &lostCounter{}) }
func TestConcurrentAddsLateReadSurvives(t *testing.T) { regressionCounter(t, &lateReadCounter{}) }

func counterProperty(mk func() adder) pbt.Property {
	return func(t *pbt.T) {
		c := mk()
		n := 0
		add := pbt.Map(pbt.IntRange(1, 5), func(d int) pbt.Command {
			return pbt.Command{
				Name: "add", Args: []any{d},
				Run:  func(pbt.Env) any { return c.Add(d) },
				Next: func(pbt.Var) { n += d },
				Post: func(_ pbt.Env, got any) error {
					if got != n+d {
						return fmt.Errorf("add(%d) returned %v, want %d", d, got, n+d)
					}
					return nil
				},
			}
		})
		commands := func() pbt.CommandGen { return add }
		pbt.ParallelCommands(func() { n = 0 }, commands).Draw(t, "cmds").RunParallel(t)
	}
}

func TestCounterOK(t *testing.T) {
	pbt.Check(t, counterProperty(func() adder { return &Counter{} }), noSave...)
}
func TestCounterLostMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, counterProperty(func() adder { return &lostCounter{} }), noSave...)
}
func TestCounterLateReadMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, counterProperty(func() adder { return &lateReadCounter{} }), noSave...)
}

// 一覧の「到達の確認」欄の測定
func TestSplitReach(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		total := pbt.IntRange(-10000, 10000).Draw(t, "total")
		n := pbt.IntRange(1, 20).Draw(t, "n")

		t.Classify(total%n != 0, "remainder")
		t.Classify(total < 0, "negative total")
	}, append(noSave, pbt.Seed(1))...)
}
