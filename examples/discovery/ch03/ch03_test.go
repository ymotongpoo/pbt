// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package ch03

import (
	"slices"
	"testing"

	"go.ymotongpoo.com/pbt"
	"go.ymotongpoo.com/pbt/examples/discovery/internal/mutant"
)

// Dedup は xs から重複を取り除き、最初に現れた順序を保って返す。
func Dedup(xs []int) []int {
	seen := map[int]bool{}
	var out []int
	for _, x := range xs {
		if !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	return out
}

func TestDedupExamples(t *testing.T) {
	cases := []struct{ in, want []int }{
		{[]int{1, 1, 2}, []int{1, 2}},
		{[]int{3, 1, 3}, []int{3, 1}},
		{[]int{}, []int{}},
	}
	for _, c := range cases {
		if got := Dedup(c.in); !slices.Equal(got, c.want) {
			t.Errorf("Dedup(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

func hasDuplicate(xs []int) bool {
	seen := map[int]bool{}
	for _, x := range xs {
		if seen[x] {
			return true
		}
		seen[x] = true
	}
	return false
}

// ---- 変異

// dedupAdjacent は前の要素とだけ比べる。
func dedupAdjacent(xs []int) []int {
	var out []int
	for i, x := range xs {
		if i == 0 || xs[i-1] != x {
			out = append(out, x)
		}
	}
	return out
}

// dedupTwice は2回目の出現だけを消し、3回目以降を残す。
func dedupTwice(xs []int) []int {
	seen := map[int]int{}
	var out []int
	for _, x := range xs {
		seen[x]++
		if seen[x] != 2 {
			out = append(out, x)
		}
	}
	return out
}

// dedupSwap は重複を見つけると末尾の要素で埋めて縮める。
func dedupSwap(xs []int) []int {
	out := slices.Clone(xs)
	seen := map[int]bool{}
	for i := 0; i < len(out); {
		if seen[out[i]] {
			out[i] = out[len(out)-1]
			out = out[:len(out)-1]
		} else {
			seen[out[i]] = true
			i++
		}
	}
	return out
}

// dedupLast は最後に現れた順序で返す。
func dedupLast(xs []int) []int {
	seen := map[int]bool{}
	var out []int
	for i := len(xs) - 1; i >= 0; i-- {
		if !seen[xs[i]] {
			seen[xs[i]] = true
			out = append(out, xs[i])
		}
	}
	slices.Reverse(out)
	return out
}

// dedupFast は4要素以上の経路でだけ先頭の要素を読み飛ばす。
func dedupFast(xs []int) []int {
	if len(xs) <= 3 {
		return Dedup(xs)
	}
	seen := map[int]bool{}
	var out []int
	for _, x := range xs[1:] {
		if !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	return out
}

// dedupNonNil は空入力に nil ではなく空スライスを返す。
func dedupNonNil(xs []int) []int {
	if len(xs) == 0 {
		return []int{}
	}
	return Dedup(xs)
}

var mutants = []struct {
	name string
	f    func([]int) []int
}{
	{"Adjacent", dedupAdjacent},
	{"Twice", dedupTwice},
	{"Swap", dedupSwap},
	{"Last", dedupLast},
	{"Fast", dedupFast},
	{"NonNil", dedupNonNil},
}

// ---- 各段階の版

// wrapped は例をそのまま Check で包んだ版である。
func wrapped(dedup func([]int) []int) pbt.Property {
	type exampleCase struct{ in, want []int }
	cases := []exampleCase{
		{[]int{1, 1, 2}, []int{1, 2}},
		{[]int{3, 1, 3}, []int{3, 1}},
		{[]int{}, []int{}},
	}
	return func(t *pbt.T) {
		c := pbt.SampledFrom(cases).Draw(t, "c")
		if got := dedup(c.in); !slices.Equal(got, c.want) {
			t.Fatalf("Dedup(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

// valuesOnly は例の値だけを生成器に変えた版である。
func valuesOnly(dedup func([]int) []int) pbt.Property {
	return func(t *pbt.T) {
		a := pbt.IntRange(-100, 100).Draw(t, "a")
		b := a + pbt.IntRange(1, 100).Draw(t, "d") // a と b は必ず異なる

		if got, want := dedup([]int{a, a, b}), []int{a, b}; !slices.Equal(got, want) {
			t.Fatalf("Dedup(%v) = %v, want %v", []int{a, a, b}, got, want)
		}
		if got, want := dedup([]int{b, a, b}), []int{b, a}; !slices.Equal(got, want) {
			t.Fatalf("Dedup(%v) = %v, want %v", []int{b, a, b}, got, want)
		}
	}
}

// checkSameElements は関係1と2を確かめる。
func checkSameElements(t *pbt.T, xs, got []int) {
	count := map[int]int{}
	for _, x := range got {
		count[x]++
		if count[x] > 1 {
			t.Fatalf("Dedup(%v) = %v has duplicate %d", xs, got, x)
		}
		if !slices.Contains(xs, x) {
			t.Fatalf("Dedup(%v) = %v invented element %d", xs, got, x)
		}
	}
	for _, x := range xs {
		if count[x] == 0 {
			t.Fatalf("Dedup(%v) = %v lost element %d", xs, got, x)
		}
	}
}

// relations12 は関係1と2だけを述べる版である。
func relations12(dedup func([]int) []int) pbt.Property {
	return func(t *pbt.T) {
		xs := pbt.SliceOf(pbt.IntRange(0, 3)).Draw(t, "xs")
		checkSameElements(t, xs, dedup(xs))
	}
}

// isSubsequence は sub が xs の部分列かを返す。
func isSubsequence(sub, xs []int) bool {
	i := 0
	for _, x := range xs {
		if i < len(sub) && sub[i] == x {
			i++
		}
	}
	return i == len(sub)
}

// relationSubseq は関係1と2に部分列の関係を足した版である。
func relationSubseq(dedup func([]int) []int) pbt.Property {
	return func(t *pbt.T) {
		xs := pbt.SliceOf(pbt.IntRange(0, 3)).Draw(t, "xs")
		got := dedup(xs)
		checkSameElements(t, xs, got)
		if !isSubsequence(got, xs) {
			t.Fatalf("Dedup(%v) = %v is not a subsequence", xs, got)
		}
	}
}

// relationFirst は関係1と2に最初の出現順の関係を足した版である。
func relationFirst(dedup func([]int) []int) pbt.Property {
	return func(t *pbt.T) {
		xs := pbt.SliceOf(pbt.IntRange(0, 3)).Draw(t, "xs")
		got := dedup(xs)
		checkSameElements(t, xs, got)
		for i := 1; i < len(got); i++ {
			if slices.Index(xs, got[i-1]) >= slices.Index(xs, got[i]) {
				t.Fatalf("Dedup(%v) = %v: %d is out of order", xs, got, got[i])
			}
		}
	}
}

var stages = []struct {
	name string
	prop func(func([]int) []int) pbt.Property
}{
	{"Wrapped", wrapped},
	{"ValuesOnly", valuesOnly},
	{"Rel12", relations12},
	{"RelSubseq", relationSubseq},
	{"RelFirst", relationFirst},
}

func TestStagesOnDedup(t *testing.T) {
	for _, s := range stages {
		t.Run(s.name, func(t *testing.T) { pbt.Check(t, s.prop(Dedup)) })
	}
}

// 変異と版の総当たり。失敗するサブテストがあるので -run で個別に確認する。
func TestStageTableMutant(t *testing.T) {
	mutant.Only(t)
	for _, m := range mutants {
		for _, s := range stages {
			t.Run(m.name+"/"+s.name, func(t *testing.T) {
				pbt.Check(t, s.prop(m.f), pbt.NoSave(), pbt.NoReplay())
			})
		}
	}
}

// ---- 3.6 の悪い例

// 悪い例：例 {1, 1, 2} の形だけを一般化している。
func onlyAdjacent(dedup func([]int) []int) pbt.Property {
	return func(t *pbt.T) {
		a := pbt.IntRange(-100, 100).Draw(t, "a")
		b := a + pbt.IntRange(1, 100).Draw(t, "d")
		n := pbt.IntRange(1, 10).Draw(t, "n")

		xs := append(slices.Repeat([]int{a}, n), b)
		if got, want := dedup(xs), []int{a, b}; !slices.Equal(got, want) {
			t.Fatalf("Dedup(%v) = %v, want %v", xs, got, want)
		}
	}
}

func TestOnlyAdjacentAdjacentSurvives(t *testing.T) {
	pbt.Check(t, onlyAdjacent(dedupAdjacent), pbt.NoSave(), pbt.NoReplay())
}

func TestRelationsAdjacentMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, relations12(dedupAdjacent), pbt.NoSave(), pbt.NoReplay())
}

// 関係1、2、最初の出現順がそろえば、Dedup の結果と一致する。
func TestRelationsDetermineResult(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		xs := pbt.SliceOf(pbt.IntRange(0, 3)).Draw(t, "xs")
		pool := pbt.Permutation(Dedup(xs)).Draw(t, "perm")
		ordered := true
		for i := 1; i < len(pool); i++ {
			if slices.Index(xs, pool[i-1]) >= slices.Index(xs, pool[i]) {
				ordered = false
			}
		}
		if ordered && !slices.Equal(pool, Dedup(xs)) {
			t.Fatalf("relations hold but %v != %v", pool, Dedup(xs))
		}
	})
}

// hasGap は、同じ値が間に別の値を挟んで現れるかを返す。
func hasGap(xs []int) bool {
	return hasDuplicate(slices.Compact(slices.Clone(xs)))
}

func TestShapeOfBadGenerator(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		a := pbt.IntRange(-100, 100).Draw(t, "a")
		b := a + pbt.IntRange(1, 100).Draw(t, "d")
		n := pbt.IntRange(1, 10).Draw(t, "n")
		xs := append(slices.Repeat([]int{a}, n), b)
		t.Classify(hasGap(xs), "gap between duplicates")
	}, pbt.Verbose(), pbt.Checks(1000), pbt.NoSave(), pbt.NoReplay())
}

func TestShapeOfGoodGenerator(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		xs := pbt.SliceOf(pbt.IntRange(0, 3)).Draw(t, "xs")
		t.Classify(hasGap(xs), "gap between duplicates")
	}, pbt.Verbose(), pbt.Checks(1000), pbt.NoSave(), pbt.NoReplay())
}

func TestDedupRelationsAndFirstOrder(t *testing.T) {
	pbt.Check(t, relationFirst(Dedup))
}
