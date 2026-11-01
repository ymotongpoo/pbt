// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package ch10

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"go.ymotongpoo.com/pbt"
	"go.ymotongpoo.com/pbt/examples/discovery/internal/mutant"
)

// Queue は番号を発行し、クローズするチケットキューである。
type Queue struct {
	next int
	open map[int]bool
}

func NewQueue() *Queue { return &Queue{open: map[int]bool{}} }

func (q *Queue) Issue() int {
	q.next++
	q.open[q.next] = true
	return q.next
}

var errNotOpen = errors.New("ticket is not open")

func (q *Queue) Close(id int) error {
	if !q.open[id] {
		return errNotOpen
	}
	delete(q.open, id)
	return nil
}

func (q *Queue) Open() int { return len(q.open) }

// ticketQueue はコマンドが対象に求める操作である。
type ticketQueue interface {
	Issue() int
	Close(id int) error
	Open() int
}

func queueProperty(newQueue func() ticketQueue) pbt.Property {
	return func(t *pbt.T) {
		q := newQueue()
		var open []pbt.Var // モデル：開いているチケットの Var の列

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
		init := func() { open = nil }

		prog := pbt.Commands(init, commands, pbt.ModelState(func() any { return open })).Draw(t, "cmds")
		prog.RunSequential(t)
	}
}

func TestQueue(t *testing.T) {
	pbt.Check(t, queueProperty(func() ticketQueue { return NewQueue() }), pbt.Verbose())
}

// ---- 変異

// dupQueue は2回に1回、前回と同じ番号を返す。
type dupQueue struct {
	Queue
	n int
}

func (q *dupQueue) Issue() int {
	q.n++
	if q.n%2 == 0 {
		return q.next
	}
	return q.Queue.Issue()
}

// leakyQueue はクローズしたチケットを取り除き忘れる。
type leakyQueue struct{ Queue }

func (q *leakyQueue) Close(id int) error {
	if !q.open[id] {
		return errNotOpen
	}
	q.open[id] = false
	return nil
}

// offByOneQueue は Open が1多い。
type offByOneQueue struct{ Queue }

func (q *offByOneQueue) Open() int { return len(q.open) + 1 }

// refusingQueue は Close が常に失敗する。
type refusingQueue struct{ Queue }

func (q *refusingQueue) Close(int) error { return errNotOpen }

// doubleCloseQueue は閉じたはずの番号をもう一度閉じられる。
type doubleCloseQueue struct {
	Queue
	closed map[int]bool
}

func (q *doubleCloseQueue) Close(id int) error {
	if q.closed[id] {
		return nil
	}
	if err := q.Queue.Close(id); err != nil {
		return err
	}
	q.closed[id] = true
	return nil
}

var noSave = []pbt.Option{pbt.NoSave(), pbt.NoReplay()}

func TestDupMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, queueProperty(func() ticketQueue { return &dupQueue{Queue: *NewQueue()} }), noSave...)
}
func TestLeakyMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, queueProperty(func() ticketQueue { return &leakyQueue{*NewQueue()} }), noSave...)
}
func TestOffByOneMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, queueProperty(func() ticketQueue { return &offByOneQueue{*NewQueue()} }), noSave...)
}
func TestRefusingMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, queueProperty(func() ticketQueue { return &refusingQueue{*NewQueue()} }), noSave...)
}

// ---- 実行順序の測定
func TestExecutionOrder(t *testing.T) {
	var events []string
	log := func(s string) { events = append(events, s) }
	pbt.Check(t, func(t *pbt.T) {
		events = nil
		n := 0
		a := func() pbt.CommandGen {
			log("gen")
			return pbt.Just(pbt.Command{
				Name: "a",
				Pre:  func() bool { log("Pre"); return true },
				Run:  func(pbt.Env) any { log("Run"); return nil },
				Next: func(pbt.Var) { log("Next"); n++ },
				Post: func(pbt.Env, any) error { log("Post"); return nil },
			})
		}
		init := func() { log("init"); n = 0 }
		prog := pbt.Commands(init, a, pbt.Length(2, 2)).Draw(t, "cmds")
		log("|")
		prog.RunSequential(t)
	}, pbt.Checks(1), pbt.NoSave(), pbt.NoReplay())
	got := strings.Join(events, " ")
	want := "init gen Pre Next gen Pre Next | init Pre Run Post Next Pre Run Post Next"
	if got != want {
		t.Fatalf("order\n got %s\nwant %s", got, want)
	}
}

// ---- 1本の列の実行：issue(); issue(); close(var1); open()
func TestTrace(t *testing.T) {
	var log []string
	pbt.Check(t, func(t *pbt.T) {
		log = nil
		q := NewQueue()
		var open []pbt.Var
		step := 0
		note := func(name string, got any) {
			log = append(log, fmt.Sprintf("%s got=%v before=%v", name, got, open))
		}
		issue := pbt.Just(pbt.Command{
			Name: "issue",
			Run:  func(pbt.Env) any { return q.Issue() },
			Post: func(_ pbt.Env, got any) error { note("issue", got); return nil },
			Next: func(v pbt.Var) { open = append(open, v); step++ },
		})
		closeFirst := func() pbt.CommandGen {
			v := open[0]
			return pbt.Just(pbt.Command{
				Name: "close", Args: []any{v},
				Run:  func(env pbt.Env) any { return q.Close(pbt.Resolve[int](env, v)) },
				Post: func(_ pbt.Env, got any) error { note("close", got); return nil },
				Next: func(pbt.Var) {
					open = slices.DeleteFunc(open, func(o pbt.Var) bool { return o == v })
					step++
				},
			})
		}
		count := pbt.Just(pbt.Command{
			Name: "open",
			Run:  func(pbt.Env) any { return q.Open() },
			Post: func(_ pbt.Env, got any) error { note("open", got); return nil },
			Next: func(pbt.Var) { step++ },
		})
		commands := func() pbt.CommandGen {
			switch step {
			case 0, 1:
				return issue
			case 2:
				return closeFirst()
			case 3:
				return count
			}
			return pbt.NoCommands()
		}
		init := func() { open, step = nil, 0 }
		pbt.Commands(init, commands, pbt.Length(4, 4)).Draw(t, "cmds").RunSequential(t)
		log = append(log, fmt.Sprintf("after=%v", open))
	}, pbt.Checks(1), pbt.NoSave(), pbt.NoReplay())
	want := []string{
		"issue got=1 before=[]",
		"issue got=2 before=[var1]",
		"close got=<nil> before=[var1 var2]",
		"open got=1 before=[var2]",
		"after=[var2]",
	}
	if !slices.Equal(log, want) {
		t.Fatalf("trace\n got %q\nwant %q", log, want)
	}
}

// ---- Pre の測定：状態から選ぶ生成器で、Pre は偽になるのか
var preFalse atomic.Int64

func queueNoPre(newQueue func() ticketQueue, withPre bool) pbt.Property {
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
				c := pbt.Command{
					Name: "close", Args: []any{v},
					Run: func(env pbt.Env) any { return q.Close(pbt.Resolve[int](env, v)) },
					Next: func(pbt.Var) {
						open = slices.DeleteFunc(open, func(o pbt.Var) bool { return o == v })
					},
					Post: func(_ pbt.Env, got any) error {
						if got != nil {
							return fmt.Errorf("close failed: %v", got)
						}
						return nil
					},
				}
				if withPre {
					c.Pre = func() bool {
						ok := slices.Contains(open, v)
						if !ok {
							preFalse.Add(1)
						}
						return ok
					}
				}
				return c
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

func TestPreCount(t *testing.T) {
	preFalse.Store(0)
	pbt.Check(t, queueNoPre(func() ticketQueue { return NewQueue() }, true), pbt.Checks(2000))
	if n := preFalse.Load(); n != 0 {
		t.Fatalf("Pre was false %d times", n)
	}
}
func TestPreCountShrinkMutant(t *testing.T) {
	mutant.Only(t)
	defer func() { t.Logf("pre false while shrinking: %d", preFalse.Load()) }()
	preFalse.Store(0)
	pbt.Check(t, queueNoPre(func() ticketQueue { return &leakyQueue{*NewQueue()} }, true), noSave...)
}
func TestNoPreShrinkMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, queueNoPre(func() ticketQueue { return &leakyQueue{*NewQueue()} }, false), noSave...)
}

// ---- 悪い例：モデルが実装のコピー
func copyModelProperty(newQueue func() ticketQueue) pbt.Property {
	return func(t *pbt.T) {
		q := newQueue()
		// 悪い例：モデルが対象の内部をそのまま持っている
		next := 0
		open := map[int]bool{}

		issue := pbt.Just(pbt.Command{
			Name: "issue",
			Run:  func(pbt.Env) any { return q.Issue() },
			// issue のモデル更新と検査
			Next: func(pbt.Var) {
				next++
				open[next] = true
			},
			Post: func(_ pbt.Env, got any) error {
				if got != next+1 {
					return fmt.Errorf("issue() = %v, want %d", got, next+1)
				}
				return nil
			},
		})
		closeTicket := func(ids []int) pbt.CommandGen {
			return pbt.Map(pbt.SampledFrom(ids), func(id int) pbt.Command {
				return pbt.Command{
					Name: "close", Args: []any{id},
					Pre: func() bool { return open[id] },
					Run: func(pbt.Env) any { return q.Close(id) },
					// close のモデル更新
					Next: func(pbt.Var) { open[id] = false }, // 対象と同じ消し忘れ
					Post: func(_ pbt.Env, got any) error {
						if got != nil {
							return fmt.Errorf("close(%d) failed: %v", id, got)
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
			var ids []int
			for _, id := range slices.Sorted(maps.Keys(open)) {
				if open[id] {
					ids = append(ids, id)
				}
			}
			if len(ids) == 0 {
				return pbt.OneOf(issue, count)
			}
			return pbt.Frequency(pbt.W(3, issue), pbt.W(2, closeTicket(ids)), pbt.W(1, count))
		}
		init := func() { next, open = 0, map[int]bool{} }
		pbt.Commands(init, commands).Draw(t, "cmds").RunSequential(t)
	}
}

// 消し忘れのバグをモデルも共有すると、テストは通ってしまう。
func TestCopyModelSurvives(t *testing.T) {
	pbt.Check(t, copyModelProperty(func() ticketQueue { return &leakyQueue{*NewQueue()} }), noSave...)
}

// spacedQueue は番号の振り方だけが違う、正しい実装である。
type spacedQueue struct{ Queue }

func (q *spacedQueue) Issue() int {
	q.next += 10
	q.open[q.next] = true
	return q.next
}

// コピーのモデルは、正しいリファクタリングでも落ちる。
func TestCopyModelRefactorMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, copyModelProperty(func() ticketQueue { return &spacedQueue{*NewQueue()} }), noSave...)
}
func TestVarModelRefactor(t *testing.T) {
	pbt.Check(t, queueProperty(func() ticketQueue { return &spacedQueue{*NewQueue()} }), noSave...)
}

// ---- 悪い例：個数だけのモデル
func countModelProperty(newQueue func() ticketQueue) pbt.Property {
	return func(t *pbt.T) {
		q := newQueue()
		// 悪い例：モデルは開いている個数だけ
		n := 0 // モデル：開いている個数だけ

		issue := pbt.Just(pbt.Command{
			Name: "issue",
			Run:  func(pbt.Env) any { return q.Issue() },
			Next: func(pbt.Var) { n++ },
		})
		closeTicket := pbt.Map(pbt.IntRange(1, 5), func(id int) pbt.Command {
			return pbt.Command{
				Name: "close", Args: []any{id},
				Run:  func(pbt.Env) any { return q.Close(id) },
				Next: func(pbt.Var) { n-- }, // id が開いていたかどうかは分からない
				// Post：got が nil であるべきかどうかも分からない
			}
		})
		count := pbt.Just(pbt.Command{
			Name: "open",
			Run:  func(pbt.Env) any { return q.Open() },
			Post: func(_ pbt.Env, got any) error {
				if got != n {
					return fmt.Errorf("%d open tickets, want %d", got, n)
				}
				return nil
			},
		})
		commands := func() pbt.CommandGen { return pbt.OneOf(issue, closeTicket, count) }
		pbt.Commands(func() { n = 0 }, commands).Draw(t, "cmds").RunSequential(t)
	}
}

// n-- を外しても、正しい Queue で落ちる。
func countModelKeepProperty(newQueue func() ticketQueue) pbt.Property {
	return func(t *pbt.T) {
		q := newQueue()
		n := 0
		issue := pbt.Just(pbt.Command{
			Name: "issue",
			Run:  func(pbt.Env) any { return q.Issue() },
			Next: func(pbt.Var) { n++ },
		})
		closeTicket := pbt.Map(pbt.IntRange(1, 5), func(id int) pbt.Command {
			return pbt.Command{
				Name: "close", Args: []any{id},
				Run: func(pbt.Env) any { return q.Close(id) },
			}
		})
		count := pbt.Just(pbt.Command{
			Name: "open",
			Run:  func(pbt.Env) any { return q.Open() },
			Post: func(_ pbt.Env, got any) error {
				if got != n {
					return fmt.Errorf("%d open tickets, want %d", got, n)
				}
				return nil
			},
		})
		commands := func() pbt.CommandGen { return pbt.OneOf(issue, closeTicket, count) }
		pbt.Commands(func() { n = 0 }, commands).Draw(t, "cmds").RunSequential(t)
	}
}

// 正しい Queue でも、個数のモデルは落ちる。
func TestCountModelDecrementMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, countModelProperty(func() ticketQueue { return NewQueue() }), noSave...)
}
func TestCountModelKeepMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, countModelKeepProperty(func() ticketQueue { return NewQueue() }), noSave...)
}

// issue と open だけの最小の形
func minimalProperty(newQueue func() ticketQueue) pbt.Property {
	return func(t *pbt.T) {
		q := newQueue()
		n := 0

		issue := pbt.Just(pbt.Command{
			Name: "issue",
			Run:  func(pbt.Env) any { return q.Issue() },
			Next: func(pbt.Var) { n++ },
		})
		count := pbt.Just(pbt.Command{
			Name: "open",
			Run:  func(pbt.Env) any { return q.Open() },
			Post: func(_ pbt.Env, got any) error {
				if got != n {
					return fmt.Errorf("%d open tickets, want %d", got, n)
				}
				return nil
			},
		})
		commands := func() pbt.CommandGen { return pbt.OneOf(issue, count) }
		pbt.Commands(func() { n = 0 }, commands).Draw(t, "cmds").RunSequential(t)
	}
}
func TestMinimal(t *testing.T) {
	pbt.Check(t, minimalProperty(func() ticketQueue { return NewQueue() }), noSave...)
}
func TestMinimalDupMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, minimalProperty(func() ticketQueue { return &dupQueue{Queue: *NewQueue()} }), noSave...)
}
func TestMinimalLeakySurvives(t *testing.T) {
	pbt.Check(t, minimalProperty(func() ticketQueue { return &leakyQueue{*NewQueue()} }), noSave...)
}

// reuseQueue は閉じた番号を次の発行で再利用する。
type reuseQueue struct {
	Queue
	free []int
}

func (q *reuseQueue) Close(id int) error {
	err := q.Queue.Close(id)
	if err == nil {
		q.free = append(q.free, id)
	}
	return err
}

func (q *reuseQueue) Issue() int {
	if len(q.free) > 0 {
		id := q.free[0]
		q.free = q.free[1:]
		q.open[id] = true
		return id
	}
	return q.Queue.Issue()
}

// 本章のプロパティは開いている番号しか閉じないので、次の2つの違反を検査しない。
func TestDoubleCloseSurvives(t *testing.T) {
	pbt.Check(t, queueProperty(func() ticketQueue {
		return &doubleCloseQueue{Queue: *NewQueue(), closed: map[int]bool{}}
	}), pbt.Checks(2000), pbt.NoSave(), pbt.NoReplay())
}

func TestReuseClosedSurvives(t *testing.T) {
	pbt.Check(t, queueProperty(func() ticketQueue {
		return &reuseQueue{Queue: *NewQueue()}
	}), pbt.Checks(2000), pbt.NoSave(), pbt.NoReplay())
}
