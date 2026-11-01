// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package ch07

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
	"testing"
	"unicode/utf8"

	"go.ymotongpoo.com/pbt"
	"go.ymotongpoo.com/pbt/examples/discovery/internal/mutant"
)

var opt = []pbt.Option{pbt.NoSave(), pbt.NoReplay()}

// ---- 7.2 IndexSorted

func IndexSorted(xs []int, v int) int {
	lo, hi := 0, len(xs)
	for lo < hi {
		mid := (lo + hi) / 2
		if xs[mid] < v {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	if lo < len(xs) && xs[lo] == v {
		return lo
	}
	return -1
}

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
		return slices.Compact(xs) // ソート済みなので、重複は隣り合って潰せる
	},
)

func indexProp(f func([]int, int) int) pbt.Property {
	return func(t *pbt.T) {
		xs := sortedDistinct.Draw(t, "xs")
		v := pbt.IntRange(0, 30).Draw(t, "v")

		t.Classify(slices.Contains(xs, v), "present")
		if got, want := f(xs, v), indexLinear(xs, v); got != want {
			t.Fatalf("IndexSorted(%v, %d) = %d, model says %d", xs, v, got, want)
		}
	}
}

func TestIndexSorted(t *testing.T) { pbt.Check(t, indexProp(IndexSorted)) }

// 変異1：比較を <= にする
func indexLE(xs []int, v int) int {
	lo, hi := 0, len(xs)
	for lo < hi {
		mid := (lo + hi) / 2
		if xs[mid] <= v {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	if lo < len(xs) && xs[lo] == v {
		return lo
	}
	return -1
}

// 変異2：終了後の範囲検査を忘れる
func indexNoBound(xs []int, v int) int {
	lo, hi := 0, len(xs)
	for lo < hi {
		mid := (lo + hi) / 2
		if xs[mid] < v {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	if xs[lo] == v {
		return lo
	}
	return -1
}

// 変異3：見つからなかったことを確かめず lo を返す
func indexNoCheck(xs []int, v int) int {
	lo, hi := 0, len(xs)
	for lo < hi {
		mid := (lo + hi) / 2
		if xs[mid] < v {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	return lo
}

// 変異4：hi の初期値を len(xs)-1 にする（結果は変わらない）
func indexHiLast(xs []int, v int) int {
	lo, hi := 0, len(xs)-1
	for lo < hi {
		mid := (lo + hi) / 2
		if xs[mid] < v {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	if lo < len(xs) && xs[lo] == v {
		return lo
	}
	return -1
}

func TestIndexLEMutant(t *testing.T) { mutant.Only(t); pbt.Check(t, indexProp(indexLE), opt...) }
func TestIndexNoBoundMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, indexProp(indexNoBound), opt...)
}
func TestIndexNoCheckMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, indexProp(indexNoCheck), opt...)
}
func TestIndexHiLastSurvives(t *testing.T) {
	pbt.Check(t, indexProp(indexHiLast), append(opt, pbt.Checks(5000))...)
}

// 存在する値だけを探す生成器だと変異3を見逃す
func TestIndexNoCheckExistingOnlySurvives(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		xs := pbt.Filter(sortedDistinct, func(xs []int) bool { return len(xs) > 0 }).Draw(t, "xs")
		v := pbt.SampledFrom(xs).Draw(t, "v")
		if got, want := indexNoCheck(xs, v), indexLinear(xs, v); got != want {
			t.Fatalf("IndexSorted(%v, %d) = %d, model says %d", xs, v, got, want)
		}
	}, opt...)
}

// 重複を許す場合：見つけた時点で返す二分探索も仕様上は正しい
func indexEarly(xs []int, v int) int {
	lo, hi := 0, len(xs)
	for lo < hi {
		mid := (lo + hi) / 2
		switch {
		case xs[mid] == v:
			return mid
		case xs[mid] < v:
			lo = mid + 1
		default:
			hi = mid
		}
	}
	return -1
}

var sortedDup = pbt.Map(pbt.SliceOf(pbt.IntRange(0, 5)), func(xs []int) []int {
	slices.Sort(xs)
	return xs
})

func TestIndexSortedDupAgreesWithModel(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		xs := sortedDup.Draw(t, "xs")
		v := pbt.IntRange(0, 5).Draw(t, "v")
		if got, want := IndexSorted(xs, v), indexLinear(xs, v); got != want {
			t.Fatalf("IndexSorted(%v, %d) = %d, model says %d", xs, v, got, want)
		}
	})
}

func TestIndexEarlyDupMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, func(t *pbt.T) {
		xs := sortedDup.Draw(t, "xs")
		v := pbt.IntRange(0, 5).Draw(t, "v")
		if got, want := indexEarly(xs, v), indexLinear(xs, v); got != want {
			t.Fatalf("indexEarly(%v, %d) = %d, model says %d", xs, v, got, want)
		}
	}, opt...)
}

// 観測の形にした主張：見つからないときだけ -1、見つかれば位置の要素が v
func TestIndexEarlyObservation(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		xs := sortedDup.Draw(t, "xs")
		v := pbt.IntRange(0, 5).Draw(t, "v")
		got := indexEarly(xs, v)
		if got == -1 {
			if slices.Contains(xs, v) {
				t.Fatalf("indexEarly(%v, %d) = -1, but %d is present", xs, v, v)
			}
			return
		}
		if xs[got] != v {
			t.Fatalf("indexEarly(%v, %d) = %d, which holds %d", xs, v, got, xs[got])
		}
	})
}

// slices.BinarySearch との比較
func TestIndexStdlib(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		xs := sortedDistinct.Draw(t, "xs")
		v := pbt.IntRange(0, 30).Draw(t, "v")
		want := -1
		if i, ok := slices.BinarySearch(xs, v); ok {
			want = i
		}
		if got := IndexSorted(xs, v); got != want {
			t.Fatalf("IndexSorted(%v, %d) = %d, BinarySearch says %d", xs, v, got, want)
		}
	})
}

// ---- 7.4 標準ライブラリをオラクルにする

var (
	errSyntax = errors.New("parse: not a decimal number")
	errRange  = errors.New("parse: value out of range")
)

// ParseUint64 は10進の数字列を uint64 にする。
func ParseUint64(s string) (uint64, error) {
	if s == "" {
		return 0, errSyntax
	}
	var n uint64
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return 0, errSyntax
		}
		d := uint64(c - '0')
		if n > (math.MaxUint64-d)/10 {
			return 0, errRange
		}
		n = n*10 + d
	}
	return n, nil
}

// 変異1：オーバーフローの検査を忘れる
func parseNoCheck(s string) (uint64, error) {
	if s == "" {
		return 0, errSyntax
	}
	var n uint64
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return 0, errSyntax
		}
		n = n*10 + uint64(c-'0')
	}
	return n, nil
}

// 変異2：検査の式から d を落とす
func parseOffByDigit(s string) (uint64, error) {
	if s == "" {
		return 0, errSyntax
	}
	var n uint64
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return 0, errSyntax
		}
		d := uint64(c - '0')
		if n > math.MaxUint64/10 {
			return 0, errRange
		}
		n = n*10 + d
	}
	return n, nil
}

func parseProp(parse func(string) (uint64, error), gen pbt.Gen[string]) pbt.Property {
	return func(t *pbt.T) {
		s := gen.Draw(t, "s")
		got, err := parse(s)
		want, werr := strconv.ParseUint(s, 10, 64)

		t.Classify(errors.Is(werr, strconv.ErrRange), "out of range")
		if (err == nil) != (werr == nil) || (err == nil && got != want) {
			t.Fatalf("parse(%q) = %d, %v; strconv says %d, %v", s, got, err, want, werr)
		}
	}
}

var (
	// 数値を整形した文字列
	formatted = pbt.Map(pbt.Uint64(), func(n uint64) string {
		return strconv.FormatUint(n, 10)
	})
	// 桁数を指定した数字列
	digits = pbt.StringOfN(pbt.SampledFrom([]rune("0123456789")), 1, 25)
	// 上限 18446744073709551615 の近くを狙う数字列
	nearMax = pbt.Map(pbt.IntRange(0, 99), func(n int) string {
		return fmt.Sprintf("184467440737095516%02d", n)
	})
	// 数字以外も混ぜた文字列
	mixed = pbt.StringOf(pbt.SampledFrom([]rune("0123456789+-_ a")))
)

func TestParseFormatted(t *testing.T) { pbt.Check(t, parseProp(ParseUint64, formatted)) }
func TestParseDigits(t *testing.T)    { pbt.Check(t, parseProp(ParseUint64, digits)) }
func TestParseNearMax(t *testing.T)   { pbt.Check(t, parseProp(ParseUint64, nearMax)) }
func TestParseMixed(t *testing.T)     { pbt.Check(t, parseProp(ParseUint64, mixed)) }

func TestNoCheckFormattedSurvives(t *testing.T) {
	pbt.Check(t, parseProp(parseNoCheck, formatted), append(opt, pbt.Checks(5000))...)
}
func TestNoCheckDigitsMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, parseProp(parseNoCheck, digits), opt...)
}
func TestNoCheckNearMaxMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, parseProp(parseNoCheck, nearMax), opt...)
}
func TestOffByDigitFormattedSurvives(t *testing.T) {
	pbt.Check(t, parseProp(parseOffByDigit, formatted), append(opt, pbt.Checks(5000))...)
}
func TestOffByDigitDigitsSurvives(t *testing.T) {
	pbt.Check(t, parseProp(parseOffByDigit, digits), append(opt, pbt.Checks(5000))...)
}
func TestOffByDigitNearMaxMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, parseProp(parseOffByDigit, nearMax), opt...)
}

// ---- 7.5 同じ発想で書いたモデル

// Reverse は s を文字単位で逆順にする。
func Reverse(s string) string {
	b := []byte(s)
	slices.Reverse(b)
	return string(b)
}

// モデル：末尾から1バイトずつ足す
func reverseModel(s string) string {
	var out []byte
	for i := len(s) - 1; i >= 0; i-- {
		out = append(out, s[i])
	}
	return string(out)
}

func TestReverseModelSurvives(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		s := pbt.String().Draw(t, "s")
		t.Classify(len(s) != utf8.RuneCountInString(s), "multibyte")
		if got, want := Reverse(s), reverseModel(s); got != want {
			t.Fatalf("Reverse(%q) = %q, model says %q", s, got, want)
		}
	}, opt...)
}

func TestReverseTwiceSurvives(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		s := pbt.String().Draw(t, "s")
		if got := Reverse(Reverse(s)); got != s {
			t.Fatalf("Reverse(Reverse(%q)) = %q", s, got)
		}
	}, opt...)
}

func validUTF8(rev func(string) string) pbt.Property {
	return func(t *pbt.T) {
		s := pbt.String().Draw(t, "s")
		if got := rev(s); !utf8.ValidString(got) {
			t.Fatalf("Reverse(%q) = %q, which is not valid UTF-8", s, got)
		}
	}
}

func TestReverseValidUTF8Mutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, validUTF8(Reverse), opt...)
}

// 直した実装
func ReverseRunes(s string) string {
	r := []rune(s)
	slices.Reverse(r)
	return string(r)
}

// 別の作りのモデル：先頭から1文字ずつ前に足す
func reverseRunesModel(s string) string {
	out := ""
	for _, r := range s {
		out = string(r) + out
	}
	return out
}

func TestParseUpperBoundExample(t *testing.T) {
	want, _ := strconv.ParseUint("18446744073709551615", 10, 64)
	if got, err := ParseUint64("18446744073709551615"); err != nil || got != want {
		t.Fatalf("ParseUint64 = %d, %v", got, err)
	}
}

func TestReverseRunes(t *testing.T) {
	pbt.Check(t, validUTF8(ReverseRunes))
	pbt.Check(t, func(t *pbt.T) {
		s := pbt.String().Draw(t, "s")
		if got, want := ReverseRunes(s), reverseRunesModel(s); got != want {
			t.Fatalf("ReverseRunes(%q) = %q, model says %q", s, got, want)
		}
	})
}

func TestReverseExampleMutant(t *testing.T) {
	mutant.Only(t)
	if got := Reverse("あい"); got != "いあ" {
		t.Fatalf("Reverse = %q", got)
	}
}
