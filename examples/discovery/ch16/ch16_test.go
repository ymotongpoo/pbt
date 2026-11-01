// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package ch16

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sync"
	"testing"

	"go.ymotongpoo.com/pbt"
	"go.ymotongpoo.com/pbt/examples/discovery/internal/mutant"
)

// requireFlaky は、失敗するかどうかが実行によって変わるテストを、
// PBT_FLAKY=1 を設定したときだけ動かす。頻度は第16章の表のとおり。
func requireFlaky(t *testing.T) {
	t.Helper()
	if os.Getenv("PBT_FLAKY") == "" {
		t.Skip("set PBT_FLAKY=1 to run this schedule-dependent test")
	}
}

// ---- カウンター

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

type adder interface{ Add(d int) int }

func counterProperty(newCounter func() adder) pbt.Property {
	return func(t *pbt.T) {
		c := newCounter()
		n := 0 // モデル：カウンターが保持しているはずの値
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
		prog := pbt.ParallelCommands(func() { n = 0 }, commands).Draw(t, "cmds")
		prog.RunParallel(t)
	}
}

func TestCounterParallel(t *testing.T) {
	pbt.Check(t, counterProperty(func() adder { return &Counter{} }))
}

// LostCounter はロックを2回に分けて取る。どのアクセスも同期されているので
// データ競合はないが、読み出しと書き込みの間に他のゴルーチンが割り込める。
type LostCounter struct {
	mu sync.Mutex
	n  int
}

func (c *LostCounter) Add(d int) int {
	c.mu.Lock()
	v := c.n
	c.mu.Unlock()

	runtime.Gosched() // 実際のコードなら、ここに別の処理が入る

	c.mu.Lock()
	defer c.mu.Unlock()
	c.n = v + d
	return c.n
}

func TestLostCounterMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, counterProperty(func() adder { return &LostCounter{} }), pbt.NoSave(), pbt.NoReplay())
}

// NarrowCounter は LostCounter から、間の処理を取り除いたものである。
type NarrowCounter struct {
	mu sync.Mutex
	n  int
}

func (c *NarrowCounter) Add(d int) int {
	c.mu.Lock()
	v := c.n
	c.mu.Unlock()

	c.mu.Lock()
	defer c.mu.Unlock()
	c.n = v + d
	return c.n
}

func TestNarrowCounterFlaky(t *testing.T) {
	requireFlaky(t)
	pbt.Check(t, counterProperty(func() adder { return &NarrowCounter{} }), pbt.NoSave(), pbt.NoReplay())
}

// counterShortPrefix は、前段を空にした counterProperty である。
func counterShortPrefix(newCounter func() adder) pbt.Property {
	return func(t *pbt.T) {
		c := newCounter()
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
		prog := pbt.ParallelCommands(func() { n = 0 }, commands, pbt.Length(0, 0)).Draw(t, "cmds")
		prog.RunParallel(t)
	}
}

func TestNarrowShortPrefixFlaky(t *testing.T) {
	requireFlaky(t)
	pbt.Check(t, counterShortPrefix(func() adder { return &NarrowCounter{} }), pbt.NoSave(), pbt.NoReplay())
}

func TestNarrowCounterSaveFlaky(t *testing.T) {
	requireFlaky(t)
	dir := filepath.Join(os.TempDir(), "pbt-ch16-narrow")
	pbt.Check(t, counterProperty(func() adder { return &NarrowCounter{} }), pbt.TestdataDir(dir))
}

// StatCounter の結果は正しいが、呼び出し回数の更新が同期されていない。
type StatCounter struct {
	Counter
	calls int
}

func (c *StatCounter) Add(d int) int {
	c.calls++
	return c.Counter.Add(d)
}

// BareCounter はロックを持たない。
type BareCounter struct{ n int }

func (c *BareCounter) Add(d int) int {
	c.n += d
	return c.n
}

func TestBareCounterFlaky(t *testing.T) {
	requireFlaky(t)
	pbt.Check(t, counterProperty(func() adder { return &BareCounter{} }), pbt.NoSave(), pbt.NoReplay())
}

// 実物の状態を読む Post
func realStateCommand(c *Counter, n *int) pbt.CommandGen {
	return pbt.Map(pbt.IntRange(1, 5), func(d int) pbt.Command {
		return pbt.Command{
			Name: "add", Args: []any{d},
			Run:  func(pbt.Env) any { return c.Add(d) },
			Next: func(pbt.Var) { *n += d },
			Post: func(pbt.Env, any) error {
				if got := c.Value(); got != *n+d {
					return fmt.Errorf("counter holds %d, want %d", got, *n+d)
				}
				return nil
			},
		}
	})
}

func TestRealStatePostSequential(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		c, n := &Counter{}, 0
		add := realStateCommand(c, &n)
		commands := func() pbt.CommandGen { return add }
		prog := pbt.Commands(func() { n = 0 }, commands).Draw(t, "cmds")
		prog.RunSequential(t)
	})
}

func TestRealStatePostParallelMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, func(t *pbt.T) {
		c, n := &Counter{}, 0
		add := realStateCommand(c, &n)
		commands := func() pbt.CommandGen { return add }
		prog := pbt.ParallelCommands(func() { n = 0 }, commands).Draw(t, "cmds")
		prog.RunParallel(t)
	}, pbt.NoSave(), pbt.NoReplay())
}

// ---- 番号の発行

type tickets interface {
	Issue() int
	Close(id int) error
	IDs() []int
}

var errNotOpen = errors.New("ticket is not open")

// Tickets は開いている番号の列を持つ。
type Tickets struct {
	mu   sync.Mutex
	next int
	ids  []int
}

func (q *Tickets) Issue() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.next++
	q.ids = append(q.ids, q.next)
	return q.next
}

func (q *Tickets) Close(id int) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	i := slices.Index(q.ids, id)
	if i < 0 {
		return errNotOpen
	}
	q.ids = slices.Delete(q.ids, i, i+1)
	return nil
}

func (q *Tickets) IDs() []int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return slices.Clone(q.ids)
}

// DupTickets は次の番号の読み出しと更新を分けている。
type DupTickets struct{ Tickets }

func (q *DupTickets) Issue() int {
	q.mu.Lock()
	id := q.next + 1
	q.mu.Unlock()

	runtime.Gosched() // 実際のコードなら、ここに別の処理が入る

	q.mu.Lock()
	defer q.mu.Unlock()
	q.next = id
	q.ids = append(q.ids, id)
	return id
}

// issueProperty は発行だけを並行に実行する。
func issueProperty(newTickets func() tickets) pbt.Property {
	return func(t *pbt.T) {
		q := newTickets()
		var issued []pbt.Var // モデル：発行された番号の Var
		issue := pbt.Just(pbt.Command{
			Name: "issue",
			Run:  func(pbt.Env) any { return q.Issue() },
			Next: func(v pbt.Var) { issued = append(issued, v) },
			Post: func(env pbt.Env, got any) error {
				for _, v := range issued {
					if pbt.Resolve[int](env, v) == got {
						return fmt.Errorf("number %v was already issued", got)
					}
				}
				return nil
			},
		})
		commands := func() pbt.CommandGen { return issue }
		prog := pbt.ParallelCommands(func() { issued = nil }, commands).Draw(t, "cmds")
		prog.RunParallel(t)

		// 全体不変条件：枝がすべて終わった対象に直接問い合わせる。
		want := len(prog.Prefix())
		for _, br := range prog.Branches() {
			want += len(br)
		}
		ids := q.IDs()
		slices.Sort(ids)
		if got := len(slices.Compact(ids)); got != want {
			t.Fatalf("%d distinct numbers, want %d", got, want)
		}
	}
}

// issueFinalOnly は、issue の Post を外して終了後の照合だけを残したものである。
func issueFinalOnly(newTickets func() tickets) pbt.Property {
	return func(t *pbt.T) {
		q := newTickets()
		issue := pbt.Just(pbt.Command{
			Name: "issue",
			Run:  func(pbt.Env) any { return q.Issue() },
		})
		commands := func() pbt.CommandGen { return issue }
		prog := pbt.ParallelCommands(func() {}, commands).Draw(t, "cmds")
		prog.RunParallel(t)

		want := len(prog.Prefix())
		for _, br := range prog.Branches() {
			want += len(br)
		}
		ids := q.IDs()
		slices.Sort(ids)
		if got := len(slices.Compact(ids)); got != want {
			t.Fatalf("%d distinct numbers, want %d", got, want)
		}
	}
}

func TestIssueParallel(t *testing.T) {
	pbt.Check(t, issueProperty(func() tickets { return &Tickets{} }))
}

func TestDupTicketsSequentialSurvives(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		q := &DupTickets{}
		var issued []pbt.Var
		issue := pbt.Just(pbt.Command{
			Name: "issue",
			Run:  func(pbt.Env) any { return q.Issue() },
			Next: func(v pbt.Var) { issued = append(issued, v) },
			Post: func(env pbt.Env, got any) error {
				for _, v := range issued {
					if pbt.Resolve[int](env, v) == got {
						return fmt.Errorf("number %v was already issued", got)
					}
				}
				return nil
			},
		})
		commands := func() pbt.CommandGen { return issue }
		prog := pbt.Commands(func() { issued = nil }, commands).Draw(t, "cmds")
		prog.RunSequential(t)
	}, pbt.NoSave(), pbt.NoReplay())
}

func TestDupTicketsPostMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, issueProperty(func() tickets { return &DupTickets{} }), pbt.NoSave(), pbt.NoReplay())
}

func TestDupTicketsFinalOnlyMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, issueFinalOnly(func() tickets { return &DupTickets{} }), pbt.NoSave(), pbt.NoReplay())
}

// ---- 事前条件が並行の検査範囲を決める

// DoubleClose は存在の確認と削除を分けている。
type DoubleClose struct{ Tickets }

func (q *DoubleClose) Close(id int) error {
	q.mu.Lock()
	i := slices.Index(q.ids, id)
	q.mu.Unlock()
	if i < 0 {
		return errNotOpen
	}

	runtime.Gosched() // 実際のコードなら、ここに別の処理が入る

	q.mu.Lock()
	defer q.mu.Unlock()
	q.ids = slices.DeleteFunc(q.ids, func(x int) bool { return x == id })
	return nil
}

// closeProperty は、枝でクローズを競わせる。Pre を持たない。
func closeProperty(newTickets func() tickets) pbt.Property {
	return func(t *pbt.T) {
		q := newTickets()
		var open []pbt.Var // モデル：開いているチケットの Var
		issue := pbt.Just(pbt.Command{
			Name: "issue",
			Run:  func(pbt.Env) any { return q.Issue() },
			Next: func(v pbt.Var) { open = append(open, v) },
		})
		closeTicket := func() pbt.CommandGen {
			return pbt.Map(pbt.SampledFrom(open), func(v pbt.Var) pbt.Command {
				return pbt.Command{
					Name: "close", Args: []any{v},
					Run: func(env pbt.Env) any { return q.Close(pbt.Resolve[int](env, v)) },
					Next: func(pbt.Var) {
						open = slices.DeleteFunc(open, func(o pbt.Var) bool { return o == v })
					},
					Post: func(_ pbt.Env, got any) error {
						if slices.Contains(open, v) != (got == nil) {
							return fmt.Errorf("close returned %v, open=%v", got, slices.Contains(open, v))
						}
						return nil
					},
				}
			})
		}
		commands := func() pbt.CommandGen {
			if len(open) == 0 {
				return issue
			}
			return pbt.Frequency(pbt.W(1, issue), pbt.W(2, closeTicket()))
		}
		prog := pbt.ParallelCommands(func() { open = nil }, commands).Draw(t, "cmds")
		prog.RunParallel(t)
	}
}

// closeStrictProperty は、closeTicket に第10章と同じ Pre を付けたものである。
func closeStrictProperty(newTickets func() tickets) pbt.Property {
	return func(t *pbt.T) {
		q := newTickets()
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
					Pre: func() bool { return slices.Contains(open, v) },
					Run: func(env pbt.Env) any { return q.Close(pbt.Resolve[int](env, v)) },
					Next: func(pbt.Var) {
						open = slices.DeleteFunc(open, func(o pbt.Var) bool { return o == v })
					},
					Post: func(_ pbt.Env, got any) error {
						if slices.Contains(open, v) != (got == nil) {
							return fmt.Errorf("close returned %v, open=%v", got, slices.Contains(open, v))
						}
						return nil
					},
				}
			})
		}
		commands := func() pbt.CommandGen {
			if len(open) == 0 {
				return issue
			}
			return pbt.Frequency(pbt.W(1, issue), pbt.W(2, closeTicket()))
		}
		prog := pbt.ParallelCommands(func() { open = nil }, commands).Draw(t, "cmds")
		prog.RunParallel(t)
	}
}

func TestCloseParallel(t *testing.T) {
	pbt.Check(t, closeProperty(func() tickets { return &Tickets{} }))
}

func TestDoubleCloseStrictPreSurvives(t *testing.T) {
	pbt.Check(t, closeStrictProperty(func() tickets { return &DoubleClose{} }), pbt.Checks(2000), pbt.NoSave(), pbt.NoReplay())
}

func TestDoubleCloseLoosePreMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, closeProperty(func() tickets { return &DoubleClose{} }), pbt.NoSave(), pbt.NoReplay())
}

// ---- 実時間の順序

// goid は実行中のゴルーチンの番号を返す。ここではゴルーチンを
// 1つのクライアントに見立てるためだけに使う。
func goid() string {
	b := make([]byte, 32)
	b = b[:runtime.Stack(b, false)]
	return string(bytes.Fields(b)[1])
}

// StickyRegister は、クライアントごとに読み取り用のレプリカを持つ。
// 書き込みは主系と自分のレプリカにしか届かず、他のレプリカへは伝わらない。
type StickyRegister struct {
	mu       sync.Mutex
	primary  int
	replicas map[string]int
}

func NewStickyRegister() *StickyRegister {
	return &StickyRegister{replicas: map[string]int{}}
}

func (r *StickyRegister) replica() string {
	id := goid()
	if _, ok := r.replicas[id]; !ok {
		r.replicas[id] = r.primary
	}
	return id
}

func (r *StickyRegister) Put(v int) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.primary = v
	r.replicas[r.replica()] = v
	return v
}

func (r *StickyRegister) Get() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.replicas[r.replica()]
}

func TestStickyRegisterParallelSurvives(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		r := NewStickyRegister()
		v := 0 // モデル：最後に書かれた値
		put := pbt.Map(pbt.IntRange(1, 9), func(x int) pbt.Command {
			return pbt.Command{
				Name: "put", Args: []any{x},
				Run:  func(pbt.Env) any { return r.Put(x) },
				Next: func(pbt.Var) { v = x },
			}
		})
		get := pbt.Just(pbt.Command{
			Name: "get",
			Run:  func(pbt.Env) any { return r.Get() },
			Post: func(_ pbt.Env, got any) error {
				if got != v {
					return fmt.Errorf("get returned %v, want %d", got, v)
				}
				return nil
			},
		})
		commands := func() pbt.CommandGen { return pbt.OneOf(put, get) }
		prog := pbt.ParallelCommands(func() { v = 0 }, commands).Draw(t, "cmds")
		prog.RunParallel(t)
	}, pbt.Checks(5000), pbt.NoSave(), pbt.NoReplay())
}

// 書き込みの完了を待ってから読んでも、古い値が返る。
func TestStickyRegisterStaleReadExample(t *testing.T) {
	r := NewStickyRegister()
	first, written, second := make(chan int), make(chan struct{}), make(chan int)
	go func() {
		first <- r.Get() // 読み取り側のレプリカを作る
		<-written
		second <- r.Get()
	}()
	if got := <-first; got != 0 {
		t.Fatalf("first read = %d", got)
	}
	r.Put(7) // 別のゴルーチンの書き込みが完了する
	close(written)
	if got := <-second; got != 0 {
		t.Fatalf("second read = %d, the replica is expected to be stale", got)
	}
}

// 更新が値を返さなくても、後続の照会で効果を観測できれば検査になる。
func voidProperty(ignore bool) pbt.Property {
	return func(t *pbt.T) {
		c := &Counter{}
		n := 0
		add := pbt.Just(pbt.Command{
			Name: "add_void",
			Run: func(pbt.Env) any {
				if !ignore {
					c.Add(1)
				}
				return nil
			},
			Next: func(pbt.Var) { n++ },
		})
		get := pbt.Just(pbt.Command{
			Name: "get",
			Run:  func(pbt.Env) any { return c.Value() },
			Post: func(_ pbt.Env, got any) error {
				if got != n {
					return fmt.Errorf("got %v, want %d", got, n)
				}
				return nil
			},
		})
		commands := func() pbt.CommandGen { return pbt.OneOf(add, get) }
		prog := pbt.ParallelCommands(func() { n = 0 }, commands).Draw(t, "cmds")
		prog.RunParallel(t)
	}
}

func TestVoidUpdate(t *testing.T) {
	pbt.Check(t, voidProperty(false), pbt.Checks(2000), pbt.NoSave(), pbt.NoReplay())
}

func TestVoidUpdateIgnoredMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, voidProperty(true), pbt.NoSave(), pbt.NoReplay())
}
