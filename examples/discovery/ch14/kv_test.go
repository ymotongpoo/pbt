// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package ch14

import (
	"fmt"
	"maps"
	"os"
	"slices"
	"testing"

	"go.ymotongpoo.com/pbt"
	"go.ymotongpoo.com/pbt/examples/discovery/internal/mutant"
)

// KV はキーと値の組を保持する単純なストアである。
type KV struct{ m map[int]int }

func NewKV() *KV { return &KV{m: map[int]int{}} }

func (s *KV) Put(k, v int)          { s.m[k] = v }
func (s *KV) Get(k int) (int, bool) { v, ok := s.m[k]; return v, ok }
func (s *KV) Delete(k int)          { delete(s.m, k) }
func (s *KV) Snapshot() map[int]int { return maps.Clone(s.m) }

type Store interface {
	Put(k, v int)
	Get(k int) (int, bool)
	Delete(k int)
	Snapshot() map[int]int
}

type putOp struct{ k, v int }

var putGen = pbt.Custom("put", func(t *pbt.T) putOp {
	return putOp{k: pbt.IntRange(0, 3).Draw(t, "k"), v: pbt.IntRange(0, 9).Draw(t, "v")}
})

// run は新しいストアに ops を順に実行して返す。
func run(newStore func() Store, ops []putOp) Store {
	s := newStore()
	for _, op := range ops {
		s.Put(op.k, op.v)
	}
	return s
}

func overwriteProperty(newStore func() Store) pbt.Property {
	return func(t *pbt.T) {
		ops := pbt.SliceOfN(putGen, 1, -1).Draw(t, "ops")
		dummy := putOp{k: ops[0].k, v: -1}

		base := run(newStore, ops)
		transformed := run(newStore, append([]putOp{dummy}, ops...))

		if !maps.Equal(base.Snapshot(), transformed.Snapshot()) {
			t.Fatalf("final states differ for %v", ops)
		}
	}
}

// 前提のない取り消し：k が既に存在すると成り立たない
func undoAnyProperty(newStore func() Store) pbt.Property {
	return func(t *pbt.T) {
		ops := pbt.SliceOf(putGen).Draw(t, "ops")
		k := pbt.IntRange(0, 3).Draw(t, "k")

		base := run(newStore, ops)
		undone := run(newStore, ops)
		undone.Put(k, -1)
		undone.Delete(k)

		if !maps.Equal(base.Snapshot(), undone.Snapshot()) {
			t.Fatalf("Put(%d) then Delete(%d) changed the state after %v", k, k, ops)
		}
	}
}

// k を「まだ存在しないキー」に限った取り消し
func undoProperty(newStore func() Store) pbt.Property {
	return func(t *pbt.T) {
		// キーは4種類なので、3手以下なら未使用のキーが必ず残る
		ops := pbt.SliceOfN(putGen, 0, 3).Draw(t, "ops")
		base := run(newStore, ops)
		absent := slices.DeleteFunc([]int{0, 1, 2, 3}, func(k int) bool {
			_, ok := base.Get(k)
			return ok
		})
		k := pbt.SampledFrom(absent).Draw(t, "k")

		undone := run(newStore, ops)
		undone.Put(k, -1)
		undone.Delete(k)

		if !maps.Equal(base.Snapshot(), undone.Snapshot()) {
			t.Fatalf("Put(%d) then Delete(%d) changed the state after %v", k, k, ops)
		}
	}
}

func swapProperty(newStore func() Store) pbt.Property {
	return func(t *pbt.T) {
		before := pbt.SliceOf(putGen).Draw(t, "before")
		x := putGen.Draw(t, "x")
		y := putGen.Draw(t, "y")
		after := pbt.SliceOfN(putGen, 0, 2).Draw(t, "after")
		t.Assume(x.k != y.k) // 影響範囲が重ならない組に限る

		xFirst := run(newStore, slices.Concat(before, []putOp{x, y}, after))
		yFirst := run(newStore, slices.Concat(before, []putOp{y, x}, after))

		if !maps.Equal(xFirst.Snapshot(), yFirst.Snapshot()) {
			t.Fatalf("swapping %v and %v changed the state", x, y)
		}
	}
}

// 同じキーも含めて交換する版
func swapAnyProperty(newStore func() Store) pbt.Property {
	return func(t *pbt.T) {
		before := pbt.SliceOf(putGen).Draw(t, "before")
		x := putGen.Draw(t, "x")
		y := putGen.Draw(t, "y")
		after := pbt.SliceOfN(putGen, 0, 2).Draw(t, "after")

		xFirst := run(newStore, slices.Concat(before, []putOp{x, y}, after))
		yFirst := run(newStore, slices.Concat(before, []putOp{y, x}, after))

		if !maps.Equal(xFirst.Snapshot(), yFirst.Snapshot()) {
			t.Fatalf("swapping %v and %v changed the state", x, y)
		}
	}
}

func readProperty(newStore func() Store, read func(s Store, k int)) pbt.Property {
	return func(t *pbt.T) {
		before := pbt.SliceOf(putGen).Draw(t, "before")
		k := pbt.IntRange(0, 3).Draw(t, "k")
		after := pbt.SliceOfN(putGen, 0, 2).Draw(t, "after")

		base := run(newStore, slices.Concat(before, after))
		probed := run(newStore, before)
		read(probed, k) // 列の途中に照会を1回挟む
		for _, op := range after {
			probed.Put(op.k, op.v)
		}

		if !maps.Equal(base.Snapshot(), probed.Snapshot()) {
			t.Fatalf("reading key %d changed the state", k)
		}
	}
}

// swapLongTailProperty は後半の列の長さを絞らない比較用の版である。
// 後半の上書きが差を消すので、既定の100ケースでは検出が安定しない。
func swapLongTailProperty(newStore func() Store) pbt.Property {
	return func(t *pbt.T) {
		before := pbt.SliceOf(putGen).Draw(t, "before")
		x := putGen.Draw(t, "x")
		y := putGen.Draw(t, "y")
		after := pbt.SliceOf(putGen).Draw(t, "after")
		t.Assume(x.k != y.k)

		xFirst := run(newStore, slices.Concat(before, []putOp{x, y}, after))
		yFirst := run(newStore, slices.Concat(before, []putOp{y, x}, after))

		if !maps.Equal(xFirst.Snapshot(), yFirst.Snapshot()) {
			t.Fatalf("swapping %v and %v changed the state", x, y)
		}
	}
}

// readLongTailProperty も同じく比較用の版である。
func readLongTailProperty(newStore func() Store, read func(s Store, k int)) pbt.Property {
	return func(t *pbt.T) {
		before := pbt.SliceOf(putGen).Draw(t, "before")
		k := pbt.IntRange(0, 3).Draw(t, "k")
		after := pbt.SliceOf(putGen).Draw(t, "after")

		base := run(newStore, slices.Concat(before, after))
		probed := run(newStore, before)
		read(probed, k)
		for _, op := range after {
			probed.Put(op.k, op.v)
		}

		if !maps.Equal(base.Snapshot(), probed.Snapshot()) {
			t.Fatalf("reading key %d changed the state", k)
		}
	}
}

// LRU は容量を超えると、最も長く使われていないキーを追い出すキャッシュである。
type LRU struct {
	capacity int
	order    []int // 使われた順。先頭が最も古い
	m        map[int]int
}

func NewLRU(capacity int) *LRU { return &LRU{capacity: capacity, m: map[int]int{}} }

func (c *LRU) touch(k int) {
	c.order = slices.DeleteFunc(c.order, func(o int) bool { return o == k })
	c.order = append(c.order, k)
}

func (c *LRU) Put(k, v int) {
	c.m[k] = v
	c.touch(k)
	if len(c.order) > c.capacity {
		delete(c.m, c.order[0])
		c.order = c.order[1:]
	}
}

// Get は値を返し、そのキーを最近使ったものとして記録する。
func (c *LRU) Get(k int) (int, bool) {
	v, ok := c.m[k]
	if ok {
		c.touch(k)
	}
	return v, ok
}

// Peek は値を返すだけで、使われた順を変えない。
func (c *LRU) Peek(k int) (int, bool) {
	v, ok := c.m[k]
	return v, ok
}

func (c *LRU) Delete(k int) {
	delete(c.m, k)
	c.order = slices.DeleteFunc(c.order, func(o int) bool { return o == k })
}

func (c *LRU) Snapshot() map[int]int { return maps.Clone(c.m) }

// entry は Get の2つの返り値をまとめたものである。
type entry struct {
	v  int
	ok bool
}

func embeddedProperty(newStore func() Store) pbt.Property {
	return func(t *pbt.T) {
		s := newStore()
		model := map[int]int{} // モデル：キーと値の対応

		overwrite := pbt.Map(putGen, func(op putOp) pbt.Command {
			return pbt.Command{
				Name: "overwrite", Args: []any{op.k, op.v},
				Run: func(pbt.Env) any {
					s.Put(op.k, -1) // 上書きされて消えるはずの値
					s.Put(op.k, op.v)
					return nil
				},
				Next: func(pbt.Var) { model[op.k] = op.v }, // モデルは本物の1回だけ更新する
			}
		})
		get := pbt.Map(pbt.IntRange(0, 3), func(k int) pbt.Command {
			return pbt.Command{
				Name: "get", Args: []any{k},
				Run: func(pbt.Env) any {
					v, ok := s.Get(k)
					return entry{v, ok}
				},
				Post: func(_ pbt.Env, got any) error {
					v, ok := model[k]
					if want := (entry{v, ok}); got != want {
						return fmt.Errorf("get(%d) = %v, want %v", k, got, want)
					}
					return nil
				},
			}
		})
		commands := func() pbt.CommandGen { return pbt.OneOf(overwrite, get) }
		init := func() { clear(model) }

		prog := pbt.Commands(init, commands).Draw(t, "cmds")
		prog.RunSequential(t)
	}
}

// ---- 変異

// dropOnce は既存キーへの Put を1回だけ無視する。
type dropOnce struct {
	KV
	done bool
}

func (s *dropOnce) Put(k, v int) {
	if _, ok := s.m[k]; ok && !s.done {
		s.done = true
		return
	}
	s.m[k] = v
}

// softDelete は Delete でキーを消さず、値を0にして残す。
type softDelete struct{ KV }

func (s *softDelete) Delete(k int) { s.m[k] = 0 }

// sharedSlot はキーの下位1ビットだけを見て、違うキーを同じ場所に書く。
type sharedSlot struct{ KV }

func (s *sharedSlot) Put(k, v int)          { s.m[k%2] = v }
func (s *sharedSlot) Get(k int) (int, bool) { v, ok := s.m[k%2]; return v, ok }

// staleRead は Get の結果を覚え、Put の後も古い値を返す。Snapshot は正しい。
type staleRead struct {
	KV
	cache map[int]int
}

func (s *staleRead) Get(k int) (int, bool) {
	if v, ok := s.cache[k]; ok {
		return v, true
	}
	v, ok := s.KV.Get(k)
	if ok {
		s.cache[k] = v
	}
	return v, ok
}

// strayKey は既存キーを上書きするとき、古い値を別のキーへ退避したまま残す。
type strayKey struct{ KV }

func (s *strayKey) Put(k, v int) {
	if old, ok := s.m[k]; ok {
		s.m[100+k] = old
	}
	s.m[k] = v
}

func mkKV() Store         { return NewKV() }
func mkDropOnce() Store   { return &dropOnce{KV: *NewKV()} }
func mkSoftDelete() Store { return &softDelete{*NewKV()} }
func mkSharedSlot() Store { return &sharedSlot{*NewKV()} }
func mkStaleRead() Store  { return &staleRead{KV: *NewKV(), cache: map[int]int{}} }
func mkStrayKey() Store   { return &strayKey{*NewKV()} }
func mkLRU() Store        { return NewLRU(2) }

var opts = []pbt.Option{pbt.NoSave(), pbt.NoReplay()}

func get(s Store, k int)  { s.Get(k) }
func peek(s Store, k int) { s.(*LRU).Peek(k) }

func TestOverwrite(t *testing.T) { pbt.Check(t, overwriteProperty(mkKV), opts...) }
func TestOverwriteDropOnceMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, overwriteProperty(mkDropOnce), opts...)
}
func TestUndoAnyMutant(t *testing.T) { mutant.Only(t); pbt.Check(t, undoAnyProperty(mkKV), opts...) }
func TestUndo(t *testing.T)          { pbt.Check(t, undoProperty(mkKV), opts...) }
func TestUndoSoftDeleteMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, undoProperty(mkSoftDelete), opts...)
}
func TestSwap(t *testing.T) { pbt.Check(t, swapProperty(mkKV), opts...) }
func TestSwapSharedSlotMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, swapProperty(mkSharedSlot), opts...)
}
func TestSwapSameKeyMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, swapAnyProperty(mkKV), opts...)
}
func TestSwapLRUMutant(t *testing.T) { mutant.Only(t); pbt.Check(t, swapProperty(mkLRU), opts...) }
func TestRead(t *testing.T)          { pbt.Check(t, readProperty(mkKV, get), opts...) }
func TestReadLRUGetMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, readProperty(mkLRU, get), append(opts, pbt.Checks(1000))...)
}
func TestReadLRUPeek(t *testing.T) { pbt.Check(t, readProperty(mkLRU, peek), opts...) }

// 検出率の比較用。結果が実行ごとに揺れるので、PBT_FLAKY=1 のときだけ動かす。
func flaky(t *testing.T) {
	if os.Getenv("PBT_FLAKY") == "" {
		t.Skip("set PBT_FLAKY=1 to measure the detection rate")
	}
}

func TestFlakySwapLongTailSharedSlot(t *testing.T) {
	flaky(t)
	pbt.Check(t, swapLongTailProperty(mkSharedSlot), opts...)
}
func TestFlakySwapLongTailLRU(t *testing.T) {
	flaky(t)
	pbt.Check(t, swapLongTailProperty(mkLRU), opts...)
}
func TestFlakyReadLongTailLRUGet(t *testing.T) {
	flaky(t)
	pbt.Check(t, readLongTailProperty(mkLRU, get), opts...)
}
func TestFlakyReadLRUGet(t *testing.T) {
	flaky(t)
	pbt.Check(t, readProperty(mkLRU, get), opts...)
}

func TestEmbedded(t *testing.T) { pbt.Check(t, embeddedProperty(mkKV), opts...) }
func TestEmbeddedDropOnceMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, embeddedProperty(mkDropOnce), opts...)
}
func TestEmbeddedStaleReadMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, embeddedProperty(mkStaleRead), opts...)
}
func TestOverwriteStaleReadSurvives(t *testing.T) {
	pbt.Check(t, overwriteProperty(mkStaleRead), append(opts, pbt.Checks(2000))...)
}
func TestOverwriteStrayKeyMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, overwriteProperty(mkStrayKey), opts...)
}
func TestEmbeddedStrayKeySurvives(t *testing.T) {
	pbt.Check(t, embeddedProperty(mkStrayKey), append(opts, pbt.Checks(2000))...)
}

// 直後の照会がすべて一致しても、同じ後続の操作で区別できる列がある。
func TestLRUContinuationDistinguishes(t *testing.T) {
	a, b := NewLRU(2), NewLRU(2)
	a.Put(0, 0)
	a.Put(1, 0)
	b.Put(1, 0)
	b.Put(0, 0)
	if !maps.Equal(a.Snapshot(), b.Snapshot()) {
		t.Fatal("snapshots differ right after the sequences")
	}
	a.Put(2, 0)
	b.Put(2, 0)
	if maps.Equal(a.Snapshot(), b.Snapshot()) {
		t.Fatal("expected the same continuation to tell them apart")
	}
}
