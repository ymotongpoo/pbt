// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"maps"
	"testing"

	"go.ymotongpoo.com/pbt"
)

type KV struct{ m map[int]int }

func NewKV() *KV                    { return &KV{m: map[int]int{}} }
func (k *KV) Put(key, v int)        { k.m[key] = v }
func (k *KV) Snapshot() map[int]int { return maps.Clone(k.m) }

type putOp struct{ k, v int }

var putGen = pbt.Custom("put", func(t *pbt.T) putOp {
	return putOp{k: pbt.IntRange(0, 3).Draw(t, "k"), v: pbt.IntRange(0, 9).Draw(t, "v")}
})

func TestKVOverwriteEquivalence(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		ops := pbt.SliceOfN(putGen, 1, -1).Draw(t, "ops")
		base := NewKV()
		for _, op := range ops {
			base.Put(op.k, op.v)
		}
		dummy := putOp{k: ops[0].k, v: -1}
		transformed := NewKV()
		transformed.Put(dummy.k, dummy.v)
		for _, op := range ops {
			transformed.Put(op.k, op.v)
		}
		if !maps.Equal(base.Snapshot(), transformed.Snapshot()) {
			t.Fatalf("final states differ for %v", ops)
		}
	})
}
