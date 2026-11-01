// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package ch02

import (
	"strings"
	"testing"
	"unicode/utf8"

	"go.ymotongpoo.com/pbt"
	"go.ymotongpoo.com/pbt/examples/discovery/internal/mutant"
)

func Truncate(s string, n int) string {
	if n <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

func alwaysEmpty(string, int) string { return "" }

var opt = []pbt.Option{pbt.NoSave(), pbt.NoReplay()}

func runeBound(f func(string, int) string) pbt.Property {
	return func(t *pbt.T) {
		s := pbt.String().Draw(t, "s")
		n := pbt.IntRange(0, 100).Draw(t, "n") // 前提 n >= 0 を生成器で担保する

		got := f(s, n)
		if c := len([]rune(got)); c > n {
			t.Fatalf("Truncate(%q, %d) has %d runes, want <= %d", s, n, c, n)
		}
	}
}

func runeExact(f func(string, int) string) pbt.Property {
	return func(t *pbt.T) {
		s := pbt.String().Draw(t, "s")
		n := pbt.IntRange(-5, 100).Draw(t, "n")

		got := f(s, n)
		if c, want := len([]rune(got)), max(0, min(n, len([]rune(s)))); c != want {
			t.Fatalf("Truncate(%q, %d) has %d runes, want %d", s, n, c, want)
		}
	}
}

func TestRuneBound(t *testing.T) { pbt.Check(t, runeBound(Truncate)) }
func TestRuneExact(t *testing.T) { pbt.Check(t, runeExact(Truncate)) }

// 不等式の主張は、常に空文字列を返す実装を見逃す。
func TestRuneBoundEmptySurvives(t *testing.T) { pbt.Check(t, runeBound(alwaysEmpty), opt...) }

// 等式の主張は同じ実装を落とす。
func TestRuneExactEmptyMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, runeExact(alwaysEmpty), opt...)
}

// バイト数の上限は、正しい実装を落とす主張である。
func TestByteBoundWrongClaimMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, func(t *pbt.T) {
		s := pbt.String().Draw(t, "s")
		n := pbt.IntRange(0, 20).Draw(t, "n")
		got := Truncate(s, n)
		if len(got) > n {
			t.Fatalf("too long: Truncate(%q, %d) = %q", s, n, got)
		}
	}, opt...)
}

func TestByteBoundExample(t *testing.T) {
	got := Truncate("あい", 1)
	if got != "あ" || len([]rune(got)) != 1 || len(got) != 3 {
		t.Fatalf("got %q", got)
	}
}

func TestAppendKeepsResult(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		s := pbt.String().Draw(t, "s")
		x := pbt.String().Draw(t, "x")
		n := pbt.IntRange(-5, len([]rune(s))).Draw(t, "n")
		if a, b := Truncate(s, n), Truncate(s+x, n); a != b {
			t.Fatalf("Truncate(%q, %d) = %q, with %q appended = %q", s, n, a, x, b)
		}
	})
}

func TestStringIsValidUTF8(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		if s := pbt.String().Draw(t, "s"); !utf8.ValidString(s) {
			t.Fatalf("invalid UTF-8: %q", s)
		}
	}, pbt.Checks(2000))
}

func TestInvalidUTF8NotPrefix(t *testing.T) {
	s := "\x80a"
	got := Truncate(s, 1)
	if got != "�" || strings.HasPrefix(s, got) {
		t.Fatalf("Truncate(%q, 1) = %q", s, got)
	}
}
