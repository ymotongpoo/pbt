// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package ch08

import (
	"cmp"
	"maps"
	"math"
	"slices"
	"strings"
	"testing"

	"go.ymotongpoo.com/pbt"
	"go.ymotongpoo.com/pbt/examples/discovery/internal/mutant"
)

var opt = []pbt.Option{pbt.NoSave(), pbt.NoReplay()}

// ---- 8.2 中央値

// Median は xs の中央値を返す。xs は非空とし、要素数が偶数のときは
// 中央2つのうち小さい方を返すものとする。
func Median(xs []int) int {
	s := slices.Clone(xs)
	slices.Sort(s)
	return s[(len(s)-1)/2]
}

// floorMean は床関数で丸めた平均を返す（中央値のつもりで書かれた誤実装）。
func floorMean(xs []int) int {
	sum := 0
	for _, x := range xs {
		sum += x
	}
	return int(math.Floor(float64(sum) / float64(len(xs))))
}

// minImpl は常に最小値を返す誤実装である。
func minImpl(xs []int) int { return slices.Min(xs) }

// offsetImpl は常に1大きい値を返す誤実装である。
func offsetImpl(xs []int) int { return Median(xs) + 1 }

type impl func([]int) int

var genXS = pbt.SliceOfN(pbt.IntRange(-100, 100), 1, -1)

func mapInts(xs []int, f func(int) int) []int {
	out := make([]int, len(xs))
	for i, x := range xs {
		out[i] = f(x)
	}
	return out
}

func relPermute(f impl) pbt.Property {
	return func(t *pbt.T) {
		xs := genXS.Draw(t, "xs")
		ys := pbt.Permutation(xs).Draw(t, "ys")
		if f(xs) != f(ys) {
			t.Fatalf("f(%v) != f(%v)", xs, ys)
		}
	}
}

func relShift(f impl) pbt.Property {
	return func(t *pbt.T) {
		xs := genXS.Draw(t, "xs")
		c := pbt.IntRange(-50, 50).Draw(t, "c")

		shifted := make([]int, len(xs))
		for i, x := range xs {
			shifted[i] = x + c
		}
		if got, want := f(shifted), f(xs)+c; got != want {
			t.Fatalf("Median(shifted) = %d, want %d (base %v, shift %d)", got, want, xs, c)
		}
	}
}

func relScale(f impl) pbt.Property {
	return func(t *pbt.T) {
		xs := genXS.Draw(t, "xs")
		k := pbt.IntRange(1, 10).Draw(t, "k")
		ys := mapInts(xs, func(x int) int { return x * k })
		if got, want := f(ys), f(xs)*k; got != want {
			t.Fatalf("f(scaled) = %d, want %d (base %v, k %d)", got, want, xs, k)
		}
	}
}

func relAddLarge(f impl) pbt.Property {
	return func(t *pbt.T) {
		xs := genXS.Draw(t, "xs")
		m := f(xs)
		v := pbt.IntRange(m, m+200).Draw(t, "v")
		if got := f(append(slices.Clone(xs), v)); got < m {
			t.Fatalf("f(%v + %d) = %d, below %d", xs, v, got, m)
		}
	}
}

func relDouble(f impl) pbt.Property {
	return func(t *pbt.T) {
		xs := genXS.Draw(t, "xs")
		if got, want := f(slices.Concat(xs, xs)), f(xs); got != want {
			t.Fatalf("f(doubled) = %d, want %d (base %v)", got, want, xs)
		}
	}
}

func relOutlier(f impl) pbt.Property {
	return func(t *pbt.T) {
		xs := pbt.SliceOfN(pbt.IntRange(-100, 100), 2, -1).Draw(t, "xs")
		c := pbt.IntRange(1, 1000).Draw(t, "c")

		ys := slices.Clone(xs)
		ys[slices.Index(ys, slices.Max(ys))] += c
		if got, want := f(ys), f(xs); got != want {
			t.Fatalf("f(%v) = %d, but f(%v) = %d", ys, got, xs, want)
		}
	}
}

func relNegateOdd(f impl) pbt.Property {
	return func(t *pbt.T) {
		xs := genXS.Draw(t, "xs")
		t.Assume(len(xs)%2 == 1)
		ys := mapInts(xs, func(x int) int { return -x })
		if got, want := f(ys), -f(xs); got != want {
			t.Fatalf("f(negated) = %d, want %d (base %v)", got, want, xs)
		}
	}
}

var relations = []struct {
	name string
	prop func(impl) pbt.Property
}{
	{"permute", relPermute},
	{"shift", relShift},
	{"scale", relScale},
	{"addLarge", relAddLarge},
	{"double", relDouble},
	{"outlier", relOutlier},
	{"negateOdd", relNegateOdd},
}

func TestMedianRelationsHold(t *testing.T) {
	for _, r := range relations {
		t.Run(r.name, func(t *testing.T) { pbt.Check(t, r.prop(Median), pbt.Checks(1000)) })
	}
}

func TestMedianEvenNegationMutant(t *testing.T) {
	mutant.Only(t)
	xs, ys := []int{1, 2}, []int{-1, -2}
	if Median(ys) != -Median(xs) {
		t.Fatalf("Median(%v) = %d, Median(%v) = %d", xs, Median(xs), ys, Median(ys))
	}
}

func TestMatrixMeanMutant(t *testing.T) {
	mutant.Only(t)
	for _, r := range relations {
		t.Run(r.name, func(t *testing.T) { pbt.Check(t, r.prop(floorMean), append(opt, pbt.Checks(1000))...) })
	}
}
func TestMatrixMinMutant(t *testing.T) {
	mutant.Only(t)
	for _, r := range relations {
		t.Run(r.name, func(t *testing.T) { pbt.Check(t, r.prop(minImpl), append(opt, pbt.Checks(1000))...) })
	}
}
func TestMatrixOffsetMutant(t *testing.T) {
	mutant.Only(t)
	for _, r := range relations {
		t.Run(r.name, func(t *testing.T) { pbt.Check(t, r.prop(offsetImpl), append(opt, pbt.Checks(1000))...) })
	}
}

func TestMedianExample(t *testing.T) {
	if got := Median([]int{1, 2, 9}); got != 2 {
		t.Fatalf("Median = %d", got)
	}
}

func TestMedianExampleSeparatesImpls(t *testing.T) {
	for _, f := range []impl{floorMean, minImpl, offsetImpl} {
		if f([]int{1, 2, 9}) == 2 {
			t.Fatalf("example does not separate %v", f([]int{1, 2, 9}))
		}
	}
	if floorMean([]int{3, 1, 2}) != 2 {
		t.Fatal("mean of 3,1,2 is 2")
	}
}

// ---- 8.4 検索

type Doc struct {
	ID   int
	Text string
}

// Search は query を含む文書の ID を、出現回数の多い順に最大 limit 件返す。
// 出現回数が同じ文書は ID の小さい順に並べる。
func Search(docs []Doc, query string, limit int) []int {
	type hit struct{ id, score int }
	var hits []hit
	for _, d := range docs {
		if c := strings.Count(d.Text, query); c > 0 {
			hits = append(hits, hit{d.ID, c})
		}
	}
	slices.SortFunc(hits, func(a, b hit) int {
		return cmp.Or(cmp.Compare(b.score, a.score), cmp.Compare(a.id, b.id))
	})
	ids := []int{}
	for _, h := range hits[:min(limit, len(hits))] {
		ids = append(ids, h.id)
	}
	return ids
}

type searcher func([]Doc, string, int) []int

// 変異1：上限で打ち切ってから並べる
func searchCutFirst(docs []Doc, query string, limit int) []int {
	type hit struct{ id, score int }
	var hits []hit
	for _, d := range docs {
		if len(hits) == limit {
			break
		}
		if c := strings.Count(d.Text, query); c > 0 {
			hits = append(hits, hit{d.ID, c})
		}
	}
	slices.SortFunc(hits, func(a, b hit) int {
		return cmp.Or(cmp.Compare(b.score, a.score), cmp.Compare(a.id, b.id))
	})
	ids := []int{}
	for _, h := range hits {
		ids = append(ids, h.id)
	}
	return ids
}

// 変異2：スコアを文書全体の平均の長さで補正する。式はBM25の形にする。
// c/(c+L/avg) のような形では avg を変えても順位が変わらないので、b を使う。
func bm25(c, length int, avg float64) float64 {
	const k1, b = 1.2, 0.75
	return float64(c) * (k1 + 1) / (float64(c) + k1*(1-b+b*float64(length)/avg))
}

func searchAvgLen(docs []Doc, query string, limit int) []int {
	total := 0
	for _, d := range docs {
		total += len(d.Text)
	}
	avg := float64(total) / float64(len(docs))
	type hit struct {
		id    int
		score float64
	}
	var hits []hit
	for _, d := range docs {
		if c := strings.Count(d.Text, query); c > 0 {
			hits = append(hits, hit{d.ID, bm25(c, len(d.Text), avg)})
		}
	}
	slices.SortFunc(hits, func(a, b hit) int {
		return cmp.Or(cmp.Compare(b.score, a.score), cmp.Compare(a.id, b.id))
	})
	ids := []int{}
	for _, h := range hits[:min(limit, len(hits))] {
		ids = append(ids, h.id)
	}
	return ids
}

// 変異3：同点の順序を入力の順に任せる
func searchNoTieBreak(docs []Doc, query string, limit int) []int {
	type hit struct{ id, score int }
	var hits []hit
	for _, d := range docs {
		if c := strings.Count(d.Text, query); c > 0 {
			hits = append(hits, hit{d.ID, c})
		}
	}
	slices.SortStableFunc(hits, func(a, b hit) int { return cmp.Compare(b.score, a.score) })
	ids := []int{}
	for _, h := range hits[:min(limit, len(hits))] {
		ids = append(ids, h.id)
	}
	return ids
}

const query = "a"

var docsGen = pbt.Custom("docs", func(t *pbt.T) []Doc {
	texts := pbt.SliceOf(pbt.StringOf(pbt.SampledFrom([]rune("ab ")))).Draw(t, "texts")
	docs := make([]Doc, len(texts))
	for i, text := range texts {
		docs[i] = Doc{ID: i, Text: text}
	}
	return docs
})

// 検索語を含まない本文
var noQueryText = pbt.StringOf(pbt.SampledFrom([]rune("bc ")))

func relIrrelevant(search searcher) pbt.Property {
	return func(t *pbt.T) {
		docs := docsGen.Draw(t, "docs")
		extra := Doc{ID: len(docs), Text: noQueryText.Draw(t, "extra")}
		limit := pbt.IntRange(1, 10).Draw(t, "limit")

		got := search(append(slices.Clone(docs), extra), query, limit)
		if want := search(docs, query, limit); !slices.Equal(got, want) {
			t.Fatalf("adding %+v changed the result from %v to %v", extra, want, got)
		}
	}
}

func relLimitPrefix(search searcher) pbt.Property {
	return func(t *pbt.T) {
		docs := docsGen.Draw(t, "docs")
		limit := pbt.IntRange(0, 10).Draw(t, "limit")
		more := pbt.IntRange(1, 10).Draw(t, "more")

		short := search(docs, query, limit)
		long := search(docs, query, limit+more)
		if !slices.Equal(long[:len(short)], short) {
			t.Fatalf("limit %d gave %v, limit %d gave %v", limit, short, limit+more, long)
		}
	}
}

func relDocOrder(search searcher) pbt.Property {
	return func(t *pbt.T) {
		docs := docsGen.Draw(t, "docs")
		shuffled := pbt.Permutation(docs).Draw(t, "shuffled")
		limit := pbt.IntRange(0, 10).Draw(t, "limit")

		if got, want := search(shuffled, query, limit), search(docs, query, limit); !slices.Equal(got, want) {
			t.Fatalf("shuffling changed the result from %v to %v", want, got)
		}
	}
}

var searchRelations = []struct {
	name string
	prop func(searcher) pbt.Property
}{
	{"irrelevant", relIrrelevant},
	{"limitPrefix", relLimitPrefix},
	{"docOrder", relDocOrder},
}

func TestSearchRelationsHold(t *testing.T) {
	for _, r := range searchRelations {
		t.Run(r.name, func(t *testing.T) { pbt.Check(t, r.prop(Search), pbt.Checks(1000)) })
	}
}
func TestSearchCutFirstMutant(t *testing.T) {
	mutant.Only(t)
	for _, r := range searchRelations {
		t.Run(r.name, func(t *testing.T) { pbt.Check(t, r.prop(searchCutFirst), append(opt, pbt.Checks(1000))...) })
	}
}
func TestSearchAvgLenMutant(t *testing.T) {
	mutant.Only(t)
	for _, r := range searchRelations {
		t.Run(r.name, func(t *testing.T) { pbt.Check(t, r.prop(searchAvgLen), append(opt, pbt.Checks(1000))...) })
	}
}
func TestSearchNoTieBreakMutant(t *testing.T) {
	mutant.Only(t)
	for _, r := range searchRelations {
		t.Run(r.name, func(t *testing.T) { pbt.Check(t, r.prop(searchNoTieBreak), append(opt, pbt.Checks(1000))...) })
	}
}

func TestSearchExample(t *testing.T) {
	docs := []Doc{{1, "a b"}, {2, "a a"}, {3, "b"}, {4, "a"}}
	if got := Search(docs, "a", 2); !slices.Equal(got, []int{2, 1}) {
		t.Fatalf("Search = %v", got)
	}
}

// ---- 8.5 集計

// CountByKey はキーごとの出現回数を数える。
func CountByKey(keys []string) map[string]int {
	counts := map[string]int{}
	for _, k := range keys {
		counts[k]++
	}
	return counts
}

// MergeCounts は2つの集計を合算する。
func MergeCounts(a, b map[string]int) map[string]int {
	out := maps.Clone(a)
	for k, v := range b {
		out[k] += v
	}
	return out
}

// 変異1：加算ではなく上書きする
func mergeOverwrite(a, b map[string]int) map[string]int {
	out := maps.Clone(a)
	for k, v := range b {
		out[k] = v
	}
	return out
}

// 変異2：a にないキーを捨てる
func mergeLeftOnly(a, b map[string]int) map[string]int {
	out := maps.Clone(a)
	for k, v := range b {
		if _, ok := out[k]; ok {
			out[k] += v
		}
	}
	return out
}

// 変異3：空文字列のキーを数えない
func countSkipEmpty(keys []string) map[string]int {
	counts := map[string]int{}
	for _, k := range keys {
		if k != "" {
			counts[k]++
		}
	}
	return counts
}

var keysGen = pbt.SliceOf(pbt.SampledFrom([]string{"", "a", "b", "c"}))

type counter func([]string) map[string]int
type merger func(a, b map[string]int) map[string]int

func relSplit(count counter, merge merger) pbt.Property {
	return func(t *pbt.T) {
		keys := keysGen.Draw(t, "keys")
		i := pbt.IntRange(0, len(keys)).Draw(t, "i")

		got := merge(count(keys[:i]), count(keys[i:]))
		if want := count(keys); !maps.Equal(got, want) {
			t.Fatalf("split at %d of %q: got %v, want %v", i, keys, got, want)
		}
	}
}

func relTotal(count counter) pbt.Property {
	return func(t *pbt.T) {
		keys := keysGen.Draw(t, "keys")
		total := 0
		for _, n := range count(keys) {
			total += n
		}
		if total != len(keys) {
			t.Fatalf("counts of %q add up to %d", keys, total)
		}
	}
}

func TestSplitHolds(t *testing.T) { pbt.Check(t, relSplit(CountByKey, MergeCounts)) }
func TestTotalHolds(t *testing.T) { pbt.Check(t, relTotal(CountByKey)) }
func TestSplitOverwriteMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, relSplit(CountByKey, mergeOverwrite), opt...)
}
func TestSplitLeftOnlyMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, relSplit(CountByKey, mergeLeftOnly), opt...)
}
func TestSplitSkipEmptySurvives(t *testing.T) {
	pbt.Check(t, relSplit(countSkipEmpty, MergeCounts), append(opt, pbt.Checks(5000))...)
}
func TestTotalSkipEmptyMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, relTotal(countSkipEmpty), opt...)
}

// BM25 の形では平均の長さが変わると順位が実際に入れ替わる（丸め誤差ではない）。
func TestBM25OrderDependsOnAverage(t *testing.T) {
	// 文書A：短く1回。文書B：長く2回。
	a, b := func(avg float64) float64 { return bm25(1, 2, avg) }, func(avg float64) float64 { return bm25(2, 8, avg) }
	if !(a(2) > b(2)+0.01 && b(20) > a(20)+0.01) {
		t.Fatalf("avg=2: %f %f, avg=20: %f %f", a(2), b(2), a(20), b(20))
	}
}
