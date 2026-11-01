// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package ch09

import (
	"math"
	"slices"
	"strings"
	"testing"

	"go.ymotongpoo.com/pbt"
	"go.ymotongpoo.com/pbt/examples/discovery/internal/mutant"
)

var opt = []pbt.Option{pbt.NoSave(), pbt.NoReplay()}

const separators = " \t\n,."

// Tokenize は s を空白と句読点で分割し、空でないトークンの列を返す。
func Tokenize(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool {
		return strings.ContainsRune(separators, r)
	})
}

type tokenizer func(string) []string

// 変異：入力を無視する
func tokenizeConst(string) []string { return []string{"x"} }

// 変異：最後のトークンを落とす
func tokenizeDropLast(s string) []string {
	toks := Tokenize(s)
	if len(toks) == 0 {
		return toks
	}
	return toks[:len(toks)-1]
}

// 変異：タブを区切りとして扱わない
func tokenizeNoTab(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool {
		return strings.ContainsRune(" \n,.", r)
	})
}

// 変異：4バイト以上のトークンを2つに割る
func tokenizeSplitLong(s string) []string {
	var out []string
	for _, tok := range Tokenize(s) {
		if len(tok) >= 4 && strings.IndexFunc(tok, func(r rune) bool { return r >= 0x80 }) < 0 {
			out = append(out, tok[:len(tok)/2], tok[len(tok)/2:])
		} else {
			out = append(out, tok)
		}
	}
	return out
}

// 分割される側の文字を多めに出す生成器
var textGen = pbt.StringOf(pbt.SampledFrom([]rune("abc \t\n,.")))

// 構造：空のトークンがなく、区切りを含まない
func structure(f tokenizer) pbt.Property {
	return func(t *pbt.T) {
		s := textGen.Draw(t, "s")

		for _, tok := range f(s) {
			if tok == "" {
				t.Fatalf("Tokenize(%q) produced an empty token", s)
			}
			if strings.ContainsAny(tok, separators) {
				t.Fatalf("Tokenize(%q) produced %q, which contains a separator", s, tok)
			}
		}
	}
}

// 保存と上限：トークンの合計の長さは入力の長さ以下である
func upperBound(f tokenizer) pbt.Property {
	return func(t *pbt.T) {
		s := textGen.Draw(t, "s")

		total := 0
		for _, tok := range f(s) {
			total += len(tok)
		}
		if total > len(s) {
			t.Fatalf("Tokenize(%q) holds %d bytes", s, total)
		}
	}
}

// 関係の残り：トークンは入力の中に、この順で現れる
func inOrder(f tokenizer) pbt.Property {
	return func(t *pbt.T) {
		s := textGen.Draw(t, "s")

		rest := s
		for _, tok := range f(s) {
			i := strings.Index(rest, tok)
			if i < 0 {
				t.Fatalf("Tokenize(%q): %q does not follow in %q", s, tok, rest)
			}
			rest = rest[i+len(tok):]
		}
	}
}

// 保存の強い形：区切り以外のバイトはすべてトークンに残る
func conserved(f tokenizer) pbt.Property {
	return func(t *pbt.T) {
		s := textGen.Draw(t, "s")

		total := 0
		for _, tok := range f(s) {
			total += len(tok)
		}
		want := len(strings.Map(func(r rune) rune {
			if strings.ContainsRune(separators, r) {
				return -1
			}
			return r
		}, s))
		if total != want {
			t.Fatalf("Tokenize(%q) holds %d bytes, want %d", s, total, want)
		}
	}
}

// 仕様の文：区切りを含まない入力は、そのまま1つのトークンになる
func wholeWord(f tokenizer) pbt.Property {
	return func(t *pbt.T) {
		w := pbt.StringOfN(pbt.SampledFrom([]rune("abc")), 1, -1).Draw(t, "w")

		if got := f(w); !slices.Equal(got, []string{w}) {
			t.Fatalf("Tokenize(%q) = %q, want a single token", w, got)
		}
	}
}

var tokenProps = []struct {
	name string
	prop func(tokenizer) pbt.Property
}{
	{"structure", structure},
	{"upperBound", upperBound},
	{"inOrder", inOrder},
	{"conserved", conserved},
	{"wholeWord", wholeWord},
}

func TestTokenizeHolds(t *testing.T) {
	for _, p := range tokenProps {
		t.Run(p.name, func(t *testing.T) { pbt.Check(t, p.prop(Tokenize), pbt.Checks(1000)) })
	}
}
func TestTokenizeConstMutant(t *testing.T) {
	mutant.Only(t)
	for _, p := range tokenProps {
		t.Run(p.name, func(t *testing.T) { pbt.Check(t, p.prop(tokenizeConst), append(opt, pbt.Checks(1000))...) })
	}
}
func TestTokenizeDropLastMutant(t *testing.T) {
	mutant.Only(t)
	for _, p := range tokenProps {
		t.Run(p.name, func(t *testing.T) { pbt.Check(t, p.prop(tokenizeDropLast), append(opt, pbt.Checks(1000))...) })
	}
}
func TestTokenizeNoTabMutant(t *testing.T) {
	mutant.Only(t)
	for _, p := range tokenProps {
		t.Run(p.name, func(t *testing.T) { pbt.Check(t, p.prop(tokenizeNoTab), append(opt, pbt.Checks(1000))...) })
	}
}
func TestTokenizeSplitLongMutant(t *testing.T) {
	mutant.Only(t)
	for _, p := range tokenProps {
		t.Run(p.name, func(t *testing.T) { pbt.Check(t, p.prop(tokenizeSplitLong), append(opt, pbt.Checks(1000))...) })
	}
}

// pbt.String() では区切りが出るかの統計
func TestTokenizeGenStats(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		s := pbt.String().Draw(t, "s")
		t.Classify(strings.ContainsAny(s, separators), "has separator")
		t.Measure("tokens", float64(len(Tokenize(s))))
	})
	pbt.Check(t, func(t *pbt.T) {
		s := textGen.Draw(t, "s")
		t.Classify(strings.ContainsAny(s, separators), "has separator")
		t.Measure("tokens", float64(len(Tokenize(s))))
	})
}

func TestTokenizeBytes(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		b := pbt.Bytes().Draw(t, "b")
		_ = Tokenize(string(b)) // パニックすれば、このケースは失敗になる
	})
}

func TestTokenizeNoPanic(t *testing.T) { pbt.Check(t, pbt.NoPanic(Tokenize)) }

// ---- 9.4 数値

// Midpoint は a と b の中点を返す。
func Midpoint(a, b int) int {
	return (a + b) / 2
}

// MidpointSafe は和を計算せずに、a と b の中点を負の無限大の方向へ丸めて返す。
func MidpointSafe(a, b int) int {
	return (a & b) + (a^b)>>1
}

// 変異：差の半分を足す書き方（差がオーバーフローする）
func midpointDiff(a, b int) int {
	return a + (b-a)/2
}

func between(f func(a, b int) int, gen pbt.Gen[int]) pbt.Property {
	return func(t *pbt.T) {
		a := gen.Draw(t, "a")
		b := gen.Draw(t, "b")

		t.Classify(a > 0 && b > math.MaxInt-a || a < 0 && b < math.MinInt-a, "sum overflows")
		lo, hi := min(a, b), max(a, b)
		if m := f(a, b); m < lo || m > hi {
			t.Fatalf("Midpoint(%d, %d) = %d, outside [%d, %d]", a, b, m, lo, hi)
		}
	}
}

func TestMidpointSmallSurvives(t *testing.T) {
	pbt.Check(t, between(Midpoint, pbt.IntRange(-1000, 1000)), append(opt, pbt.Checks(5000))...)
}
func TestMidpointIntMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, between(Midpoint, pbt.Int()), opt...)
}
func TestMidpointFullMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, between(Midpoint, pbt.IntRange(math.MinInt, math.MaxInt)), opt...)
}
func TestMidpointSafe(t *testing.T) {
	pbt.Check(t, between(MidpointSafe, pbt.Int()), pbt.Checks(5000))
}
func TestMidpointDiffMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, between(midpointDiff, pbt.Int()), opt...)
}
func TestMidpointStats(t *testing.T) {
	pbt.Check(t, between(MidpointSafe, pbt.Int()))
	pbt.Check(t, between(MidpointSafe, pbt.IntRange(-1000, 1000)))
}

func TestTokenizeStructureString(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		s := pbt.String().Draw(t, "s")

		for _, tok := range Tokenize(s) {
			if tok == "" {
				t.Fatalf("Tokenize(%q) produced an empty token", s)
			}
			if strings.ContainsAny(tok, separators) {
				t.Fatalf("Tokenize(%q) produced %q, which contains a separator", s, tok)
			}
		}
	})
}

func TestMidpointIntDistribution(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		a := pbt.Int().Draw(t, "a")
		abs := math.Abs(float64(a))
		t.Classify(abs < 1e3, "|a| < 1e3")
		t.Classify(abs >= 1e3 && abs < 1e18, "1e3 <= |a| < 1e18")
		t.Classify(abs >= 1e18, "|a| >= 1e18")
		t.Classify(a == math.MaxInt || a == math.MinInt, "a is a bound")
	}, pbt.Checks(5000))
}

// Percent は total のうち done が占める割合を0から100の整数で返す。
// 0 <= done <= total かつ total > 0 とする。
func Percent(done, total int) int {
	return done * 100 / total
}

func PercentSafe(done, total int) int {
	return int(float64(done) / float64(total) * 100)
}

func percentRange(f func(done, total int) int) pbt.Property {
	return func(t *pbt.T) {
		total := pbt.IntRange(1, math.MaxInt).Draw(t, "total")
		done := pbt.IntRange(0, total).Draw(t, "done")

		if p := f(done, total); p < 0 || p > 100 {
			t.Fatalf("Percent(%d, %d) = %d", done, total, p)
		}
	}
}

func TestPercentMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, percentRange(Percent), append(opt, pbt.Checks(5000))...)
}
func TestPercentFullMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, func(t *pbt.T) {
		total := pbt.IntRange(1, math.MaxInt).Draw(t, "total")
		if p := Percent(total, total); p != 100 {
			t.Fatalf("Percent(%d, %d) = %d", total, total, p)
		}
	}, opt...)
}
func TestPercentSafe(t *testing.T) { pbt.Check(t, percentRange(PercentSafe), pbt.Checks(5000)) }
func TestPercentSmallSurvives(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		total := pbt.IntRange(1, 1000000).Draw(t, "total")
		done := pbt.IntRange(0, total).Draw(t, "done")
		if p := Percent(done, total); p < 0 || p > 100 {
			t.Fatalf("Percent(%d, %d) = %d", done, total, p)
		}
	}, append(opt, pbt.Checks(5000))...)
}

// Abs は x の絶対値を返す。
func Abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func TestAbsMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, func(t *pbt.T) {
		x := pbt.Int().Draw(t, "x")
		if got := Abs(x); got < 0 {
			t.Fatalf("Abs(%d) = %d", x, got)
		}
	}, opt...)
}
