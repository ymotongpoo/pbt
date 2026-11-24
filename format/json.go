// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package format

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"go.ymotongpoo.com/pbt"
)

// maxJSONDepth bounds the nesting of arrays and objects.
const maxJSONDepth = 4

var (
	jsonWS      = choice(6, 3, 1) // nothing, a space, white space of the four kinds RFC 8259 allows
	jsonWSText  = stringIn(" \t\n\r", 1, 4)
	jsonSign    = choice(4, 1)    // none, a minus sign
	jsonInt     = choice(1, 3)    // zero, digits that do not start with zero
	jsonFrac    = choice(3, 1)    // none, a fraction
	jsonExp     = choice(4, 1)    // none, an exponent
	jsonExpSign = choice(2, 1, 1) // none, "+", "-"
	jsonDigits  = stringIn(digits, 0, 24)
	jsonDigits1 = stringIn(digits, 1, 24)
	expDigits   = stringIn(digits, 1, 4)
	jsonEscape  = choice(8, 1, 1) // literal, a short escape when there is one, \u
	jsonHexCase = choice(3, 1)
	jsonItem    = choice(30, 1) // a character, an unpaired surrogate escape
	jsonRunes   = pbt.Rune()
	jsonLonely  = pbt.IntRange(0xD800, 0xDFFF)
	jsonKeyPool = choice(3, 1) // any name, one of a few short names
	jsonArray   = pbt.IntRange(0, 6)
	ijsonNumber = choice(2, 1) // a safe integer, a binary64 value
	safeInts    = pbt.Int64Range(-(1<<53 - 1), 1<<53-1)
	floats      = pbt.Float64()
	ijsonTop    = choice(1, 1) // an array, an object
)

// JSON generates JSON texts in the syntax of RFC 8259: a value of any
// kind at the top level, as §2 allows, with white space of spaces, tabs,
// line feeds and carriage returns between tokens. Numbers follow the
// grammar of §6, including integers far beyond 64 bits, fractions and
// exponents of either case with or without a sign; strings mix literal
// characters with every escape of §7, \u escapes in either case, escaped
// surrogate pairs, and in a share of the strings an unpaired surrogate
// escape, which §8.2 says the grammar allows and parsers handle
// unpredictably; objects sometimes repeat a member name, which §4 allows
// with the same warning; arrays and objects nest up to four levels.
// encoding/json.Valid reports true for every value. Values shrink toward
// "null".
func JSON() pbt.Gen[string] {
	values := jsonValues(false)
	return pbt.Custom("JSON()", func(t *pbt.T) string {
		return ws(t) + values[maxJSONDepth].Draw(t, "") + ws(t)
	})
}

// IJSON generates texts of the Internet JSON profile of RFC 7493: an
// object or an array at the top level, as §4.1 recommends; member names
// that are unique within their object after unescaping (§2.3); numbers
// that IEEE 754 binary64 represents exactly, integers within ±(2^53-1) and
// values written by strconv.FormatFloat, with an exponent when they would
// otherwise read as an integer beyond that range (§2.2); and strings of Unicode
// scalar values without noncharacters, with surrogates only as escaped
// pairs (§2.1). Values shrink toward "[]".
func IJSON() pbt.Gen[string] {
	values := jsonValues(true)
	return pbt.Custom("IJSON()", func(t *pbt.T) string {
		top := ijsonTop.Draw(t, "")
		var v string
		if top == 0 {
			v = jsonArrayOf(t, values[maxJSONDepth-1])
		} else {
			v = ijsonObject(t, values[maxJSONDepth-1])
		}
		return ws(t) + v + ws(t)
	})
}

func ws(t *pbt.T) string {
	switch jsonWS.Draw(t, "") {
	case 1:
		return " "
	case 2:
		return jsonWSText.Draw(t, "")
	}
	return ""
}

// jsonValues returns the generators of values nested up to each depth.
func jsonValues(ijson bool) []pbt.Gen[string] {
	gens := make([]pbt.Gen[string], maxJSONDepth+1)
	str := jsonString(ijson)
	num := jsonNumber
	if ijson {
		num = ijsonNumberGen
	}
	for d := range gens {
		alts := []pbt.Weighted[string]{
			pbt.W(1, pbt.Just("null")),
			pbt.W(1, pbt.Just("true")),
			pbt.W(1, pbt.Just("false")),
			pbt.W(4, num),
			pbt.W(4, str),
		}
		if d > 0 {
			inner := gens[d-1]
			alts = append(alts,
				pbt.W(2, pbt.Custom("array", func(t *pbt.T) string { return jsonArrayOf(t, inner) })),
				pbt.W(2, pbt.Custom("object", func(t *pbt.T) string {
					if ijson {
						return ijsonObject(t, inner)
					}
					return jsonObject(t, inner, str)
				})),
			)
		}
		gens[d] = pbt.Frequency(alts...)
	}
	return gens
}

func jsonArrayOf(t *pbt.T, elems pbt.Gen[string]) string {
	n := jsonArray.Draw(t, "")
	var b strings.Builder
	b.WriteString("[" + ws(t))
	for i := range n {
		if i > 0 {
			b.WriteString(ws(t) + "," + ws(t))
		}
		b.WriteString(elems.Draw(t, ""))
	}
	b.WriteString(ws(t) + "]")
	return b.String()
}

// jsonObject writes members whose names may repeat: a quarter of the
// names come from a pool of two.
func jsonObject(t *pbt.T, values, names pbt.Gen[string]) string {
	n := jsonArray.Draw(t, "")
	var b strings.Builder
	b.WriteString("{" + ws(t))
	for i := range n {
		if i > 0 {
			b.WriteString(ws(t) + "," + ws(t))
		}
		name := `"a"`
		switch jsonKeyPool.Draw(t, "") {
		case 0:
			name = names.Draw(t, "")
		default:
			if pbt.Bool().Draw(t, "") {
				name = `"b"`
			}
		}
		b.WriteString(name + ws(t) + ":" + ws(t) + values.Draw(t, ""))
	}
	b.WriteString(ws(t) + "}")
	return b.String()
}

// ijsonNames generates the distinct unescaped names of an I-JSON object.
var ijsonNames = pbt.SliceOfNDistinct(pbt.StringOfN(scalar(), 0, 8), 0, 6, func(s string) string { return s })

func ijsonObject(t *pbt.T, values pbt.Gen[string]) string {
	var b strings.Builder
	b.WriteString("{" + ws(t))
	for i, name := range ijsonNames.Draw(t, "") {
		if i > 0 {
			b.WriteString(ws(t) + "," + ws(t))
		}
		b.WriteString(encodeJSON(t, name) + ws(t) + ":" + ws(t) + values.Draw(t, ""))
	}
	b.WriteString(ws(t) + "}")
	return b.String()
}

// scalar generates the runes of pbt.Rune with the noncharacters of Unicode
// replaced by 'a', for I-JSON.
func scalar() pbt.Gen[rune] {
	return pbt.Map(pbt.Rune(), func(r rune) rune {
		if isNoncharacter(r) {
			return 'a'
		}
		return r
	})
}

func isNoncharacter(r rune) bool {
	return (r >= 0xFDD0 && r <= 0xFDEF) || r&0xFFFE == 0xFFFE
}

// jsonString returns the generator of JSON strings; for I-JSON without
// unpaired surrogates and noncharacters.
func jsonString(ijson bool) pbt.Gen[string] {
	if ijson {
		raw := pbt.StringOfN(scalar(), 0, 16)
		return pbt.Custom("string", func(t *pbt.T) string { return encodeJSON(t, raw.Draw(t, "")) })
	}
	item := pbt.Custom("char", func(t *pbt.T) string {
		if jsonItem.Draw(t, "") == 1 {
			up := jsonHexCase.Draw(t, "") == 1
			return uEscape(rune(jsonLonely.Draw(t, "")), up)
		}
		return encodeRune(t, jsonRunes.Draw(t, ""))
	})
	items := pbt.SliceOfN(item, 0, 16)
	return pbt.Custom("string", func(t *pbt.T) string {
		return `"` + strings.Join(items.Draw(t, ""), "") + `"`
	})
}

// encodeJSON writes s as a JSON string, escaping its runes at random.
func encodeJSON(t *pbt.T, s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		b.WriteString(encodeRune(t, r))
	}
	b.WriteByte('"')
	return b.String()
}

var shortEscapes = map[rune]string{'"': `\"`, '\\': `\\`, '/': `\/`, '\b': `\b`, '\f': `\f`, '\n': `\n`, '\r': `\r`, '\t': `\t`}

// encodeRune writes r literally, with its short escape, or as \u escapes,
// a surrogate pair beyond the Basic Multilingual Plane. The quotation mark,
// the backslash and the control characters are always escaped (§7).
func encodeRune(t *pbt.T, r rune) string {
	form := jsonEscape.Draw(t, "")
	up := jsonHexCase.Draw(t, "") == 1
	if form == 0 && (r == '"' || r == '\\' || r < 0x20) {
		form = 1
	}
	if form == 1 {
		if e, ok := shortEscapes[r]; ok {
			return e
		}
		if r >= 0x20 {
			form = 0
		}
	}
	if form == 0 {
		return string(r)
	}
	if r > 0xFFFF {
		r -= 0x10000
		return uEscape(0xD800+(r>>10), up) + uEscape(0xDC00+(r&0x3FF), up)
	}
	return uEscape(r, up)
}

func uEscape(r rune, up bool) string {
	s := fmt.Sprintf(`\u%04x`, r)
	if up {
		s = `\u` + strings.ToUpper(s[2:])
	}
	return s
}

// jsonNumber generates numbers in the grammar of RFC 8259 §6.
var jsonNumber = pbt.Custom("number", func(t *pbt.T) string {
	var b strings.Builder
	if jsonSign.Draw(t, "") == 1 {
		b.WriteByte('-')
	}
	if jsonInt.Draw(t, "") == 0 {
		b.WriteByte('0')
	} else {
		b.WriteByte(byte('1' + pbt.IntRange(0, 8).Draw(t, "")))
		b.WriteString(jsonDigits.Draw(t, ""))
	}
	if jsonFrac.Draw(t, "") == 1 {
		b.WriteString("." + jsonDigits1.Draw(t, ""))
	}
	if jsonExp.Draw(t, "") == 1 {
		b.WriteString([...]string{"e", "E"}[pbt.IntRange(0, 1).Draw(t, "")])
		b.WriteString([...]string{"", "+", "-"}[jsonExpSign.Draw(t, "")])
		b.WriteString(expDigits.Draw(t, ""))
	}
	return b.String()
})

// ijsonNumberGen generates numbers that binary64 represents exactly.
var ijsonNumberGen = pbt.Custom("number", func(t *pbt.T) string {
	if ijsonNumber.Draw(t, "") == 0 {
		return strconv.FormatInt(safeInts.Draw(t, ""), 10)
	}
	f := floats.Draw(t, "")
	format := [...]byte{'g', 'e', 'f'}[pbt.IntRange(0, 2).Draw(t, "")]
	if format == 'f' && math.Abs(f) > 1<<53-1 {
		// Written without an exponent, such a value reads as an integer
		// outside the range §2.2 lets a receiver treat as exact.
		format = 'e'
	}
	return strconv.FormatFloat(f, format, -1, 64)
})
