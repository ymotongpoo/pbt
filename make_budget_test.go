// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package pbt

import (
	"reflect"
	"testing"
)

func TestBoundedGrowthSharesNestedAllowance(t *testing.T) {
	// Forced lengths complete even without fuel, but their elements still
	// spend it. There is no new budget for the second nested wrapper.
	g := BoundedGrowth(Custom("pair", func(pt *T) [2][]int {
		return [2][]int{SliceOfN(Int(), 8, 8).Draw(pt, ""), BoundedGrowth(SliceOf(Int())).Draw(pt, "")}
	}))
	prefix := make([]uint64, 64)
	for i := range prefix {
		prefix[i] = 1
	}
	pt := newT("", prefix, 0, 0, nil)
	v := g.Draw(pt, "")
	if len(v[0]) != 8 || len(v[1]) != 0 || pt.makeBudget != nil {
		t.Fatalf("%v budget=%v", v, pt.makeBudget)
	}
	// Negative control: without the outer scope the nested wrapper gets
	// eight fresh units and the same replay prefix grows its second slice.
	control := newT("", prefix, 0, 0, nil)
	SliceOfN(Int(), 8, 8).Draw(control, "")
	if n := len(BoundedGrowth(SliceOf(Int())).Draw(control, "")); n != 8 {
		t.Fatalf("negative control length=%d", n)
	}
}
func TestUnboundedGrowthPreservesGeneratorContract(t *testing.T) {
	g := BoundedGrowth(Custom("pair", func(pt *T) [2]string {
		return [2]string{UnboundedGrowth(StringN(20, 20)).Draw(pt, ""), StringN(8, 8).Draw(pt, "")}
	}))
	pt := newT("", nil, 0, 0, newRand(1))
	v := g.Draw(pt, "")
	want := newT("", nil, 0, 0, newRand(1))
	a, b := StringN(20, 20).Draw(want, ""), StringN(8, 8).Draw(want, "")
	if v != [2]string{a, b} || !reflect.DeepEqual(pt.choices, want.choices) {
		t.Fatalf("got %q want %q %q", v, a, b)
	}
}
func TestGrowthWrappersRestoreAfterPanic(t *testing.T) {
	pt := newT("", nil, 1, 1, newRand(1))
	outer := &budget{left: 3}
	pt.makeBudget = outer
	g := UnboundedGrowth(BoundedGrowth(Custom("panic", func(pt *T) int { panic("test") })))
	func() {
		defer func() {
			if recover() != "test" {
				t.Fatal("missing panic")
			}
		}()
		g.Draw(pt, "")
	}()
	if pt.makeBudget != outer || outer.left != 3 || outer.depth != 0 {
		t.Fatal("allowance not restored")
	}
}
func TestGrowthLimitDoesNotChangeOrdinaryCombinators(t *testing.T) {
	a := newT("", nil, 100, 100, newRand(1))
	b := newT("", nil, 100, 100, newRand(1))
	g := SliceOf(String())
	x, y := g.Draw(a, ""), UnboundedGrowth(g).Draw(b, "")
	if !reflect.DeepEqual(x, y) || !reflect.DeepEqual(a.choices, b.choices) || workUnits(reflect.ValueOf(x)) <= 800 {
		t.Fatal("ordinary combinator changed or did not exceed the derived budget")
	}
}

// A positive-min recursive chain used to stop at 21 nodes due to Make's
// span-based depth cutoff. It now stops at 64 growth steps, consistently
// with generated code and before the engine's own nesting guard discards it.
func TestMakeRequiredRecursiveChain(t *testing.T) {
	type chain struct {
		N    int
		Next []chain `pbt:"min=1,max=1"`
	}
	for _, size := range []int{0, 1, 10, 100, 1000} {
		pt := newT("", nil, size, size, newRand(1))
		v := Make[chain]().Draw(pt, "")
		n := 1
		for len(v.Next) > 0 {
			n++
			v = v.Next[0]
		}
		if want := 1 + min(8*max(size, 1), maxGrowthDepth); n != want {
			t.Fatalf("size=%d nodes=%d want=%d", size, n, want)
		}
	}
}
