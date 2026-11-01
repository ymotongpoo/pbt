// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

// Package ch17 は第17章の弱いプロパティの例と変異を実行する。
package ch17

import (
	"errors"
	"fmt"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"

	"go.ymotongpoo.com/pbt"
	"go.ymotongpoo.com/pbt/examples/discovery/internal/mutant"
)

var noSave = []pbt.Option{pbt.NoSave(), pbt.NoReplay()}

// ---- 題材：largest

func largestOK(xs []int) int {
	m := xs[0]
	for _, x := range xs[1:] {
		if x > m {
			m = x
		}
	}
	return m
}

func firstElement(xs []int) int { return xs[0] }

func largestZeroInit(xs []int) int { // 初期値の変更
	m := 0
	for _, x := range xs {
		if x > m {
			m = x
		}
	}
	return m
}

func largestFlipped(xs []int) int { // 比較の反転
	m := xs[0]
	for _, x := range xs[1:] {
		if x < m {
			m = x
		}
	}
	return m
}

// 症状1：常に真
func aliasProperty(largest func([]int) int) pbt.Property {
	return func(t *pbt.T) {
		xs := pbt.SliceOfN(pbt.Int(), 1, -1).Draw(t, "xs")
		ys := xs // 複製のつもり
		slices.Reverse(ys)
		if largest(xs) != largest(ys) {
			t.Fatalf("largest(%v) != largest(%v)", xs, ys)
		}
	}
}

func cloneProperty(largest func([]int) int) pbt.Property {
	return func(t *pbt.T) {
		xs := pbt.SliceOfN(pbt.Int(), 1, -1).Draw(t, "xs")
		ys := slices.Clone(xs)
		slices.Reverse(ys)
		if largest(xs) != largest(ys) {
			t.Fatalf("largest(%v) != largest(%v)", xs, ys)
		}
	}
}

func TestAliasOK(t *testing.T)            { pbt.Check(t, aliasProperty(largestOK), noSave...) }
func TestAliasFirstSurvives(t *testing.T) { pbt.Check(t, aliasProperty(firstElement), noSave...) }
func TestCloneOK(t *testing.T)            { pbt.Check(t, cloneProperty(largestOK), noSave...) }
func TestCloneFirstMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, cloneProperty(firstElement), noSave...)
}

// 表：初期値の変更と比較の反転
func elementProperty(largest func([]int) int) pbt.Property {
	return func(t *pbt.T) {
		xs := pbt.SliceOfN(pbt.Int(), 1, -1).Draw(t, "xs")
		if got := largest(xs); !slices.Contains(xs, got) {
			t.Fatalf("largest(%v) = %d, which is not an element", xs, got)
		}
	}
}

func upperProperty(largest func([]int) int) pbt.Property {
	return func(t *pbt.T) {
		xs := pbt.SliceOfN(pbt.Int(), 1, -1).Draw(t, "xs")
		got := largest(xs)
		for _, x := range xs {
			if got < x {
				t.Fatalf("largest(%v) = %d, but %d is larger", xs, got, x)
			}
		}
	}
}

func TestLargestZeroInitMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, elementProperty(largestZeroInit), noSave...)
}
func TestLargestFlippedMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, upperProperty(largestFlipped), noSave...)
}
func TestLargestFlippedElementSurvives(t *testing.T) {
	pbt.Check(t, elementProperty(largestFlipped), noSave...)
}

// ---- 症状2：前提がケースを減らす

func indexLinear(xs []int, v int) int {
	for i, x := range xs {
		if x == v {
			return i
		}
	}
	return -1
}

func indexSortedOK(xs []int, v int) int {
	if len(xs) < 16 {
		return indexLinear(xs, v)
	}
	lo, hi := 0, len(xs)
	for lo < hi {
		mid := (lo + hi) / 2
		if xs[mid] < v {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	if lo < len(xs) && xs[lo] == v {
		return lo
	}
	return -1
}

func indexSortedBad(xs []int, v int) int { // 比較の変異
	if len(xs) < 16 {
		return indexLinear(xs, v)
	}
	lo, hi := 0, len(xs)
	for lo < hi {
		mid := (lo + hi) / 2
		if xs[mid] <= v {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	if lo < len(xs) && xs[lo] == v {
		return lo
	}
	return -1
}

func assumeWideProperty(IndexSorted func([]int, int) int) pbt.Property {
	return func(t *pbt.T) {
		xs := pbt.SliceOf(pbt.IntRange(0, 30)).Draw(t, "xs")
		v := pbt.IntRange(0, 30).Draw(t, "v")
		t.Assume(slices.IsSorted(xs))

		if got, want := IndexSorted(xs, v), indexLinear(xs, v); got != want {
			t.Fatalf("IndexSorted(%v, %d) = %d, model says %d", xs, v, got, want)
		}
	}
}

func assumeProperty(IndexSorted func([]int, int) int) pbt.Property {
	return func(t *pbt.T) {
		xs := pbt.SliceOf(pbt.IntRange(0, 3)).Draw(t, "xs")
		v := pbt.IntRange(0, 3).Draw(t, "v")
		t.Assume(slices.IsSorted(xs))
		t.Measure("len", float64(len(xs)))

		if got, want := IndexSorted(xs, v), indexLinear(xs, v); got != want {
			t.Fatalf("IndexSorted(%v, %d) = %d, model says %d", xs, v, got, want)
		}
	}
}

var sortedDistinct = pbt.Map(
	pbt.SliceOf(pbt.IntRange(0, 30)),
	func(xs []int) []int {
		slices.Sort(xs)
		return slices.Compact(xs)
	},
)

func generatorProperty(IndexSorted func([]int, int) int) pbt.Property {
	return func(t *pbt.T) {
		xs := sortedDistinct.Draw(t, "xs")
		v := pbt.IntRange(0, 30).Draw(t, "v")
		t.Measure("len", float64(len(xs)))

		if got, want := IndexSorted(xs, v), indexLinear(xs, v); got != want {
			t.Fatalf("IndexSorted(%v, %d) = %d, model says %d", xs, v, got, want)
		}
	}
}

func TestAssumeOK(t *testing.T) {
	pbt.Check(t, assumeProperty(indexSortedOK), append(noSave, pbt.Verbose(), pbt.Seed(1))...)
}
func TestAssumeBadSurvives(t *testing.T) {
	pbt.Check(t, assumeProperty(indexSortedBad), append(noSave, pbt.Seed(1))...)
}
func TestGeneratorOK(t *testing.T) {
	pbt.Check(t, generatorProperty(indexSortedOK), append(noSave, pbt.Verbose())...)
}
func TestGeneratorBadMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, generatorProperty(indexSortedBad), noSave...)
}

// 破棄が上限を超えると実行は失敗する（名前は Mutant だが、対象は正しい実装）
func TestAssumeWideGivesUpMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, assumeWideProperty(indexSortedOK), noSave...)
}

// ---- 症状3：実装のコピー

func truncateBytes(s string, n int) string { // バイトで切る誤解
	if n <= 0 {
		return ""
	}
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func truncateOK(s string, n int) string {
	if n <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

func copyProperty(Truncate func(string, int) string) pbt.Property {
	return func(t *pbt.T) {
		s := pbt.String().Draw(t, "s")
		n := pbt.IntRange(-2, 40).Draw(t, "n")

		want := s
		if n <= 0 {
			want = ""
		} else if len(s) > n {
			want = s[:n]
		}
		if got := Truncate(s, n); got != want {
			t.Fatalf("Truncate(%q, %d) = %q, want %q", s, n, got, want)
		}
	}
}

func specProperty(Truncate func(string, int) string) pbt.Property {
	return func(t *pbt.T) {
		s := pbt.String().Draw(t, "s")
		n := pbt.IntRange(0, 40).Draw(t, "n")

		got := Truncate(s, n)
		if !strings.HasPrefix(s, got) {
			t.Fatalf("Truncate(%q, %d) = %q is not a prefix", s, n, got)
		}
		if c, want := utf8.RuneCountInString(got), min(n, utf8.RuneCountInString(s)); c != want {
			t.Fatalf("Truncate(%q, %d) has %d runes, want %d", s, n, c, want)
		}
	}
}

func TestCopyBytesSurvives(t *testing.T) { pbt.Check(t, copyProperty(truncateBytes), noSave...) }
func TestCopyRunesFailsCorrectMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, copyProperty(truncateOK), noSave...)
}
func TestSpecOK(t *testing.T) { pbt.Check(t, specProperty(truncateOK), noSave...) }
func TestSpecBytesMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, specProperty(truncateBytes), noSave...)
}

// ---- 症状4：生成器が単調

func parseLimitOK(s string) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, errors.New("empty")
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0, err
	}
	if v < 1 || v > 1000 {
		return 0, errors.New("out of range")
	}
	return v, nil
}

func parseLimitBoundary(s string) (int, error) { // 境界の変異
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, errors.New("empty")
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0, err
	}
	if v < 1 || v >= 1000 {
		return 0, errors.New("out of range")
	}
	return v, nil
}

func narrowProperty(ParseLimit func(string) (int, error)) pbt.Property {
	return func(t *pbt.T) {
		v := pbt.IntRange(1, 100).Draw(t, "v")
		t.Classify(v == 1000, "upper bound")
		got, err := ParseLimit(strconv.Itoa(v))
		if err != nil || got != v {
			t.Fatalf("ParseLimit(%q) = %d, %v; want %d, nil", strconv.Itoa(v), got, err, v)
		}
	}
}

func fullRangeProperty(ParseLimit func(string) (int, error)) pbt.Property {
	return func(t *pbt.T) {
		v := pbt.IntRange(1, 1000).Draw(t, "v")
		t.Classify(v == 1000, "upper bound")
		got, err := ParseLimit(strconv.Itoa(v))
		if err != nil || got != v {
			t.Fatalf("ParseLimit(%q) = %d, %v; want %d, nil", strconv.Itoa(v), got, err, v)
		}
	}
}

var limitValue = pbt.Frequency(
	pbt.W(3, pbt.IntRange(1, 1000)),
	pbt.W(1, pbt.SampledFrom([]int{1, 2, 999, 1000})),
)

func boundaryProperty(ParseLimit func(string) (int, error)) pbt.Property {
	return func(t *pbt.T) {
		v := limitValue.Draw(t, "v")
		t.Classify(v == 1000, "upper bound")
		got, err := ParseLimit(strconv.Itoa(v))
		if err != nil || got != v {
			t.Fatalf("ParseLimit(%q) = %d, %v; want %d, nil", strconv.Itoa(v), got, err, v)
		}
	}
}

func TestNarrowOK(t *testing.T) {
	pbt.Check(t, narrowProperty(parseLimitOK), append(noSave, pbt.Verbose())...)
}
func TestNarrowBoundarySurvives(t *testing.T) {
	pbt.Check(t, narrowProperty(parseLimitBoundary), noSave...)
}
func TestFullRangeStats(t *testing.T) {
	pbt.Check(t, fullRangeProperty(parseLimitOK), append(noSave, pbt.Verbose())...)
}
func TestFullRangeBoundaryMaybeMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, fullRangeProperty(parseLimitBoundary), noSave...)
}
func TestBoundaryStats(t *testing.T) {
	pbt.Check(t, boundaryProperty(parseLimitOK), append(noSave, pbt.Verbose())...)
}
func TestBoundaryMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, boundaryProperty(parseLimitBoundary), noSave...)
}

// ---- 症状5と表：台帳

type ledger interface {
	Deposit(int) int
	Withdraw(int) error
	Balance() int
}

type Ledger struct{ bal int }

var ErrInsufficient = errors.New("insufficient funds")

func (l *Ledger) Deposit(d int) int { l.bal += d; return l.bal }
func (l *Ledger) Withdraw(d int) error {
	if d > l.bal {
		return ErrInsufficient
	}
	l.bal -= d
	return nil
}
func (l *Ledger) Balance() int { return l.bal }

type lenientLedger struct{ Ledger } // エラー条件の無効化

func (l *lenientLedger) Withdraw(d int) error { l.bal -= d; return nil }

type doubleDepositLedger struct{ Ledger } // 二重適用

func (l *doubleDepositLedger) Deposit(d int) int { l.bal += 2 * d; return l.bal }

type forgetWithdrawLedger struct{ Ledger } // 更新のし忘れ

func (l *forgetWithdrawLedger) Withdraw(d int) error {
	if d > l.bal {
		return ErrInsufficient
	}
	return nil
}

// 悪い例：balance の Post がモデルの量しか見ていない
func modelOnlyProperty(newLedger func() ledger, checkGot bool) pbt.Property {
	return func(t *pbt.T) {
		l := newLedger()
		var bal, deposited, withdrawn int

		deposit := pbt.Map(pbt.IntRange(1, 100), func(d int) pbt.Command {
			return pbt.Command{
				Name: "deposit", Args: []any{d},
				Run:  func(pbt.Env) any { return l.Deposit(d) },
				Next: func(pbt.Var) { bal += d; deposited += d },
			}
		})
		withdraw := func() pbt.CommandGen {
			return pbt.Map(pbt.IntRange(1, bal), func(d int) pbt.Command {
				return pbt.Command{
					Name: "withdraw", Args: []any{d},
					Pre:  func() bool { return d <= bal },
					Run:  func(pbt.Env) any { return l.Withdraw(d) },
					Next: func(pbt.Var) { bal -= d; withdrawn += d },
					Post: func(_ pbt.Env, got any) error {
						if got != nil {
							return fmt.Errorf("withdraw(%d) failed: %v", d, got)
						}
						return nil
					},
				}
			})
		}
		balanceBad := pbt.Just(pbt.Command{
			Name: "balance",
			Run:  func(pbt.Env) any { return l.Balance() },
			Post: func(pbt.Env, any) error {
				if bal != deposited-withdrawn {
					return fmt.Errorf("balance %d, want %d", bal, deposited-withdrawn)
				}
				return nil
			},
		})
		balanceGood := pbt.Just(pbt.Command{
			Name: "balance",
			Run:  func(pbt.Env) any { return l.Balance() },
			Post: func(_ pbt.Env, got any) error {
				if got != bal {
					return fmt.Errorf("balance = %v, want %d", got, bal)
				}
				return nil
			},
		})
		balance := balanceBad
		if checkGot {
			balance = balanceGood
		}
		commands := func() pbt.CommandGen {
			if bal == 0 {
				return pbt.OneOf(deposit, balance)
			}
			return pbt.OneOf(deposit, withdraw(), balance)
		}
		init := func() { bal, deposited, withdrawn = 0, 0, 0 }
		pbt.Commands(init, commands).Draw(t, "cmds").RunSequential(t)
	}
}

func mkOK() ledger      { return &Ledger{} }
func mkLenient() ledger { return &lenientLedger{} }
func mkDouble() ledger  { return &doubleDepositLedger{} }
func mkForget() ledger  { return &forgetWithdrawLedger{} }

func TestModelOnlyOK(t *testing.T) { pbt.Check(t, modelOnlyProperty(mkOK, false), noSave...) }
func TestModelOnlyDoubleSurvives(t *testing.T) {
	pbt.Check(t, modelOnlyProperty(mkDouble, false), noSave...)
}
func TestModelOnlyForgetSurvives(t *testing.T) {
	pbt.Check(t, modelOnlyProperty(mkForget, false), noSave...)
}
func TestGotOK(t *testing.T) { pbt.Check(t, modelOnlyProperty(mkOK, true), noSave...) }
func TestGotDoubleMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, modelOnlyProperty(mkDouble, true), noSave...)
}
func TestGotForgetMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, modelOnlyProperty(mkForget, true), noSave...)
}

// 第11章の完全なプロパティ
func ledgerProperty(newLedger func() ledger) pbt.Property {
	return func(t *pbt.T) {
		l := newLedger()
		bal := 0

		deposit := pbt.Map(pbt.IntRange(1, 100), func(d int) pbt.Command {
			return pbt.Command{
				Name: "deposit", Args: []any{d},
				Run:  func(pbt.Env) any { return l.Deposit(d) },
				Next: func(pbt.Var) { bal += d },
				Post: func(_ pbt.Env, got any) error {
					if got.(int) != bal+d {
						return fmt.Errorf("deposit(%d) returned balance %v, want %d", d, got, bal+d)
					}
					return nil
				},
			}
		})
		withdraw := func() pbt.CommandGen {
			return pbt.Map(pbt.IntRange(1, bal), func(d int) pbt.Command {
				return pbt.Command{
					Name: "withdraw", Args: []any{d},
					Pre:  func() bool { return d <= bal },
					Run:  func(pbt.Env) any { return l.Withdraw(d) },
					Next: func(pbt.Var) { bal -= d },
					Post: func(_ pbt.Env, got any) error {
						if got != nil {
							return fmt.Errorf("withdraw(%d) failed: %v", d, got)
						}
						return nil
					},
				}
			})
		}
		overdraft := func() pbt.CommandGen {
			return pbt.Map(pbt.IntRange(1, 50), func(e int) pbt.Command {
				amt := bal + e
				return pbt.Command{
					Name: "overdraft", Args: []any{amt},
					Run: func(pbt.Env) any { return l.Withdraw(amt) },
					Post: func(_ pbt.Env, got any) error {
						if got == nil {
							return fmt.Errorf("withdraw(%d) succeeded, want error", amt)
						}
						return nil
					},
				}
			})
		}
		balance := pbt.Just(pbt.Command{
			Name: "balance",
			Run:  func(pbt.Env) any { return l.Balance() },
			Post: func(_ pbt.Env, got any) error {
				if got != bal {
					return fmt.Errorf("balance = %v, want %d", got, bal)
				}
				return nil
			},
		})
		commands := func() pbt.CommandGen {
			if bal == 0 {
				return pbt.OneOf(deposit, overdraft(), balance)
			}
			return pbt.OneOf(deposit, withdraw(), overdraft(), balance)
		}
		init := func() { bal = 0 }
		pbt.Commands(init, commands).Draw(t, "cmds").RunSequential(t)
	}
}

func TestLedgerOK(t *testing.T) { pbt.Check(t, ledgerProperty(mkOK), noSave...) }
func TestLedgerLenientMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, ledgerProperty(mkLenient), noSave...)
}
func TestLedgerDoubleMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, ledgerProperty(mkDouble), noSave...)
}
func TestLedgerForgetMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, ledgerProperty(mkForget), noSave...)
}

// ---- 表：チケットキューの消し忘れ

type ticketQueue interface {
	Issue() int
	Close(int) error
	Open() int
}

type Queue struct {
	next int
	open map[int]bool
}

var errNotOpen = errors.New("ticket is not open")

func (q *Queue) Issue() int { q.next++; q.open[q.next] = true; return q.next }
func (q *Queue) Close(id int) error {
	if !q.open[id] {
		return errNotOpen
	}
	delete(q.open, id)
	return nil
}
func (q *Queue) Open() int { return len(q.open) }

type forgetDeleteQueue struct{ Queue } // 削除のし忘れ

func (q *forgetDeleteQueue) Close(id int) error {
	if !q.open[id] {
		return errNotOpen
	}
	return nil
}

func queueProperty(newQueue func() ticketQueue) pbt.Property {
	return func(t *pbt.T) {
		q := newQueue()
		var open []pbt.Var
		issue := pbt.Just(pbt.Command{
			Name: "issue",
			Run:  func(pbt.Env) any { return q.Issue() },
			Next: func(v pbt.Var) { open = append(open, v) },
		})
		closeTicket := func() pbt.CommandGen {
			return pbt.Map(pbt.SampledFrom(open), func(v pbt.Var) pbt.Command {
				return pbt.Command{
					Name: "close", Args: []any{v},
					Pre:  func() bool { return slices.Contains(open, v) },
					Run:  func(env pbt.Env) any { return q.Close(pbt.Resolve[int](env, v)) },
					Next: func(pbt.Var) { open = slices.DeleteFunc(open, func(o pbt.Var) bool { return o == v }) },
					Post: func(_ pbt.Env, got any) error {
						if got != nil {
							return fmt.Errorf("close failed: %v", got)
						}
						return nil
					},
				}
			})
		}
		count := pbt.Just(pbt.Command{
			Name: "open",
			Run:  func(pbt.Env) any { return q.Open() },
			Post: func(_ pbt.Env, got any) error {
				if got != len(open) {
					return fmt.Errorf("%d open tickets, want %d", got, len(open))
				}
				return nil
			},
		})
		commands := func() pbt.CommandGen {
			if len(open) == 0 {
				return pbt.OneOf(issue, count)
			}
			return pbt.Frequency(pbt.W(3, issue), pbt.W(2, closeTicket()), pbt.W(1, count))
		}
		pbt.Commands(func() { open = nil }, commands).Draw(t, "cmds").RunSequential(t)
	}
}

func TestQueueOK(t *testing.T) {
	pbt.Check(t, queueProperty(func() ticketQueue { return &Queue{open: map[int]bool{}} }), noSave...)
}
func TestQueueForgetDeleteMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, queueProperty(func() ticketQueue { return &forgetDeleteQueue{Queue{open: map[int]bool{}}} }), noSave...)
}

// ---- 表：同期の除去

type adder interface{ Add(int) int }

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

type splitCounter struct{ Counter } // 読みと書きの間でロックを手放す

func (c *splitCounter) Add(d int) int {
	c.mu.Lock()
	v := c.n
	c.mu.Unlock()
	runtime.Gosched()
	c.mu.Lock()
	defer c.mu.Unlock()
	c.n = v + d
	return c.n
}

type noLockCounter struct{ n int } // 同期の除去

func (c *noLockCounter) Add(d int) int {
	v := c.n
	runtime.Gosched()
	c.n = v + d
	return c.n
}

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
func TestCounterSplitMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, counterProperty(func() adder { return &splitCounter{} }), noSave...)
}
func TestCounterNoLockMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, counterProperty(func() adder { return &noLockCounter{} }), noSave...)
}
