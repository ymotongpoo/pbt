// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package ch04

import (
	"fmt"
	"strconv"
	"testing"

	"go.ymotongpoo.com/pbt"
)

func TestParseLimitValid(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		v := pbt.IntRange(1, 1000).Draw(t, "v")

		got, err := ParseLimit(strconv.Itoa(v))
		if err != nil || got != v {
			t.Fatalf("ParseLimit(%q) = %d, %v; want %d, nil", strconv.Itoa(v), got, err, v)
		}
	})
}

// 仕様文2：成功して返る値は必ず 1..1000 の範囲内である
func TestParseLimitRange(t *testing.T) {
	digits := pbt.StringOfN(pbt.SampledFrom([]rune("0123456789")), 1, 6)
	pbt.Check(t, func(t *pbt.T) {
		s := digits.Draw(t, "s")

		got, err := ParseLimit(s)
		if err != nil {
			return // 失敗してよい入力もある。ここでは成功時だけを述べる
		}
		if got < 1 || got > 1000 {
			t.Fatalf("ParseLimit(%q) = %d, out of 1..1000", s, got)
		}
	})
}

// 仕様文3：前後の空白は解釈の結果を変えない
func TestParseLimitSpaces(t *testing.T) {
	blank := pbt.StringOfN(pbt.SampledFrom([]rune(" \t")), 0, 3)
	pbt.Check(t, func(t *pbt.T) {
		v := pbt.IntRange(1, 1000).Draw(t, "v")
		s := blank.Draw(t, "left") + strconv.Itoa(v) + blank.Draw(t, "right")

		got, err := ParseLimit(s)
		if err != nil || got != v {
			t.Fatalf("ParseLimit(%q) = %d, %v; want %d, nil", s, got, err, v)
		}
	})
}

func TestParseLimitErrors(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		s := invalidLimit.Draw(t, "s")
		got, err := ParseLimit(s)
		if err == nil {
			t.Fatalf("ParseLimit(%q) succeeded with %d, want error", s, got)
		}
		if got != 0 { // 仕様文5：エラー時の数値は0
			t.Fatalf("ParseLimit(%q) = %d on error, want 0", s, got)
		}
	})
}

func TestParseLimitErrorsAll(t *testing.T) {
	pbt.Check(t, errorsWith(ParseLimit, pbt.OneOf(invalidLimit, mixedLimit)))
}

func TestParseLimitBoundary(t *testing.T) {
	for _, c := range []struct {
		s  string
		ok bool
	}{{"1", true}, {"1000", true}, {"0", false}, {"1001", false}} {
		if _, err := ParseLimit(c.s); (err == nil) != c.ok {
			t.Errorf("ParseLimit(%q): err = %v, want ok = %v", c.s, err, c.ok)
		}
	}
}

func TestBookBadLeadingZero(t *testing.T) {
	// 悪い例：先行ゼロが通ることを、仕様にないのに主張している
	pbt.Check(t, func(t *pbt.T) {
		v := pbt.IntRange(1, 1000).Draw(t, "v")
		s := fmt.Sprintf("%05d", v)

		got, err := ParseLimit(s)
		if err != nil || got != v {
			t.Fatalf("ParseLimit(%q) = %d, %v; want %d, nil", s, got, err, v)
		}
	})
}

// 4.5節の要確認の項目について、現在の実装の振る舞いを確かめる。
func TestUndecidedInputsToday(t *testing.T) {
	for s, want := range map[string]int{"007": 7, "+5": 5, "　5": 5} {
		if got, err := ParseLimit(s); err != nil || got != want {
			t.Errorf("ParseLimit(%q) = %d, %v; want %d", s, got, err, want)
		}
	}
	if _, err := ParseLimit("１２３"); err == nil {
		t.Error("full-width digits accepted")
	}
}
