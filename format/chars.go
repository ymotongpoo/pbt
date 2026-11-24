// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package format

import (
	"strings"
	"sync"

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

// How the generators of this package are built, so that the shrinker can
// simplify them: an alternative is a pbt.Frequency over generators that
// each write the whole branch, and an optional part is a Frequency of the
// empty string and the part, so that the index and the branch share one
// span; a part of variable length is drawn in a span of its own whose
// first choice is the length. The shrinker lowers the first choice of a
// span and reconnects the choices that follow the span, which needs both.

// choice generates the indices of weights in proportion to them, shrinking
// toward 0 whatever its weight. It is for decisions that do not change how
// many choices follow, such as the letter case of a fixed string.
func choice(weights ...int) pbt.Gen[int] {
	alts := make([]pbt.Weighted[int], len(weights))
	for i, w := range weights {
		alts[i] = pbt.W(w, pbt.Just(i))
	}
	return pbt.Frequency(alts...)
}

// optional generates "" in absent cases and a value of g in present ones,
// out of absent+present, shrinking toward "".
func optional(absent, present int, g pbt.Gen[string]) pbt.Gen[string] {
	return pbt.Frequency(pbt.W(absent, pbt.Just("")), pbt.W(present, g))
}

// concat generates the values of gens joined in order.
func concat(gens ...pbt.Gen[string]) pbt.Gen[string] {
	return pbt.Custom("concat", func(t *pbt.T) string {
		var b strings.Builder
		for _, g := range gens {
			b.WriteString(g.Draw(t, ""))
		}
		return b.String()
	})
}

// literalCase generates s as written in present cases and in lower case in
// lowered ones, for the case-insensitive strings of ABNF (RFC 5234 §2.3).
func literalCase(s string, written, lowered int) pbt.Gen[string] {
	return pbt.Frequency(pbt.W(written, pbt.Just(s)), pbt.W(lowered, pbt.Just(strings.ToLower(s))))
}

// charGens caches the generators of charIn, which the grammars call per
// character.
var charGens sync.Map // string -> pbt.Gen[byte]

// charIn generates one byte of set, shrinking toward set[0].
func charIn(set string) pbt.Gen[byte] {
	if g, ok := charGens.Load(set); ok {
		return g.(pbt.Gen[byte])
	}
	g, _ := charGens.LoadOrStore(set, pbt.SampledFrom([]byte(set)))
	return g.(pbt.Gen[byte])
}

// stringIn generates between minLen and maxLen bytes of set, with a length
// that grows with the size parameter like pbt.SliceOfN; a negative maxLen
// means no upper bound.
func stringIn(set string, minLen, maxLen int) pbt.Gen[string] {
	return pbt.Map(pbt.SliceOfN(charIn(set), minLen, maxLen), func(b []byte) string { return string(b) })
}

// fixedString generates strings of exactly n bytes of set.
func fixedString(set string, n int) pbt.Gen[string] {
	return pbt.Custom("fixed", func(t *pbt.T) string { return drawChars(t, set, n) })
}

// drawChars draws n bytes of set.
func drawChars(t *pbt.T, set string, n int) string {
	g := charIn(set)
	b := make([]byte, n)
	for i := range b {
		b[i] = g.Draw(t, "")
	}
	return string(b)
}

// joinWS joins items, dropping a white-space item that follows another:
// folding white space (RFC 5322 §3.2.2) may fold a line once between two
// other items, so two folds in a row are not FWS.
func joinWS(items []string) string {
	var b strings.Builder
	prev := false
	for _, it := range items {
		ws := it != "" && strings.IndexByte(" \t\r", it[0]) >= 0
		if ws && prev {
			continue
		}
		prev = ws
		b.WriteString(it)
	}
	return b.String()
}

// ldhLabel draws a label of n letters, digits and hyphens that starts with
// a character of first and ends with an alphanumeric character.
func ldhLabel(t *pbt.T, first string, n int) string {
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

var labelLen = pbt.IntRange(1, 63)

// labelGens caches the generators of labelOf.
var labelGens sync.Map // string -> pbt.Gen[string]

// labelOf generates labels like ldhLabel with a length from 1 to 63, drawn
// like an integer so that 63 is common, in a span of their own whose first
// choice is the length.
func labelOf(first string) pbt.Gen[string] {
	if g, ok := labelGens.Load(first); ok {
		return g.(pbt.Gen[string])
	}
	g, _ := labelGens.LoadOrStore(first, pbt.Custom("label", func(t *pbt.T) string {
		return ldhLabel(t, first, labelLen.Draw(t, ""))
	}))
	return g.(pbt.Gen[string])
}

// hexDigit writes the nibble v as a hex digit, in upper case when up is set.
func hexDigit(v byte, up bool) byte {
	c := hexLower[v&0xf]
	if up && c >= 'a' {
		c -= 'a' - 'A'
	}
	return c
}
