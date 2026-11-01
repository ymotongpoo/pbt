// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

// Package ch18 は第18章の統計の読み方と分布の直し方を実行する。
package ch18

import (
	"fmt"
	"slices"
	"testing"
	"unicode/utf8"

	"go.ymotongpoo.com/pbt"
	"go.ymotongpoo.com/pbt/examples/discovery/internal/mutant"
)

var opts = []pbt.Option{pbt.NoSave(), pbt.NoReplay(), pbt.Seed(1)}

func with(o ...pbt.Option) []pbt.Option { return append(slices.Clone(opts), o...) }

func indexLinear(xs []int, v int) int {
	for i, x := range xs {
		if x == v {
			return i
		}
	}
	return -1
}

var sortedDistinct = pbt.Map(
	pbt.SliceOf(pbt.IntRange(0, 30)),
	func(xs []int) []int {
		slices.Sort(xs)
		return slices.Compact(xs)
	},
)

// ---- Classify

func TestClassify(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		xs := sortedDistinct.Draw(t, "xs")
		v := pbt.IntRange(0, 30).Draw(t, "v")

		t.Classify(slices.Contains(xs, v), "found")
		t.Classify(len(xs) == 0, "empty")
		t.Classify(len(xs) >= 16, "long")
	}, opts...)
}

// ---- Collect

func position(xs []int, v int) string {
	switch i := indexLinear(xs, v); {
	case i < 0:
		return "absent"
	case i == 0:
		return "first"
	case i == len(xs)-1:
		return "last"
	}
	return "middle"
}

func TestCollect(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		xs := sortedDistinct.Draw(t, "xs")
		v := pbt.IntRange(0, 30).Draw(t, "v")

		t.Collect(position(xs, v))
	}, opts...)
}

// ---- Measure

func TestMeasure(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		xs := sortedDistinct.Draw(t, "xs")

		t.Measure("len", float64(len(xs)))
	}, opts...)
}

// ---- Aggregate

func runeWidth(r rune) string {
	return fmt.Sprintf("%d byte", utf8.RuneLen(r))
}

func TestAggregateRunes(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		s := pbt.String().Draw(t, "s")
		for _, r := range s {
			t.Aggregate(runeWidth(r))
		}
		t.Classify(!isASCII(s), "non-ASCII")
	}, opts...)
}

func isASCII(s string) bool {
	return len(s) == utf8.RuneCountInString(s)
}

// ---- 値域を直す前後：found の割合

func TestFoundRanges(t *testing.T) {
	for _, hi := range []int{30, 1000, 1_000_000} {
		t.Run(fmt.Sprint(hi), func(t *testing.T) {
			pbt.Check(t, func(t *pbt.T) {
				xs := pbt.SliceOf(pbt.IntRange(0, hi)).Draw(t, "xs")
				v := pbt.IntRange(0, hi).Draw(t, "v")
				t.Classify(slices.Contains(xs, v), "found")
			}, opts...)
		})
	}
}

// ---- Cover

func TestCoverWideShortfallMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, func(t *pbt.T) {
		xs := pbt.SliceOf(pbt.IntRange(0, 1_000_000)).Draw(t, "xs")
		v := pbt.IntRange(0, 1_000_000).Draw(t, "v")

		t.Cover(slices.Contains(xs, v), 40, "found")
	}, with(pbt.CheckCoverage())...)
}

func TestCoverUndecidedMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, func(t *pbt.T) {
		xs := pbt.SliceOf(pbt.IntRange(0, 1_000_000)).Draw(t, "xs")
		v := pbt.IntRange(0, 1_000_000).Draw(t, "v")

		t.Cover(slices.Contains(xs, v), 21, "found")
	}, with(pbt.CheckCoverage())...)
}

func TestCoverDecidedAfterRaisingChecksMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, func(t *pbt.T) {
		xs := pbt.SliceOf(pbt.IntRange(0, 1_000_000)).Draw(t, "xs")
		v := pbt.IntRange(0, 1_000_000).Draw(t, "v")

		t.Cover(slices.Contains(xs, v), 21, "found")
	}, with(pbt.CheckCoverage(), pbt.Checks(1000))...)
}

func TestCoverFixed(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		xs := pbt.SliceOf(pbt.IntRange(0, 1_000_000)).Draw(t, "xs")
		candidates := pbt.IntRange(0, 1_000_000)
		if len(xs) > 0 {
			candidates = pbt.OneOf(pbt.SampledFrom(xs), candidates)
		}
		v := candidates.Draw(t, "v")

		found := slices.Contains(xs, v)
		t.Classify(found, "found")
		t.Cover(found, 40, "found")
		t.Cover(!found, 20, "absent")
	}, with(pbt.CheckCoverage())...)
}

// Collect の偏りを直す
func target(xs []int) pbt.Gen[int] {
	values := pbt.IntRange(0, 30)
	if len(xs) == 0 {
		return values
	}
	return pbt.Frequency(
		pbt.W(3, values),
		pbt.W(1, pbt.Just(xs[len(xs)-1])),
	)
}

func TestCollectFixed(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		xs := sortedDistinct.Draw(t, "xs")
		v := target(xs).Draw(t, "v")

		t.Collect(position(xs, v))
	}, opts...)
}

// ---- コマンド列の混合比

type JobQueue struct {
	next int
	jobs []int
}

func (q *JobQueue) Enqueue() int { q.next++; q.jobs = append(q.jobs, q.next); return q.next }
func (q *JobQueue) Dequeue() int { j := q.jobs[0]; q.jobs = q.jobs[1:]; return j }
func (q *JobQueue) IDs() []int   { return slices.Clone(q.jobs) }

type mix struct {
	enqueue, dequeue, audit int
	length                  []pbt.StatefulOption
}

func jobQueueProperty(m mix) pbt.Property {
	return func(t *pbt.T) {
		q := &JobQueue{}
		var pending []pbt.Var
		deepest := 0

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
		audit := pbt.Just(pbt.Command{
			Name: "audit",
			Run: func(pbt.Env) any {
				ids := q.IDs()
				deepest = max(deepest, len(ids))
				return ids
			},
			Post: func(env pbt.Env, got any) error {
				ids := got.([]int)
				if len(ids) != len(pending) {
					return fmt.Errorf("system holds %d jobs, model holds %d", len(ids), len(pending))
				}
				return nil
			},
		})
		commands := func() pbt.CommandGen {
			if len(pending) == 0 {
				return pbt.Frequency(pbt.W(m.enqueue, enqueue), pbt.W(m.audit, audit))
			}
			return pbt.Frequency(pbt.W(m.enqueue, enqueue), pbt.W(m.dequeue, dequeue()), pbt.W(m.audit, audit))
		}
		prog := pbt.Commands(func() { pending = nil }, commands, m.length...).Draw(t, "cmds")
		prog.RunSequential(t)

		t.Measure("commands", float64(prog.Len()))
		t.Measure("deepest audit", float64(deepest))
		t.Classify(deepest >= 5, "audited at depth 5 or more")
	}
}

func TestMixEven(t *testing.T) {
	pbt.Check(t, jobQueueProperty(mix{enqueue: 1, dequeue: 1, audit: 1}), opts...)
}
func TestMixWeighted(t *testing.T) {
	pbt.Check(t, jobQueueProperty(mix{enqueue: 4, dequeue: 2, audit: 3}), opts...)
}
func TestMixWeightedLong(t *testing.T) {
	pbt.Check(t, jobQueueProperty(mix{enqueue: 4, dequeue: 2, audit: 3, length: []pbt.StatefulOption{pbt.Length(20, 60)}}), opts...)
}
func TestMixDefaultSizes200(t *testing.T) {
	pbt.Check(t, jobQueueProperty(mix{enqueue: 4, dequeue: 2, audit: 3}), with(pbt.Sizes(1, 200))...)
}

// ---- 生成器を偏らせる

var mixedRune = pbt.Frequency(
	pbt.W(2, pbt.SampledFrom([]rune("abc xyz"))),
	pbt.W(2, pbt.SampledFrom([]rune("あいう漢字"))),
	pbt.W(1, pbt.SampledFrom([]rune("𠮷𩸽"))),
)

func TestAggregateMixed(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		s := pbt.StringOf(mixedRune).Draw(t, "s")
		for _, r := range s {
			t.Aggregate(runeWidth(r))
		}
		t.Classify(!isASCII(s), "non-ASCII")
	}, opts...)
}

// ---- サイズ

func TestSizesDefault(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		xs := pbt.SliceOf(pbt.Int()).Draw(t, "xs")
		t.Measure("len", float64(len(xs)))
	}, opts...)
}

func TestSizesLarge(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		xs := pbt.SliceOf(pbt.Int()).Draw(t, "xs")
		t.Measure("len", float64(len(xs)))
	}, with(pbt.Sizes(1, 200))...)
}

type Tree struct {
	Value    int
	Children []Tree
}

func tree() pbt.Gen[Tree] {
	return pbt.Sized(func(size int) pbt.Gen[Tree] {
		if size <= 1 {
			return pbt.Map(pbt.Int(), func(v int) Tree { return Tree{Value: v} })
		}
		return pbt.Custom("tree", func(t *pbt.T) Tree {
			child := pbt.Resize(size/2, tree())
			return Tree{Value: pbt.Int().Draw(t, ""), Children: pbt.SliceOfN(child, 0, 3).Draw(t, "")}
		})
	})
}

func (n Tree) depth() int {
	d := 0
	for _, c := range n.Children {
		d = max(d, c.depth())
	}
	return d + 1
}

func (n Tree) nodes() int {
	c := 1
	for _, x := range n.Children {
		c += x.nodes()
	}
	return c
}

func TestTreeDefault(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		n := tree().Draw(t, "tree")
		t.Measure("depth", float64(n.depth()))
		t.Measure("nodes", float64(n.nodes()))
	}, opts...)
}

func TestTreeLarge(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		n := tree().Draw(t, "tree")
		t.Measure("depth", float64(n.depth()))
		t.Measure("nodes", float64(n.nodes()))
	}, with(pbt.Sizes(1, 200), pbt.Checks(200))...)
}

func TestResizeZero(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		xs := pbt.Resize(0, pbt.SliceOf(pbt.Int())).Draw(t, "xs")
		ys := pbt.Resize(100, pbt.SliceOf(pbt.Int())).Draw(t, "ys")
		t.Measure("Resize(0)", float64(len(xs)))
		t.Measure("Resize(100)", float64(len(ys)))
	}, opts...)
}

func TestSizesLargeChecks(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		xs := pbt.SliceOf(pbt.Int()).Draw(t, "xs")
		t.Measure("len", float64(len(xs)))
	}, with(pbt.Sizes(1, 200), pbt.Checks(200))...)
}

// Classify と Collect は呼ばれた回数だけ数える。同じケースの重複を除かない。
func TestRepeatedLabelsCountPerCall(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		t.Classify(true, "hit")
		t.Classify(true, "hit")
		t.Collect("kind")
		t.Collect("kind")
	}, pbt.NoSave(), pbt.NoReplay())
}

// Resize が固定するのはサイズで、明示した長さの制約は保たれる。
func TestResizeKeepsLengthBounds(t *testing.T) {
	g := pbt.Resize(0, pbt.SliceOfN(pbt.Int(), 1, 3))
	pbt.Check(t, func(t *pbt.T) {
		if xs := g.Draw(t, "xs"); len(xs) != 1 {
			t.Fatalf("len = %d", len(xs))
		}
	}, pbt.NoSave(), pbt.NoReplay())
}

// 100%の要求は統計の判定によらず、全ケースで成り立てば充足する。
func TestCover100IsNotStatistical(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		t.Cover(true, 100, "always")
	}, pbt.CheckCoverage(), pbt.NoSave(), pbt.NoReplay())
}

// 100%の要求は1ケースでも外れれば不足になる。
func TestCover100MissMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, func(t *pbt.T) {
		n := pbt.IntRange(0, 9).Draw(t, "n")
		t.Cover(n != 0, 100, "never zero")
	}, pbt.CheckCoverage(), pbt.NoSave(), pbt.NoReplay())
}
