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

// runeRange is one range of a unicode.RangeTable: lo, lo+stride, ..., up
// to hi, with n code points.
type runeRange struct {
	lo, stride rune
	n          uint64
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
// point of the first table.
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
			ranges = appendRange(ranges, rune(r.Lo), rune(r.Hi), rune(r.Stride))
		}
		for _, r := range tab.R32 {
			ranges = appendRange(ranges, rune(r.Lo), rune(r.Hi), rune(r.Stride))
		}
	}
	name := "RuneIn(" + strings.Join(names, ", ") + ")"
	if len(ranges) == 0 {
		panic(fmt.Sprintf("pbt: %s: the tables hold no valid runes", name))
	}
	return Gen[rune]{name: name, draw: func(t *T) rune {
		r := ranges[t.draw(uint64(len(ranges)-1), plainMeta, modeUniform)]
		return r.lo + r.stride*rune(t.draw(r.n-1, plainMeta, modeBiased))
	}}
}

// appendRange appends the range lo..hi with the given stride, split around
// the surrogates.
func appendRange(rs []runeRange, lo, hi, stride rune) []runeRange {
	const surLo, surHi = 0xD800, 0xDFFF
	add := func(lo, hi rune) {
		if lo <= hi {
			rs = append(rs, runeRange{lo: lo, stride: stride, n: uint64((hi-lo)/stride) + 1})
		}
	}
	if hi < surLo || lo > surHi {
		add(lo, hi)
		return rs
	}
	if lo < surLo {
		add(lo, lo+(surLo-1-lo)/stride*stride)
	}
	if hi > surHi {
		first := lo + ((surHi+1-lo)+stride-1)/stride*stride
		add(first, hi)
	}
	return rs
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
