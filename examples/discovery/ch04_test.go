// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"go.ymotongpoo.com/pbt"
)

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

func TestParseLimitSuccess(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		v := pbt.IntRange(1, 1000).Draw(t, "v")
		got, err := ParseLimit(strconv.Itoa(v))
		if err != nil || got != v {
			t.Fatalf("ParseLimit(%q) = %d, %v; want %d, nil", strconv.Itoa(v), got, err, v)
		}
	})
}

var invalidLimit = pbt.OneOf(
	pbt.Just(""),
	pbt.StringOfN(pbt.SampledFrom([]rune("abcdef")), 1, 5),
	pbt.Map(pbt.OneOf(pbt.Just(0), pbt.IntRange(1001, 100000)),
		func(v int) string { return strconv.Itoa(v) }),
)

func TestParseLimitErrors(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		s := invalidLimit.Draw(t, "s")
		got, err := ParseLimit(s)
		if err == nil {
			t.Fatalf("ParseLimit(%q) succeeded with %d, want error", s, got)
		}
		if got != 0 {
			t.Fatalf("ParseLimit(%q) = %d on error, want 0", s, got)
		}
	})
}
