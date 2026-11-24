// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package format

import (
	"strings"

	"go.ymotongpoo.com/pbt"
)

// JSONPointer generates json-pointer of RFC 6901 §3: zero to eight
// slash-prefixed reference-tokens, each with zero to 16 Unicode scalar
// values, escaping tilde as ~0 and slash as ~1. A character is arbitrary
// Unicode with probability 3/5, and tilde or slash with probability 1/5
// each. Controls including NUL, empty tokens and the empty pointer are
// allowed. No target document or array-index semantics are imposed.
// Values shrink toward "" (the whole document), or "/" if nonempty.
func JSONPointer() pbt.Gen[string] {
	tokens := pbt.SliceOfN(pointerToken, 0, 8)
	return pbt.Custom("JSONPointer()", func(t *pbt.T) string {
		var b strings.Builder
		for _, s := range tokens.Draw(t, "") {
			b.WriteByte('/')
			b.WriteString(s)
		}
		return b.String()
	})
}

var pointerToken = pbt.Map(pbt.SliceOfN(pbt.Frequency(pbt.W(3, pbt.Rune()), pbt.W(1, pbt.Just('~')), pbt.W(1, pbt.Just('/'))), 0, 16), func(rs []rune) string {
	var b strings.Builder
	for _, r := range rs {
		switch r {
		case '~':
			b.WriteString("~0")
		case '/':
			b.WriteString("~1")
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
})

// JSONPointerFragment generates the URI fragment representation of RFC
// 6901 §6: "#" followed by a pointer with UTF-8 octets outside RFC 3986's
// fragment alphabet percent-encoded using upper-case hex. Bounds and
// character frequencies are as JSONPointer; optional percent-encoding of
// allowed ASCII characters is not generated. net/url parses the URI
// reference, but does not evaluate the pointer. Values shrink toward "#",
// or "#/" if a nonempty pointer is required.
func JSONPointerFragment() pbt.Gen[string] {
	pointers := JSONPointer()
	return pbt.Custom("JSONPointerFragment()", func(t *pbt.T) string {
		s := pointers.Draw(t, "")
		var b strings.Builder
		b.WriteByte('#')
		for i := range len(s) {
			c := s[i]
			if strings.ContainsRune(pchar+"/?", rune(c)) {
				b.WriteByte(c)
			} else {
				b.WriteByte('%')
				b.WriteByte(hexDigit(c>>4, true))
				b.WriteByte(hexDigit(c, true))
			}
		}
		return b.String()
	})
}
