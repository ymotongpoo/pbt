// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package ch04

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"go.ymotongpoo.com/pbt"
	"go.ymotongpoo.com/pbt/examples/discovery/internal/mutant"
)

// ParseLimit は上限値の設定文字列を解釈する。
func ParseLimit(s string) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, errors.New("empty")
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0, err
	}
	if v < 1 || v > 1000 {
		return 0, errors.New("out of range")
	}
	return v, nil
}

type parser func(string) (int, error)

// ---- 仕様文ごとのプロパティ

func valid(parse parser) pbt.Property {
	return func(t *pbt.T) {
		v := pbt.IntRange(1, 1000).Draw(t, "v")

		got, err := parse(strconv.Itoa(v))
		if err != nil || got != v {
			t.Fatalf("ParseLimit(%q) = %d, %v; want %d, nil", strconv.Itoa(v), got, err, v)
		}
	}
}

func inRange(parse parser) pbt.Property {
	digits := pbt.StringOfN(pbt.SampledFrom([]rune("0123456789")), 1, 6)
	return func(t *pbt.T) {
		s := digits.Draw(t, "s")

		got, err := parse(s)
		if err != nil {
			return // 失敗してよい入力もある。ここでは成功時だけを述べる
		}
		if got < 1 || got > 1000 {
			t.Fatalf("ParseLimit(%q) = %d, out of 1..1000", s, got)
		}
	}
}

func spaces(parse parser) pbt.Property {
	blank := pbt.StringOfN(pbt.SampledFrom([]rune(" \t")), 0, 3)
	return func(t *pbt.T) {
		v := pbt.IntRange(1, 1000).Draw(t, "v")
		s := blank.Draw(t, "left") + strconv.Itoa(v) + blank.Draw(t, "right")

		got, err := parse(s)
		if err != nil || got != v {
			t.Fatalf("ParseLimit(%q) = %d, %v; want %d, nil", s, got, err, v)
		}
	}
}

// 不正な入力の生成器：空、文字だけ、範囲外の数
var invalidLimit = pbt.OneOf(
	pbt.Just(""),
	pbt.StringOfN(pbt.SampledFrom([]rune("abcdef")), 1, 5),
	pbt.Map(pbt.OneOf(pbt.Just(0), pbt.IntRange(1001, 100000)),
		func(v int) string { return strconv.Itoa(v) }),
)

// 数字の列のどこかに、数字以外の文字を1つ差し込む
var mixedLimit = pbt.Custom("mixedLimit", func(t *pbt.T) string {
	digits := pbt.StringOfN(pbt.SampledFrom([]rune("0123456789")), 1, 4).Draw(t, "digits")
	c := pbt.SampledFrom([]string{"a", "x", ".", "-"}).Draw(t, "c")
	i := pbt.IntRange(0, len(digits)).Draw(t, "i")
	return digits[:i] + c + digits[i:]
})

func errorsWith(parse parser, gen pbt.Gen[string]) pbt.Property {
	return func(t *pbt.T) {
		s := gen.Draw(t, "s")
		got, err := parse(s)
		if err == nil {
			t.Fatalf("ParseLimit(%q) succeeded with %d, want error", s, got)
		}
		if got != 0 { // 仕様文5：エラー時の数値は0
			t.Fatalf("ParseLimit(%q) = %d on error, want 0", s, got)
		}
	}
}

func boundary(t *testing.T, parse parser) {
	t.Helper()
	for _, c := range []struct {
		s  string
		ok bool
	}{{"1", true}, {"1000", true}, {"0", false}, {"1001", false}} {
		if _, err := parse(c.s); (err == nil) != c.ok {
			t.Errorf("ParseLimit(%q): err = %v, want ok = %v", c.s, err, c.ok)
		}
	}
}

func TestParseLimitMixed(t *testing.T) { pbt.Check(t, errorsWith(ParseLimit, mixedLimit)) }

// ---- 変異

func rangeCheck(v int, lo, hi int) (int, error) {
	if v < lo || v > hi {
		return 0, errors.New("out of range")
	}
	return v, nil
}

func trimmed(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", errors.New("empty")
	}
	return s, nil
}

// 上限の取り違え：1000 を拒否する
func upperExclusive(s string) (int, error) {
	s, err := trimmed(s)
	if err != nil {
		return 0, err
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0, err
	}
	return rangeCheck(v, 1, 999)
}

// 上限の取り違え：1001 を許す
func upperPlusOne(s string) (int, error) {
	s, err := trimmed(s)
	if err != nil {
		return 0, err
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0, err
	}
	return rangeCheck(v, 1, 1001)
}

// 下限の取り違え：0 を許す
func lowerZero(s string) (int, error) {
	s, err := trimmed(s)
	if err != nil {
		return 0, err
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0, err
	}
	return rangeCheck(v, 0, 1000)
}

// 下限の取り違え：1 を拒否する
func lowerExclusive(s string) (int, error) {
	s, err := trimmed(s)
	if err != nil {
		return 0, err
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0, err
	}
	return rangeCheck(v, 2, 1000)
}

// 空白を無視しない
func noTrim(s string) (int, error) {
	if s == "" {
		return 0, errors.New("empty")
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0, err
	}
	return rangeCheck(v, 1, 1000)
}

// エラーのときも解釈した値を返す
func valueOnError(s string) (int, error) {
	s, err := trimmed(s)
	if err != nil {
		return 0, err
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0, err
	}
	if v < 1 || v > 1000 {
		return v, errors.New("out of range")
	}
	return v, nil
}

// 先頭の数字だけを読み、続きの文字を無視する
var leadingDigits = regexp.MustCompile(`^[0-9]+`)

func prefixOnly(s string) (int, error) {
	s, err := trimmed(s)
	if err != nil {
		return 0, err
	}
	m := leadingDigits.FindString(s)
	if m == "" {
		return 0, errors.New("no digits")
	}
	v, err := strconv.Atoi(m)
	if err != nil {
		return 0, err
	}
	return rangeCheck(v, 1, 1000)
}

var mutants = []struct {
	name  string
	parse parser
}{
	{"UpperExclusive", upperExclusive},
	{"UpperPlusOne", upperPlusOne},
	{"LowerZero", lowerZero},
	{"LowerExclusive", lowerExclusive},
	{"NoTrim", noTrim},
	{"ValueOnError", valueOnError},
	{"PrefixOnly", prefixOnly},
}

func TestSpecTableMutant(t *testing.T) {
	mutant.Only(t)
	for _, m := range mutants {
		props := map[string]pbt.Property{
			"Valid":  valid(m.parse),
			"Range":  inRange(m.parse),
			"Spaces": spaces(m.parse),
			"Errors": errorsWith(m.parse, invalidLimit),
			"Mixed":  errorsWith(m.parse, mixedLimit),
		}
		for _, name := range []string{"Valid", "Range", "Spaces", "Errors", "Mixed"} {
			t.Run(m.name+"/"+name, func(t *testing.T) {
				pbt.Check(t, props[name], pbt.NoSave(), pbt.NoReplay())
			})
		}
		t.Run(m.name+"/Boundary", func(t *testing.T) { boundary(t, m.parse) })
	}
}

// 何回の実行で境界に当たるかの測定
func TestBoundaryReach(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		v := pbt.IntRange(1, 1000).Draw(t, "v")
		t.Classify(v == 1000, "v = 1000")
		t.Classify(v == 1, "v = 1")
	}, pbt.Verbose(), pbt.Checks(10000), pbt.NoSave(), pbt.NoReplay())
	pbt.Check(t, func(t *pbt.T) {
		v := pbt.IntRange(1001, 100000).Draw(t, "v")
		t.Classify(v == 1001, "v = 1001")
	}, pbt.Verbose(), pbt.Checks(10000), pbt.NoSave(), pbt.NoReplay())
}

// ---- 4.5 の悪い例

// parseStrict は先行ゼロを拒否する実装である。仕様5文はすべて満たす。
func parseStrict(s string) (int, error) {
	s, err := trimmed(s)
	if err != nil {
		return 0, err
	}
	if len(s) > 1 && s[0] == '0' {
		return 0, errors.New("leading zero")
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0, err
	}
	return rangeCheck(v, 1, 1000)
}

// 悪い例：先行ゼロが通ることを、仕様にないのに主張している
func leadingZeroAccepted(parse parser) pbt.Property {
	return func(t *pbt.T) {
		v := pbt.IntRange(1, 1000).Draw(t, "v")
		s := fmt.Sprintf("%05d", v)

		got, err := parse(s)
		if err != nil || got != v {
			t.Fatalf("ParseLimit(%q) = %d, %v; want %d, nil", s, got, err, v)
		}
	}
}

func TestLeadingZeroOnCurrent(t *testing.T) { pbt.Check(t, leadingZeroAccepted(ParseLimit)) }
func TestLeadingZeroOnStrictMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, leadingZeroAccepted(parseStrict), pbt.NoSave(), pbt.NoReplay())
}

// 実装を差し替えても、仕様文から書いたプロパティはすべて通る。
func TestStrictPassesSpec(t *testing.T) {
	pbt.Check(t, valid(parseStrict))
	pbt.Check(t, inRange(parseStrict))
	pbt.Check(t, spaces(parseStrict))
	pbt.Check(t, errorsWith(parseStrict, invalidLimit))
	pbt.Check(t, errorsWith(parseStrict, mixedLimit))
	boundary(t, parseStrict)
}
