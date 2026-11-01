// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package ch03

import (
	"slices"
	"testing"

	"go.ymotongpoo.com/pbt"
)

func TestRelations12(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		xs := pbt.SliceOf(pbt.IntRange(0, 3)).Draw(t, "xs")
		got := Dedup(xs)

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

		// 関係4：結果の並びは、入力での最初の出現位置の順である
		for i := 1; i < len(got); i++ {
			if slices.Index(xs, got[i-1]) >= slices.Index(xs, got[i]) {
				t.Fatalf("Dedup(%v) = %v: %d is out of order", xs, got, got[i])
			}
		}
	})
}

type exampleCase struct{ in, want []int }

var cases = []exampleCase{
	{[]int{1, 1, 2}, []int{1, 2}},
	{[]int{3, 1, 3}, []int{3, 1}},
	{[]int{}, []int{}},
}

func TestWrapped(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		c := pbt.SampledFrom(cases).Draw(t, "c")
		if got := Dedup(c.in); !slices.Equal(got, c.want) {
			t.Fatalf("Dedup(%v) = %v, want %v", c.in, got, c.want)
		}
	})
}

func TestValuesOnly(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		a := pbt.IntRange(-100, 100).Draw(t, "a")
		b := a + pbt.IntRange(1, 100).Draw(t, "d") // a と b は必ず異なる

		if got, want := Dedup([]int{a, a, b}), []int{a, b}; !slices.Equal(got, want) {
			t.Fatalf("Dedup(%v) = %v, want %v", []int{a, a, b}, got, want)
		}
		if got, want := Dedup([]int{b, a, b}), []int{b, a}; !slices.Equal(got, want) {
			t.Fatalf("Dedup(%v) = %v, want %v", []int{b, a, b}, got, want)
		}
	})
}

func TestBadOnlyAdjacent(t *testing.T) {
	// 悪い例：例 {1, 1, 2} の形だけを一般化している
	pbt.Check(t, func(t *pbt.T) {
		a := pbt.IntRange(-100, 100).Draw(t, "a")
		b := a + pbt.IntRange(1, 100).Draw(t, "d")
		n := pbt.IntRange(1, 10).Draw(t, "n")

		xs := append(slices.Repeat([]int{a}, n), b)
		if got, want := Dedup(xs), []int{a, b}; !slices.Equal(got, want) {
			t.Fatalf("Dedup(%v) = %v, want %v", xs, got, want)
		}
	})
}

func TestGeneratorSnippets(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		xs := pbt.SliceOf(pbt.Int()).Draw(t, "xs") // まずはそのまま
		_ = xs
	})
	pbt.Check(t, func(t *pbt.T) {
		// 値を 0..3 に絞ることで、重複が頻繁に起きる生成器にする
		xs := pbt.SliceOf(pbt.IntRange(0, 3)).Draw(t, "xs")
		t.Classify(hasDuplicate(xs), "has duplicate")
	})
}
