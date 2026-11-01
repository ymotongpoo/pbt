// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package ch05

import (
	"fmt"
	"slices"
	"testing"

	"go.ymotongpoo.com/pbt"
	"go.ymotongpoo.com/pbt/examples/discovery/internal/mutant"
)

// Union は a と b の和集合を、ソート済み、重複なしで返す。
func Union(a, b []int) []int {
	seen := map[int]bool{}
	var out []int
	for _, xs := range [][]int{a, b} {
		for _, x := range xs {
			if !seen[x] {
				seen[x] = true
				out = append(out, x)
			}
		}
	}
	slices.Sort(out)
	return out
}

// set は、値の重なりが頻繁に出る整数列の生成器である。
var set = pbt.SliceOf(pbt.IntRange(0, 10))

// normalize は xs の正規化された形を、Union を使わずに作る。
func normalize(xs []int) []int {
	out := slices.Clone(xs)
	slices.Sort(out)
	return slices.Compact(out)
}

type unionFunc func(a, b []int) []int

func commutative(union unionFunc) pbt.Property {
	return func(t *pbt.T) {
		a := set.Draw(t, "a")
		b := set.Draw(t, "b")

		if got, want := union(a, b), union(b, a); !slices.Equal(got, want) {
			t.Fatalf("Union(%v, %v) = %v, but swapped = %v", a, b, got, want)
		}
	}
}

func associative(union unionFunc) pbt.Property {
	return func(t *pbt.T) {
		a := set.Draw(t, "a")
		b := set.Draw(t, "b")
		c := set.Draw(t, "c")

		left, right := union(union(a, b), c), union(a, union(b, c))
		if !slices.Equal(left, right) {
			t.Fatalf("(%v, %v, %v): left = %v, right = %v", a, b, c, left, right)
		}
	}
}

func idempotent(union unionFunc) pbt.Property {
	return func(t *pbt.T) {
		a := set.Draw(t, "a")

		if got, want := union(a, a), union(a, nil); !slices.Equal(got, want) {
			t.Fatalf("Union(a, a) = %v, Union(a, nil) = %v, a = %v", got, want, a)
		}
	}
}

func identity(union unionFunc) pbt.Property {
	return func(t *pbt.T) {
		a := set.Draw(t, "a")

		if got, want := union(a, nil), normalize(a); !slices.Equal(got, want) {
			t.Fatalf("Union(%v, nil) = %v, want %v", a, got, want)
		}
		if got, want := union(nil, a), normalize(a); !slices.Equal(got, want) {
			t.Fatalf("Union(nil, %v) = %v, want %v", a, got, want)
		}
	}
}

func monotonic(union unionFunc) pbt.Property {
	return func(t *pbt.T) {
		a := set.Draw(t, "a")
		b := set.Draw(t, "b")
		x := pbt.IntRange(0, 10).Draw(t, "x")

		before := union(a, b)
		after := union(append(slices.Clone(a), x), b)
		for _, e := range before {
			if !slices.Contains(after, e) {
				t.Fatalf("adding %d to %v lost %d (b = %v)", x, a, e, b)
			}
		}
	}
}

var laws = []struct {
	name string
	prop func(unionFunc) pbt.Property
}{
	{"Comm", commutative},
	{"Assoc", associative},
	{"Idem", idempotent},
	{"Id", identity},
	{"Mono", monotonic},
}

func TestUnionLaws(t *testing.T) {
	for _, l := range laws {
		t.Run(l.name, func(t *testing.T) { pbt.Check(t, l.prop(Union)) })
	}
}

// ---- 変異

// unionNoSort はソートを忘れる。
func unionNoSort(a, b []int) []int {
	seen := map[int]bool{}
	var out []int
	for _, xs := range [][]int{a, b} {
		for _, x := range xs {
			if !seen[x] {
				seen[x] = true
				out = append(out, x)
			}
		}
	}
	return out
}

// unionOneSide は b の側の重複除去を忘れる。
func unionOneSide(a, b []int) []int {
	seen := map[int]bool{}
	var out []int
	for _, x := range a {
		if !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	out = append(out, b...)
	slices.Sort(out)
	return out
}

// unionNoDedup は重複除去を両側とも忘れる。
func unionNoDedup(a, b []int) []int {
	out := append(slices.Clone(a), b...)
	slices.Sort(out)
	return out
}

// unionFirst は第2引数を無視する。
func unionFirst(a, _ []int) []int { return normalize(a) }

// unionIntersect は共通部分を返す。
func unionIntersect(a, b []int) []int {
	var out []int
	for _, x := range normalize(a) {
		if slices.Contains(b, x) {
			out = append(out, x)
		}
	}
	return out
}

// unionXor はどちらか一方だけにある要素を返す。
func unionXor(a, b []int) []int {
	var out []int
	for _, x := range normalize(append(slices.Clone(a), b...)) {
		if slices.Contains(a, x) != slices.Contains(b, x) {
			out = append(out, x)
		}
	}
	return out
}

// unionEmpty は常に空を返す。
func unionEmpty(_, _ []int) []int { return nil }

// unionCap は8より大きい値を黙って捨てる。
func unionCap(a, b []int) []int {
	return slices.DeleteFunc(Union(a, b), func(x int) bool { return x > 8 })
}

var mutants = []struct {
	name string
	f    unionFunc
}{
	{"NoSort", unionNoSort},
	{"OneSide", unionOneSide},
	{"NoDedup", unionNoDedup},
	{"First", unionFirst},
	{"Intersect", unionIntersect},
	{"Xor", unionXor},
	{"Empty", unionEmpty},
	{"Cap", unionCap},
}

func TestLawTableMutant(t *testing.T) {
	mutant.Only(t)
	for _, m := range mutants {
		for _, l := range laws {
			t.Run(m.name+"/"+l.name, func(t *testing.T) {
				pbt.Check(t, l.prop(m.f), pbt.NoSave(), pbt.NoReplay())
			})
		}
	}
}

// ---- 金額

// AddAmount は小数で持った金額を合算する。
func AddAmount(a, b float64) float64 { return a + b }

// amount は 0.00 から 100.00 までの、小数第2位までの金額を生成する。
var amount = pbt.Map(pbt.IntRange(0, 10000), func(c int) float64 {
	return float64(c) / 100
})

func TestFloatAssociativeMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, func(t *pbt.T) {
		a := amount.Draw(t, "a")
		b := amount.Draw(t, "b")
		c := amount.Draw(t, "c")

		left := AddAmount(AddAmount(a, b), c)
		right := AddAmount(a, AddAmount(b, c))
		if left != right {
			t.Fatalf("(%v + %v) + %v = %v, but %v + (%v + %v) = %v", a, b, c, left, a, b, c, right)
		}
	}, pbt.NoSave(), pbt.NoReplay())
}

// Cents は金額を最小単位の整数で表す。
type Cents int64

var cents = pbt.Map(pbt.IntRange(0, 100000), func(c int) Cents { return Cents(c) })

func TestCentsAssociative(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		a := cents.Draw(t, "a")
		b := cents.Draw(t, "b")
		c := cents.Draw(t, "c")

		if left, right := (a+b)+c, a+(b+c); left != right {
			t.Fatalf("(%d + %d) + %d = %d, but right = %d", a, b, c, left, right)
		}
	})
}

// Fee は amount の3.5%の手数料を、最小単位へ四捨五入して返す。
func Fee(amount Cents) Cents {
	return (amount*35 + 500) / 1000
}

func TestFeeAdditiveMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, func(t *pbt.T) {
		a := cents.Draw(t, "a")
		b := cents.Draw(t, "b")

		if got, want := Fee(a)+Fee(b), Fee(a+b); got != want {
			t.Fatalf("Fee(%d) + Fee(%d) = %d, but Fee(%d) = %d", a, b, got, a+b, want)
		}
	}, pbt.NoSave(), pbt.NoReplay())
}

func TestFeeWithinOneCent(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		a := cents.Draw(t, "a")
		b := cents.Draw(t, "b")

		diff := Fee(a) + Fee(b) - Fee(a+b)
		if diff < -1 || diff > 1 {
			t.Fatalf("Fee(%d) + Fee(%d) differs from Fee(%d) by %d", a, b, a+b, diff)
		}
	})
}

// 差の分布の測定
func TestFeeDiffDistribution(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		a := cents.Draw(t, "a")
		b := cents.Draw(t, "b")
		t.Collect(fmt.Sprint(Fee(a) + Fee(b) - Fee(a+b)))
	}, pbt.Verbose(), pbt.Checks(5000), pbt.NoSave(), pbt.NoReplay())
}

// ---- マージ

// Item はキーと、由来を表す札を持つ。
type Item struct {
	Key int
	Tag string
}

// Merge は Key でソート済みの a と b を1本にマージする。
// Key が等しいときは a の要素を先に置く。
func Merge(a, b []Item) []Item {
	out := make([]Item, 0, len(a)+len(b))
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		if b[j].Key < a[i].Key {
			out = append(out, b[j])
			j++
		} else {
			out = append(out, a[i])
			i++
		}
	}
	out = append(out, a[i:]...)
	return append(out, b[j:]...)
}

// mergeRightFirst は Key が等しいとき b の要素を先に置く変異である。
func mergeRightFirst(a, b []Item) []Item {
	out := make([]Item, 0, len(a)+len(b))
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		if b[j].Key <= a[i].Key {
			out = append(out, b[j])
			j++
		} else {
			out = append(out, a[i])
			i++
		}
	}
	out = append(out, a[i:]...)
	return append(out, b[j:]...)
}

// sortedItems は Key でソート済みの列を生成する。tag は由来の札である。
func sortedItems(tag string) pbt.Gen[[]Item] {
	return pbt.Map(pbt.SliceOf(pbt.IntRange(0, 5)), func(keys []int) []Item {
		slices.Sort(keys)
		items := make([]Item, len(keys))
		for i, k := range keys {
			items[i] = Item{Key: k, Tag: fmt.Sprintf("%s%d", tag, i)}
		}
		return items
	})
}

// keys は Item の列からキーだけを取り出す。
func keys(items []Item) []int {
	out := make([]int, len(items))
	for i, it := range items {
		out[i] = it.Key
	}
	return out
}

type mergeFunc func(a, b []Item) []Item

func mergeAssociative(merge mergeFunc) pbt.Property {
	return func(t *pbt.T) {
		a := sortedItems("a").Draw(t, "a")
		b := sortedItems("b").Draw(t, "b")
		c := sortedItems("c").Draw(t, "c")

		left, right := merge(merge(a, b), c), merge(a, merge(b, c))
		if !slices.Equal(left, right) {
			t.Fatalf("left = %v, right = %v", left, right)
		}
	}
}

func mergeIdentity(merge mergeFunc) pbt.Property {
	return func(t *pbt.T) {
		a := sortedItems("a").Draw(t, "a")

		if got := merge(a, nil); !slices.Equal(got, a) {
			t.Fatalf("Merge(%v, nil) = %v", a, got)
		}
		if got := merge(nil, a); !slices.Equal(got, a) {
			t.Fatalf("Merge(nil, %v) = %v", a, got)
		}
	}
}

func mergeCommutative(merge mergeFunc) pbt.Property {
	return func(t *pbt.T) {
		a := sortedItems("a").Draw(t, "a")
		b := sortedItems("b").Draw(t, "b")

		if got, want := merge(a, b), merge(b, a); !slices.Equal(got, want) {
			t.Fatalf("Merge(a, b) = %v, Merge(b, a) = %v", got, want)
		}
	}
}

func mergeKeysCommutative(merge mergeFunc) pbt.Property {
	return func(t *pbt.T) {
		a := sortedItems("a").Draw(t, "a")
		b := sortedItems("b").Draw(t, "b")

		got, want := keys(merge(a, b)), keys(merge(b, a))
		if !slices.Equal(got, want) {
			t.Fatalf("keys differ: %v, %v", got, want)
		}
	}
}

// 同じキーの中では a の要素が b の要素より先に来る。
func mergeStable(merge mergeFunc) pbt.Property {
	return func(t *pbt.T) {
		a := sortedItems("a").Draw(t, "a")
		b := sortedItems("b").Draw(t, "b")

		got := merge(a, b)
		for i := 1; i < len(got); i++ {
			p, q := got[i-1], got[i]
			if p.Key == q.Key && p.Tag[0] == 'b' && q.Tag[0] == 'a' {
				t.Fatalf("Merge(%v, %v) = %v: %v precedes %v", a, b, got, p, q)
			}
		}
	}
}

func TestMergeLaws(t *testing.T) {
	pbt.Check(t, mergeAssociative(Merge))
	pbt.Check(t, mergeIdentity(Merge))
	pbt.Check(t, mergeKeysCommutative(Merge))
	pbt.Check(t, mergeStable(Merge))
}

func TestMergeCommutativeMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, mergeCommutative(Merge), pbt.NoSave(), pbt.NoReplay())
}

// 左右の優先を逆にしても、結合性、単位元、キーの可換性は通る。
func TestMergeRightFirstSurvives(t *testing.T) {
	pbt.Check(t, mergeAssociative(mergeRightFirst), pbt.NoSave(), pbt.NoReplay())
	pbt.Check(t, mergeIdentity(mergeRightFirst), pbt.NoSave(), pbt.NoReplay())
	pbt.Check(t, mergeKeysCommutative(mergeRightFirst), pbt.NoSave(), pbt.NoReplay())
}

func TestMergeRightFirstCommutativeMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, mergeCommutative(mergeRightFirst), pbt.NoSave(), pbt.NoReplay())
}

func TestMergeRightFirstStableMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, mergeStable(mergeRightFirst), pbt.NoSave(), pbt.NoReplay())
}

// ---- 手数料の主張が何を捕まえるか

func feeWithinOneCent(fee func(Cents) Cents) pbt.Property {
	return func(t *pbt.T) {
		a := cents.Draw(t, "a")
		b := cents.Draw(t, "b")

		diff := fee(a) + fee(b) - fee(a+b)
		if diff < -1 || diff > 1 {
			t.Fatalf("Fee(%d) + Fee(%d) differs from Fee(%d) by %d", a, b, a+b, diff)
		}
	}
}

// feeFloor は四捨五入でなく切り捨てる。
func feeFloor(amount Cents) Cents { return amount * 35 / 1000 }

// feeTen は10セント単位に丸める。
func feeTen(amount Cents) Cents { return (amount*35 + 5000) / 10000 * 10 }

// feeFixed は固定額を上乗せする。
func feeFixed(amount Cents) Cents { return Fee(amount) + 30 }

// feeRate は料率を間違える。
func feeRate(amount Cents) Cents { return (amount*36 + 500) / 1000 }

func TestFeeFloorSurvives(t *testing.T) {
	pbt.Check(t, feeWithinOneCent(feeFloor), pbt.Checks(2000), pbt.NoSave(), pbt.NoReplay())
}

func TestFeeRateSurvives(t *testing.T) {
	pbt.Check(t, feeWithinOneCent(feeRate), pbt.Checks(2000), pbt.NoSave(), pbt.NoReplay())
}

func TestFeeTenMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, feeWithinOneCent(feeTen), pbt.NoSave(), pbt.NoReplay())
}

func TestFeeFixedMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, feeWithinOneCent(feeFixed), pbt.NoSave(), pbt.NoReplay())
}
