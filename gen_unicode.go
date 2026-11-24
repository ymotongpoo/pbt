// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package pbt

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"unicode"
)

// runeRange is one range of a unicode.RangeTable without its surrogates:
// n code points lo, lo+stride, ..., where the points from index below on
// continue at above instead, past the surrogates. A range that does not
// cross the surrogates has below == n.
type runeRange struct {
	lo, stride, above rune
	n, below          uint64
}

// at returns the k-th code point of the range.
func (r runeRange) at(k uint64) rune {
	if k < r.below {
		return r.lo + r.stride*rune(k)
	}
	return r.above + r.stride*rune(k-r.below)
}

// RuneIn generates the code points of the given Unicode range tables, such
// as the categories, scripts and properties of package unicode:
// RuneIn(unicode.Han, unicode.Hiragana, unicode.Katakana) or
// RuneIn(unicode.Nd). Surrogates (U+D800 to U+DFFF) are left out because
// they are not valid runes, so RuneIn(unicode.Cs) panics, as does RuneIn
// without tables.
//
// Every range of the tables is equally likely, and the code points of a
// range are drawn like an unsigned integer, so small scripts are not drowned
// out by large blocks such as the CJK ideographs, and the first and the last
// code point of a range are common. Values shrink toward the first code
// point of the first table. RuneIn panics on a table with a range that is
// not valid: a stride of 0, a low bound above the high bound, or a bound
// above unicode.MaxRune.
func RuneIn(tables ...*unicode.RangeTable) Gen[rune] {
	if len(tables) == 0 {
		panic("pbt: RuneIn needs at least one table")
	}
	var ranges []runeRange
	names := make([]string, len(tables))
	for i, tab := range tables {
		if tab == nil {
			panic("pbt: RuneIn: nil table")
		}
		names[i] = tableName(tab)
		for _, r := range tab.R16 {
			ranges = appendRange(ranges, uint32(r.Lo), uint32(r.Hi), uint32(r.Stride), names[i])
		}
		for _, r := range tab.R32 {
			ranges = appendRange(ranges, r.Lo, r.Hi, r.Stride, names[i])
		}
	}
	name := "RuneIn(" + strings.Join(names, ", ") + ")"
	if len(ranges) == 0 {
		panic(fmt.Sprintf("pbt: %s: the tables hold no valid runes", name))
	}
	return Gen[rune]{name: name, draw: func(t *T) rune {
		r := ranges[t.draw(uint64(len(ranges)-1), plainMeta, modeUniform)]
		return r.at(t.draw(r.n-1, plainMeta, modeBiased))
	}}
}

// appendRange appends the range lo..hi with the given stride without its
// surrogates, as one range so that it keeps its share of the draws. It
// panics on a range that is not valid in a unicode.RangeTable.
func appendRange(rs []runeRange, lo, hi, stride uint32, table string) []runeRange {
	const surLo, surHi = 0xD800, 0xDFFF
	if stride == 0 || lo > hi || hi > unicode.MaxRune {
		panic(fmt.Sprintf("pbt: RuneIn: table %s has an invalid range %#x..%#x with stride %d", table, lo, hi, stride))
	}
	last := lo + (hi-lo)/stride*stride
	count := func(a, b uint32) uint64 { // points of the range in [a, b]
		if b < lo || a > last || a > b {
			return 0
		}
		first := lo
		if a > lo {
			first = lo + (a-lo+stride-1)/stride*stride
		}
		end := min(b, last)
		if first > end {
			return 0
		}
		return uint64((end-first)/stride) + 1
	}
	below := count(0, surLo-1)
	n := below + count(surHi+1, unicode.MaxRune)
	if n == 0 {
		return rs
	}
	above := rune(lo) + rune(stride)*rune(below+count(surLo, surHi))
	return append(rs, runeRange{lo: rune(lo), stride: rune(stride), above: above, n: n, below: below})
}

// tableNames maps the tables of package unicode to their names, for the
// description of RuneIn.
var tableNames = sync.OnceValue(func() map[*unicode.RangeTable]string {
	m := map[*unicode.RangeTable]string{}
	for _, group := range []map[string]*unicode.RangeTable{unicode.Properties, unicode.Scripts, unicode.Categories} {
		keys := make([]string, 0, len(group))
		for k := range group {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			m[group[k]] = k // categories last, so unicode.L is "L" and not a property alias
		}
	}
	return m
})

func tableName(tab *unicode.RangeTable) string {
	if n, ok := tableNames()[tab]; ok {
		return n
	}
	return "table"
}
