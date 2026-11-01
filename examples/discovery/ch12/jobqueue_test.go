// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package ch12

import (
	"fmt"
	"os"
	"slices"
	"testing"

	"go.ymotongpoo.com/pbt"
	"go.ymotongpoo.com/pbt/examples/discovery/internal/mutant"
)

// JobQueue は先入れ先出しのジョブキューである。
type JobQueue struct {
	next int
	jobs []int
}

func (q *JobQueue) Enqueue() int {
	q.next++
	q.jobs = append(q.jobs, q.next)
	return q.next
}

func (q *JobQueue) Dequeue() int {
	j := q.jobs[0]
	q.jobs = q.jobs[1:]
	return j
}

func (q *JobQueue) Len() int { return len(q.jobs) }

func (q *JobQueue) IDs() []int { return slices.Clone(q.jobs) }

type jobQueue interface {
	Enqueue() int
	Dequeue() int
	Len() int
	IDs() []int
}

// jobQueueProperty は監査コマンドを列に混ぜる書き方である。
func jobQueueProperty(newQueue func() jobQueue) pbt.Property {
	return func(t *pbt.T) {
		q := newQueue()
		var pending []pbt.Var // モデル：入っている仕事の Var の列（古い順）

		enqueue := pbt.Just(pbt.Command{
			Name: "enqueue",
			Run:  func(pbt.Env) any { return q.Enqueue() },
			Next: func(v pbt.Var) { pending = append(pending, v) },
		})
		dequeue := func() pbt.CommandGen {
			want := pending[0]
			return pbt.Just(pbt.Command{
				Name: "dequeue", Args: []any{want},
				Pre:  func() bool { return len(pending) > 0 && pending[0] == want },
				Run:  func(pbt.Env) any { return q.Dequeue() },
				Next: func(pbt.Var) { pending = pending[1:] },
				Post: func(env pbt.Env, got any) error {
					if w := pbt.Resolve[int](env, want); got != w {
						return fmt.Errorf("dequeue() = %v, want %d", got, w)
					}
					return nil
				},
			})
		}
		length := pbt.Just(pbt.Command{
			Name: "len",
			Run:  func(pbt.Env) any { return q.Len() },
			Post: func(_ pbt.Env, got any) error {
				if got != len(pending) {
					return fmt.Errorf("len() = %v, model says %d", got, len(pending))
				}
				return nil
			},
		})
		// audit は全体不変条件をコマンドにしたもの：列のどの時点でも、
		// システムの全体観測はモデルと一致しなければならない。
		audit := pbt.Just(pbt.Command{
			Name: "audit",
			Run:  func(pbt.Env) any { return q.IDs() },
			Post: func(env pbt.Env, got any) error {
				ids := got.([]int)
				if len(ids) != len(pending) {
					return fmt.Errorf("system holds %d jobs, model holds %d", len(ids), len(pending))
				}
				for i, v := range pending {
					if want := pbt.Resolve[int](env, v); ids[i] != want {
						return fmt.Errorf("job %d in system is %d, model says %d", i, ids[i], want)
					}
				}
				return nil
			},
		})
		commands := func() pbt.CommandGen {
			if len(pending) == 0 {
				return pbt.Frequency(pbt.W(4, enqueue), pbt.W(1, length), pbt.W(1, audit))
			}
			return pbt.Frequency(pbt.W(4, enqueue), pbt.W(3, dequeue()), pbt.W(1, length), pbt.W(2, audit))
		}
		init := func() { pending = nil }

		prog := pbt.Commands(init, commands, pbt.ModelState(func() any { return pending })).Draw(t, "cmds")
		prog.RunSequential(t)
	}
}

func TestJobQueue(t *testing.T) {
	pbt.Check(t, jobQueueProperty(func() jobQueue { return &JobQueue{} }))
}

// weightedProperty は監査の重みを変え、統計と列末の検査を足した測定用の版である。
func weightedProperty(newQueue func() jobQueue, auditWeight, emptyAuditWeight int) pbt.Property {
	return func(t *pbt.T) {
		q := newQueue()
		var pending []pbt.Var

		enqueue := pbt.Just(pbt.Command{
			Name: "enqueue",
			Run:  func(pbt.Env) any { return q.Enqueue() },
			Next: func(v pbt.Var) { pending = append(pending, v) },
		})
		dequeue := func() pbt.CommandGen {
			want := pending[0]
			return pbt.Just(pbt.Command{
				Name: "dequeue", Args: []any{want},
				Pre:  func() bool { return len(pending) > 0 && pending[0] == want },
				Run:  func(pbt.Env) any { return q.Dequeue() },
				Next: func(pbt.Var) { pending = pending[1:] },
				Post: func(env pbt.Env, got any) error {
					if w := pbt.Resolve[int](env, want); got != w {
						return fmt.Errorf("dequeue() = %v, want %d", got, w)
					}
					return nil
				},
			})
		}
		length := pbt.Just(pbt.Command{
			Name: "len",
			Run:  func(pbt.Env) any { return q.Len() },
			Post: func(_ pbt.Env, got any) error {
				if got != len(pending) {
					return fmt.Errorf("len() = %v, model says %d", got, len(pending))
				}
				return nil
			},
		})
		audit := pbt.Just(pbt.Command{
			Name: "audit",
			Run:  func(pbt.Env) any { return q.IDs() },
			Post: func(env pbt.Env, got any) error {
				ids := got.([]int)
				if len(ids) != len(pending) {
					return fmt.Errorf("system holds %d jobs, model holds %d", len(ids), len(pending))
				}
				for i, v := range pending {
					if want := pbt.Resolve[int](env, v); ids[i] != want {
						return fmt.Errorf("job %d in system is %d, model says %d", i, ids[i], want)
					}
				}
				return nil
			},
		})
		commands := func() pbt.CommandGen {
			alts := []pbt.Weighted[pbt.Command]{pbt.W(4, enqueue), pbt.W(1, length)}
			if len(pending) == 0 {
				if emptyAuditWeight > 0 {
					alts = append(alts, pbt.W(emptyAuditWeight, audit))
				}
				return pbt.Frequency(alts...)
			}
			if auditWeight > 0 {
				alts = append(alts, pbt.W(auditWeight, audit))
			}
			return pbt.Frequency(append(alts, pbt.W(3, dequeue()))...)
		}
		init := func() { pending = nil }

		prog := pbt.Commands(init, commands, pbt.ModelState(func() any { return pending })).Draw(t, "cmds")
		hasAudit := slices.ContainsFunc(prog.Steps(), func(s pbt.Step) bool {
			return s.Command.Name == "audit"
		})
		t.Classify(hasAudit, "列に audit がある")
		t.Measure("commands", float64(prog.Len()))
		prog.RunSequential(t)

		// 列の最後に必ず1回、Var を使わない検査を置く
		if got := q.Len(); got != len(pending) {
			t.Fatalf("after the sequence: len() = %d, model says %d", got, len(pending))
		}
	}
}

func TestStats(t *testing.T) {
	pbt.Check(t, weightedProperty(func() jobQueue { return &JobQueue{} }, 2, 1), pbt.Verbose(), pbt.Seed(1))
}

// everyPostProperty は共通の検査を各コマンドの Post から呼ぶ書き方である。
func everyPostProperty(newQueue func() jobQueue) pbt.Property {
	return func(t *pbt.T) {
		q := newQueue()
		var pending []pbt.Var

		// resolved は Var の列を実際の ID の列に直す。
		resolved := func(env pbt.Env, vs []pbt.Var) []int {
			ids := make([]int, len(vs))
			for i, v := range vs {
				ids[i] = pbt.Resolve[int](env, v)
			}
			return ids
		}
		// holds は対象の全体観測が want と一致することを確かめる。
		holds := func(want []int) error {
			if ids := q.IDs(); !slices.Equal(ids, want) {
				return fmt.Errorf("system holds %v, model says %v", ids, want)
			}
			return nil
		}

		enqueue := pbt.Just(pbt.Command{
			Name: "enqueue",
			Run:  func(pbt.Env) any { return q.Enqueue() },
			Next: func(v pbt.Var) { pending = append(pending, v) },
			Post: func(env pbt.Env, got any) error {
				// Post は Next より先に動くので、更新後の列を手で作る
				return holds(append(resolved(env, pending), got.(int)))
			},
		})
		dequeue := func() pbt.CommandGen {
			want := pending[0]
			return pbt.Just(pbt.Command{
				Name: "dequeue", Args: []any{want},
				Pre:  func() bool { return len(pending) > 0 && pending[0] == want },
				Run:  func(pbt.Env) any { return q.Dequeue() },
				Next: func(pbt.Var) { pending = pending[1:] },
				Post: func(env pbt.Env, got any) error {
					if w := pbt.Resolve[int](env, want); got != w {
						return fmt.Errorf("dequeue() = %v, want %d", got, w)
					}
					return holds(resolved(env, pending[1:]))
				},
			})
		}
		length := pbt.Just(pbt.Command{
			Name: "len",
			Run:  func(pbt.Env) any { return q.Len() },
			Post: func(env pbt.Env, got any) error {
				if got != len(pending) {
					return fmt.Errorf("len() = %v, model says %d", got, len(pending))
				}
				return holds(resolved(env, pending))
			},
		})
		commands := func() pbt.CommandGen {
			if len(pending) == 0 {
				return pbt.Frequency(pbt.W(4, enqueue), pbt.W(1, length))
			}
			return pbt.Frequency(pbt.W(4, enqueue), pbt.W(3, dequeue()), pbt.W(1, length))
		}
		pbt.Commands(func() { pending = nil }, commands).Draw(t, "cmds").RunSequential(t)
	}
}

func TestEveryPost(t *testing.T) {
	pbt.Check(t, everyPostProperty(func() jobQueue { return &JobQueue{} }))
}

var noSave = []pbt.Option{pbt.NoSave(), pbt.NoReplay()}

// ---- 潜伏する変異

// swapQueue は3件目以降の投入で末尾の2件を入れ替える。件数は正しい。
type swapQueue struct{ JobQueue }

func (q *swapQueue) Enqueue() int {
	id := q.JobQueue.Enqueue()
	if n := len(q.jobs); n >= 3 {
		q.jobs[n-1], q.jobs[n-2] = q.jobs[n-2], q.jobs[n-1]
	}
	return id
}

// deepSwapQueue は12件目以降の投入で末尾の2件を入れ替える。
// 取り出しの照合で気づくには、先頭から11件を取り出す必要がある。
type deepSwapQueue struct{ JobQueue }

func (q *deepSwapQueue) Enqueue() int {
	id := q.JobQueue.Enqueue()
	if n := len(q.jobs); n >= 12 {
		q.jobs[n-1], q.jobs[n-2] = q.jobs[n-2], q.jobs[n-1]
	}
	return id
}

// ghostQueue は取り出した仕事を末尾に残す。件数は別に数えているので正しい。
type ghostQueue struct {
	JobQueue
	n int
}

func (q *ghostQueue) Enqueue() int {
	q.n++
	return q.JobQueue.Enqueue()
}

func (q *ghostQueue) Dequeue() int {
	j := q.jobs[0]
	q.jobs = append(q.jobs[1:], j)
	q.n--
	return j
}

func (q *ghostQueue) Len() int { return q.n }

// lazyQueue は投入を buf に溜め、観測のときに jobs へ移す。
// Dequeue が buf を見忘れている。
type lazyQueue struct {
	next      int
	jobs, buf []int
}

func (q *lazyQueue) Enqueue() int {
	q.next++
	q.buf = append(q.buf, q.next)
	return q.next
}

func (q *lazyQueue) flush() {
	q.jobs = append(q.jobs, q.buf...)
	q.buf = nil
}

func (q *lazyQueue) Dequeue() int {
	j := q.jobs[0] // jobs が空なら、ここでパニックする
	q.jobs = q.jobs[1:]
	return j
}

func (q *lazyQueue) Len() int {
	q.flush()
	return len(q.jobs)
}

func (q *lazyQueue) IDs() []int {
	q.flush()
	return slices.Clone(q.jobs)
}

func TestAuditSwapMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, jobQueueProperty(func() jobQueue { return &swapQueue{} }), noSave...)
}
func TestEveryPostSwapMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, everyPostProperty(func() jobQueue { return &swapQueue{} }), noSave...)
}
func TestAuditGhostMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, jobQueueProperty(func() jobQueue { return &ghostQueue{} }), noSave...)
}
func TestEveryPostGhostMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, everyPostProperty(func() jobQueue { return &ghostQueue{} }), noSave...)
}
func TestAuditLazyMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, jobQueueProperty(func() jobQueue { return &lazyQueue{} }), noSave...)
}

// 毎回観測すると、観測のない列が生成されないので lazyQueue のバグは隠れる。
func TestEveryPostLazySurvives(t *testing.T) {
	pbt.Check(t, everyPostProperty(func() jobQueue { return &lazyQueue{} }), append(noSave, pbt.Checks(2000))...)
}

// 発見までのケース数の測定用
func TestWeight0Mutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, weightedProperty(func() jobQueue { return &deepSwapQueue{} }, 0, 0), pbt.NoSave(), pbt.NoReplay(), pbt.Checks(100000))
}
func TestWeight1Mutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, weightedProperty(func() jobQueue { return &deepSwapQueue{} }, 1, 1), pbt.NoSave(), pbt.NoReplay(), pbt.Checks(100000))
}
func TestWeight2Mutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, weightedProperty(func() jobQueue { return &deepSwapQueue{} }, 2, 2), pbt.NoSave(), pbt.NoReplay(), pbt.Checks(100000))
}
func TestWeight4Mutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, weightedProperty(func() jobQueue { return &deepSwapQueue{} }, 4, 4), pbt.NoSave(), pbt.NoReplay(), pbt.Checks(100000))
}

func TestEveryPostDeepMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, everyPostProperty(func() jobQueue { return &deepSwapQueue{} }), pbt.NoSave(), pbt.NoReplay(), pbt.Checks(100000))
}

// ---- 監査の到達を要求にする
func coverProperty(newQueue func() jobQueue, enqueueWeight, auditWeight int) pbt.Property {
	return func(t *pbt.T) {
		q := newQueue()
		var pending []pbt.Var
		enqueue := pbt.Just(pbt.Command{
			Name: "enqueue",
			Run:  func(pbt.Env) any { return q.Enqueue() },
			Next: func(v pbt.Var) { pending = append(pending, v) },
		})
		audit := pbt.Just(pbt.Command{
			Name: "audit",
			Run:  func(pbt.Env) any { return q.IDs() },
			Post: func(_ pbt.Env, got any) error {
				if len(got.([]int)) != len(pending) {
					return fmt.Errorf("system holds %d jobs, model holds %d", len(got.([]int)), len(pending))
				}
				return nil
			},
		})
		commands := func() pbt.CommandGen {
			return pbt.Frequency(pbt.W(enqueueWeight, enqueue), pbt.W(auditWeight, audit))
		}
		prog := pbt.Commands(func() { pending = nil }, commands).Draw(t, "cmds")
		hasAudit := slices.ContainsFunc(prog.Steps(), func(s pbt.Step) bool {
			return s.Command.Name == "audit"
		})
		t.Cover(hasAudit, 50, "列に audit がある")
		prog.RunSequential(t)
	}
}

func TestCoverEnough(t *testing.T) {
	pbt.Check(t, coverProperty(func() jobQueue { return &JobQueue{} }, 4, 2), pbt.CheckCoverage(), pbt.Verbose())
}

// 重みを極端に下げると、要求を満たせず失敗する。
func TestCoverShortMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, coverProperty(func() jobQueue { return &JobQueue{} }, 500, 1), pbt.CheckCoverage(), pbt.NoSave(), pbt.NoReplay(), pbt.Seed(1))
}

// skipUnlessMeasuring は確率的に通る測定用のテストを、通常の実行から外す。
// PBT_MEASURE=1 を付け、measure_seeds.py からシードを変えて実行する。
func skipUnlessMeasuring(t *testing.T) {
	t.Helper()
	if os.Getenv("PBT_MEASURE") == "" {
		t.Skip("set PBT_MEASURE=1 to run")
	}
}

// 既定の100ケースで見つかるかの測定用（25件目以降で入れ替える）
type deeperSwapQueue struct{ JobQueue }

func (q *deeperSwapQueue) Enqueue() int {
	id := q.JobQueue.Enqueue()
	if n := len(q.jobs); n >= 25 {
		q.jobs[n-1], q.jobs[n-2] = q.jobs[n-2], q.jobs[n-1]
	}
	return id
}

func TestMeasureDeeper4(t *testing.T) {
	skipUnlessMeasuring(t)
	pbt.Check(t, weightedProperty(func() jobQueue { return &deeperSwapQueue{} }, 4, 4), pbt.NoSave(), pbt.NoReplay())
}
func TestMeasureDeeper0(t *testing.T) {
	skipUnlessMeasuring(t)
	pbt.Check(t, weightedProperty(func() jobQueue { return &deeperSwapQueue{} }, 0, 0), pbt.NoSave(), pbt.NoReplay())
}
func TestMeasureDeeper2(t *testing.T) {
	skipUnlessMeasuring(t)
	pbt.Check(t, weightedProperty(func() jobQueue { return &deeperSwapQueue{} }, 2, 2), pbt.NoSave(), pbt.NoReplay())
}

// ---- 4つの置き場所
type snapshot struct {
	n   int
	ids []int
}

func invariantProperty(newQueue func() jobQueue) pbt.Property {
	return func(t *pbt.T) {
		q := newQueue()
		in, out := 0, 0 // 投入と取り出しの回数

		enqueue := pbt.Just(pbt.Command{
			Name: "enqueue",
			Run:  func(pbt.Env) any { return q.Enqueue() },
			Next: func(pbt.Var) { in++ },
		})
		dequeue := pbt.Just(pbt.Command{
			Name: "dequeue",
			Pre:  func() bool { return in > out },
			Run:  func(pbt.Env) any { return q.Dequeue() },
			Next: func(pbt.Var) { out++ },
		})
		invariant := pbt.Just(pbt.Command{
			Name: "invariant",
			Run:  func(pbt.Env) any { return snapshot{q.Len(), q.IDs()} },
			Post: func(_ pbt.Env, got any) error {
				s := got.(snapshot)
				if s.n < 0 || s.n > in { // 範囲
					return fmt.Errorf("range: len() = %d after %d enqueues", s.n, in)
				}
				if len(s.ids) != in-out { // 保存と上限
					return fmt.Errorf("conservation: %d jobs held, want %d", len(s.ids), in-out)
				}
				sorted := slices.Sorted(slices.Values(s.ids))
				if len(slices.Compact(sorted)) != len(s.ids) { // 構造
					return fmt.Errorf("structure: duplicate job in %v", s.ids)
				}
				if s.n != len(s.ids) { // 関係の残り
					return fmt.Errorf("relation: len() = %d, but IDs() has %d", s.n, len(s.ids))
				}
				return nil
			},
		})
		commands := func() pbt.CommandGen {
			if in == out {
				return pbt.Frequency(pbt.W(3, enqueue), pbt.W(1, invariant))
			}
			return pbt.Frequency(pbt.W(3, enqueue), pbt.W(2, dequeue), pbt.W(1, invariant))
		}
		pbt.Commands(func() { in, out = 0, 0 }, commands).Draw(t, "cmds").RunSequential(t)
	}
}

func TestInvariant(t *testing.T) {
	pbt.Check(t, invariantProperty(func() jobQueue { return &JobQueue{} }))
}

// relationProperty は Len と IDs の関係だけを、モデルなしで確かめる。
func relationProperty(newQueue func() jobQueue) pbt.Property {
	return func(t *pbt.T) {
		q := newQueue()
		enqueue := pbt.Just(pbt.Command{
			Name: "enqueue",
			Run:  func(pbt.Env) any { return q.Enqueue() },
		})
		dequeue := pbt.Just(pbt.Command{
			Name: "dequeue",
			Run: func(pbt.Env) any {
				if len(q.IDs()) == 0 {
					return nil
				}
				return q.Dequeue()
			},
		})
		relation := pbt.Just(pbt.Command{
			Name: "relation",
			Run:  func(pbt.Env) any { return [2]int{q.Len(), len(q.IDs())} },
			Post: func(_ pbt.Env, got any) error {
				if r := got.([2]int); r[0] != r[1] {
					return fmt.Errorf("len() = %d, but IDs() has %d", r[0], r[1])
				}
				return nil
			},
		})
		commands := func() pbt.CommandGen { return pbt.OneOf(enqueue, dequeue, relation) }
		pbt.Commands(func() {}, commands).Draw(t, "cmds").RunSequential(t)
	}
}

func TestRelation(t *testing.T) {
	pbt.Check(t, relationProperty(func() jobQueue { return &JobQueue{} }))
}

// countDriftQueue は件数を別に数え、空になる取り出しで減らし忘れる。
type countDriftQueue struct {
	JobQueue
	n int
}

func (q *countDriftQueue) Enqueue() int {
	q.n++
	return q.JobQueue.Enqueue()
}

func (q *countDriftQueue) Dequeue() int {
	j := q.JobQueue.Dequeue()
	if len(q.jobs) > 0 {
		q.n--
	}
	return j
}

func (q *countDriftQueue) Len() int { return q.n }

// dupJobQueue は3回目の投入で前回と同じ ID を発行する。件数は正しい。
type dupJobQueue struct{ JobQueue }

func (q *dupJobQueue) Enqueue() int {
	id := q.JobQueue.Enqueue()
	if id == 3 {
		q.jobs[len(q.jobs)-1] = 2
		return 2
	}
	return id
}

// phantomQueue は最初の投入で余分な仕事を1つ作る。
type phantomQueue struct{ JobQueue }

func (q *phantomQueue) Enqueue() int {
	id := q.JobQueue.Enqueue()
	if id == 1 {
		q.JobQueue.Enqueue()
	}
	return id
}

// lostJobQueue は3回目の投入を捨てる。件数も IDs も一致している。
type lostJobQueue struct{ JobQueue }

func (q *lostJobQueue) Enqueue() int {
	id := q.JobQueue.Enqueue()
	if id == 3 {
		q.jobs = q.jobs[:len(q.jobs)-1]
	}
	return id
}

func TestRelationDriftMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, relationProperty(func() jobQueue { return &countDriftQueue{} }), noSave...)
}
func TestRelationLostSurvives(t *testing.T) { // Len と IDs がそろって仕事を失っても、関係は成り立つ
	pbt.Check(t, relationProperty(func() jobQueue { return &lostJobQueue{} }), noSave...)
}
func TestInvariantDriftMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, invariantProperty(func() jobQueue { return &countDriftQueue{} }), noSave...)
}
func TestInvariantDupMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, invariantProperty(func() jobQueue { return &dupJobQueue{} }), noSave...)
}
func TestInvariantLostMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, invariantProperty(func() jobQueue { return &lostJobQueue{} }), noSave...)
}
func TestInvariantPhantomMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, invariantProperty(func() jobQueue { return &phantomQueue{} }), noSave...)
}
func TestInvariantSwapSurvives(t *testing.T) { // 順序のバグは置き場所の検査では見えない
	pbt.Check(t, invariantProperty(func() jobQueue { return &swapQueue{} }), noSave...)
}
