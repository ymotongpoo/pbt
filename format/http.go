// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package format

import (
	"strings"
	"time"

	"go.ymotongpoo.com/pbt"
)

// HTTPToken generates token of RFC 9110 §5.6.2: one to 32 tchar bytes,
// with both letter cases and all punctuation. Values shrink toward "a".
func HTTPToken() pbt.Gen[string] {
	return pbt.Custom("HTTPToken()", func(t *pbt.T) string { return httpToken.Draw(t, "") })
}

// HTTPFieldName generates field-name of RFC 9110 §5.1, which is token
// (§5.6.2), with 1 to 32 bytes in any case. Values shrink toward "a".
func HTTPFieldName() pbt.Gen[string] {
	return pbt.Custom("HTTPFieldName()", func(t *pbt.T) string { return httpToken.Draw(t, "") })
}

// HTTPFieldValue generates the ASCII subset of field-value of RFC 9110
// §5.5: 0 to 64 bytes, VCHAR at the edges, SP and HTAB also allowed inside.
// It excludes obsolete obs-text, other controls and line folding. Values shrink
// toward "". For all octets of the production use RFC9110FieldValue.
func HTTPFieldValue() pbt.Gen[string] { return httpFieldValue(false) }

// RFC9110FieldValue generates field-value of RFC 9110 §5.5, including
// obs-text (%x80-FF), which need not be UTF-8. Length is 0 to 64 bytes;
// nonempty values have field-vchar at both edges, with SP and HTAB also
// allowed inside. Each byte is drawn from its full allowed alphabet, so
// obs-text is common in longer values. CR, LF and DEL are never emitted.
// Values shrink toward "".
func RFC9110FieldValue() pbt.Gen[string] { return httpFieldValue(true) }
func httpFieldValue(obs bool) pbt.Gen[string] {
	name := "HTTPFieldValue()"
	set := vchar
	if obs {
		name = "RFC9110FieldValue()"
		set += obsBytes
	}
	return pbt.Custom(name, func(t *pbt.T) string {
		n := fieldValueLen.Draw(t, "")
		b := make([]byte, n)
		for i := range b {
			chars := set
			if i > 0 && i < n-1 {
				chars += " \t"
			}
			b[i] = charIn(chars).Draw(t, "")
		}
		return string(b)
	})
}

var fieldValueLen = pbt.IntRange(0, 64)

var cookieOctets = strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(vchar, "\"", ""), ",", ""), ";", "")
var rawCookieValue = stringIn(strings.ReplaceAll(cookieOctets, "\\", ""), 0, 32)
var cookieValue = pbt.OneOf(rawCookieValue, affix(rawCookieValue, "\"", "\""))
var cookiePair = pbt.Custom("cookie-pair", func(t *pbt.T) string { return httpToken.Draw(t, "") + "=" + cookieValue.Draw(t, "") })

// RFC6265Cookie generates cookie-string of RFC 6265 §4.2.1: one to six
// cookie-pairs separated by "; ". Names are tokens of 1 to 32 bytes;
// values have 0 to 32 cookie-octets, quoted in half the pairs. Names may
// repeat. net/http.ParseCookie accepts every value. Values shrink toward
// "a="; if quoting is retained the single pair shrinks toward `a=""`.
func RFC6265Cookie() pbt.Gen[string] {
	return pbt.Custom("RFC6265Cookie()", func(t *pbt.T) string { return strings.Join(cookiePairs.Draw(t, ""), "; ") })
}

var (
	cookiePairs      = pbt.SliceOfN(cookiePair, 1, 6)
	cookieAttributes = pbt.SliceOfN(cookieAttribute, 0, 6)
)

// RFC6265SetCookie generates set-cookie-string of RFC 6265 §4.1.1: a
// cookie-pair and zero to six cookie-av attributes. Attribute kinds are
// equally likely: Expires (sane-cookie-date), Max-Age (1 to 20 digits,
// positive), Domain (one to four LDH labels, at most 253 bytes), Path,
// Secure, HttpOnly, or extension-av. Attribute names take any letter case;
// repeated attributes are allowed by the grammar. Path and extension
// values have 0 to 32 printable ASCII bytes except semicolon (the sequences
// clarified by RFC 6265 errata 3444). Expires dates keep the letter case of
// rfc1123-date, which RFC 2616 §3.3.1 makes case-sensitive, and have years
// from 0000 to 9999; the syntax permits years below the recipient
// algorithm's 1601 cutoff (§5.1.1).
// No SameSite-specific semantics from later drafts are imposed; such text
// can occur as extension-av. Other bounds and quoting are as RFC6265Cookie.
// Values shrink toward "a=".
//
// net/http.ParseSetCookie accepts these strings but can ignore attributes:
// overflowing Max-Age goes into Unparsed, and backslash or quote in a Path
// makes it unparsed. Go also accepts nonpositive Max-Age, outside the
// max-age-av production, and parses attributes with recipient leniency.
func RFC6265SetCookie() pbt.Gen[string] {
	return pbt.Custom("RFC6265SetCookie()", func(t *pbt.T) string {
		s := cookiePair.Draw(t, "")
		for _, a := range cookieAttributes.Draw(t, "") {
			s += "; " + a
		}
		return s
	})
}
func cookieCase(t *pbt.T, s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
			b[i] = c
		}
		if c >= 'a' && c <= 'z' && pbt.Bool().Draw(t, "") {
			b[i] = c - 'a' + 'A'
		}
	}
	return string(b)
}
func cookieAV(name string, value pbt.Gen[string]) pbt.Gen[string] {
	return pbt.Custom(name, func(t *pbt.T) string { return cookieCase(t, name) + value.Draw(t, "") })
}

var cookieAttribute = pbt.OneOf(
	cookieAV("Expires=", cookieDate),
	cookieAV("Max-Age=", cookieMaxAge),
	cookieAV("Domain=", cookieDomain),
	cookieAV("Path=", stringIn(strings.ReplaceAll(vchar+" ", ";", ""), 0, 32)),
	cookieAV("Secure", pbt.Just("")),
	cookieAV("HttpOnly", pbt.Just("")),
	stringIn(strings.ReplaceAll(vchar+" ", ";", ""), 0, 32),
)

// cookieDate generates sane-cookie-date, the rfc1123-date of RFC 2616
// §3.3.1, which is case-sensitive, so only the attribute name varies in
// case.
var cookieDate = pbt.Custom("sane-cookie-date", func(t *pbt.T) string {
	y, m, d := calendarDate(t, years0to9, 0, 9999)
	v := time.Date(y, time.Month(m), d, hours.Draw(t, ""), minutes.Draw(t, ""), seconds.Draw(t, ""), 0, time.UTC)
	return v.Format(imfFixdate)
})
var maxAgeLen = pbt.IntRange(1, 20)

var cookieMaxAge = pbt.Custom("Max-Age", func(t *pbt.T) string {
	n := maxAgeLen.Draw(t, "")
	var b strings.Builder
	for i := range n {
		set := digits
		if i == 0 {
			set = "123456789"
		}
		b.WriteByte(charIn(set).Draw(t, ""))
	}
	return b.String()
})
var (
	domainParts  = pbt.SliceOfN(labelOf(alnum), 1, 4)
	cookieDomain = pbt.Map(domainParts, func(ls []string) string { return fitLabels(ls, 253) })
)
