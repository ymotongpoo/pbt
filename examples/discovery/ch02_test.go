// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"strings"
	"testing"

	"go.ymotongpoo.com/pbt"
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

func TestTruncateRunes(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		s := pbt.String().Draw(t, "s")
		n := pbt.IntRange(0, 100).Draw(t, "n")
		got := Truncate(s, n)
		if c := len([]rune(got)); c > n {
			t.Fatalf("Truncate(%q, %d) has %d runes, want <= %d", s, n, c, n)
		}
	})
}

func TestTruncateCoverage(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		s := pbt.String().Draw(t, "s")
		n := pbt.IntRange(0, 20).Draw(t, "n")

		t.Cover(len([]rune(s)) > n, 30, "actually truncated")
		t.Classify(strings.ContainsFunc(s, func(r rune) bool { return r > 127 }),
			"has multibyte")

		got := Truncate(s, n)
		if !strings.HasPrefix(s, got) {
			t.Fatalf("Truncate(%q, %d) = %q, not a prefix", s, n, got)
		}
	}, pbt.CheckCoverage(), pbt.Verbose())
}

var _ = pbt.StringOf(pbt.SampledFrom([]rune("あいうえおabc")))
