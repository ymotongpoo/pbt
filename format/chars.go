// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package format

import (
	"strings"

	"go.ymotongpoo.com/pbt"
)

// Character sets of the grammars, each listed from the simplest character,
// which is where shrinking ends, to the most unusual one.
const (
	lower    = "abcdefghijklmnopqrstuvwxyz"
	upper    = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	digits   = "0123456789"
	alpha    = lower + upper
	alnum    = lower + digits + upper
	hexLower = "0123456789abcdef"
	// atext of RFC 5322 §3.2.3 and RFC 5321 §4.1.2.
	atext = alnum + "!#$%&'*+-/=?^_`{|}~"
)

// choice generates the indices of weights in proportion to them, shrinking
// toward 0 whatever its weight.
func choice(weights ...int) pbt.Gen[int] {
	alts := make([]pbt.Weighted[int], len(weights))
	for i, w := range weights {
		alts[i] = pbt.W(w, pbt.Just(i))
	}
	return pbt.Frequency(alts...)
}

// charIn generates one byte of set, shrinking toward set[0].
func charIn(set string) pbt.Gen[byte] { return pbt.SampledFrom([]byte(set)) }

// stringIn generates between minLen and maxLen bytes of set, with a length
// that grows with the size parameter like pbt.SliceOfN; a negative maxLen
// means no upper bound.
func stringIn(set string, minLen, maxLen int) pbt.Gen[string] {
	return pbt.Map(pbt.SliceOfN(charIn(set), minLen, maxLen), func(b []byte) string { return string(b) })
}

// ldhLabel draws a label of letters, digits and hyphens that starts with a
// character of first and ends with an alphanumeric character, with a
// length between 1 and maxLen drawn like an integer, so that the longest
// label is common.
func ldhLabel(t *pbt.T, first string, maxLen int) string {
	n := pbt.IntRange(1, maxLen).Draw(t, "")
	var b strings.Builder
	b.WriteByte(charIn(first).Draw(t, ""))
	for i := 1; i < n; i++ {
		set := alnum + "-"
		if i == n-1 {
			set = alnum
		}
		b.WriteByte(charIn(set).Draw(t, ""))
	}
	return b.String()
}

// hexDigit writes the nibble v as a hex digit, in upper case when up is set.
func hexDigit(v byte, up bool) byte {
	c := hexLower[v&0xf]
	if up && c >= 'a' {
		c -= 'a' - 'A'
	}
	return c
}
