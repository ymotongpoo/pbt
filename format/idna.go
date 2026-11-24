// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package format

import (
	"strings"

	"go.ymotongpoo.com/pbt"
)

// punycode implements RFC 3492 §§6.1, 6.3, without case annotations.
// Callers pass at most 256 Unicode scalar values (the generators use 57).
// int64 keeps delta arithmetic safe even on 32-bit platforms under this bound.
func punycode(s string) string {
	input := []rune(s)
	var out strings.Builder
	for _, r := range input {
		if r < 128 {
			out.WriteByte(byte(r))
		}
	}
	basic := out.Len()
	if basic > 0 {
		out.WriteByte('-')
	}
	n, delta, bias := int64(128), int64(0), int64(72)
	for handled := basic; handled < len(input); n, delta = n+1, delta+1 {
		m := int64(0x110000)
		for _, r := range input {
			if int64(r) >= n && int64(r) < m {
				m = int64(r)
			}
		}
		delta += (m - n) * int64(handled+1)
		n = m
		for _, r := range input {
			if int64(r) < n {
				delta++
			}
			if int64(r) != n {
				continue
			}
			q := delta
			for k := int64(36); ; k += 36 {
				threshold := max(int64(1), min(int64(26), k-bias))
				if q < threshold {
					break
				}
				out.WriteByte(punyDigit(threshold + (q-threshold)%(36-threshold)))
				q = (q - threshold) / (36 - threshold)
			}
			out.WriteByte(punyDigit(q))
			bias = punyAdapt(delta, int64(handled+1), handled == basic)
			delta = 0
			handled++
		}
	}
	return out.String()
}

func punyDigit(d int64) byte {
	if d < 26 {
		return byte(d) + 'a'
	}
	return byte(d-26) + '0'
}

func punyAdapt(delta, points int64, first bool) int64 {
	if first {
		delta /= 700
	} else {
		delta /= 2
	}
	delta += delta / points
	k := int64(0)
	for delta > 455 {
		delta /= 35
		k += 36
	}
	return k + 36*delta/(delta+38)
}

// These precomposed letters are PVALID under RFC 5892, have bidi class L,
// and cannot combine with one another. No normalization library is needed.
const idnaLetters = "éöüαβγабвあいうアイウ一丁七"

var (
	idnaFirst    = pbt.SampledFrom([]rune(idnaLetters))
	idnaRest     = pbt.SampledFrom([]rune("abcdefghijklmnopqrstuvwxyz0123456789" + idnaLetters))
	idnaLength   = pbt.IntRange(1, 57)
	unicodeLabel = pbt.Custom("U-label", func(t *pbt.T) string {
		n := idnaLength.Draw(t, "")
		rs := make([]rune, n)
		rs[0] = idnaFirst.Draw(t, "")
		for i := 1; i < n; i++ {
			rs[i] = idnaRest.Draw(t, "")
		}
		// Trim complete trailing code points, never bytes or an encoded A-label.
		for len("xn--"+punycode(string(rs))) > 63 {
			rs = rs[:len(rs)-1]
		}
		return string(rs)
	})
	unicodeLabels = pbt.SliceOfN(unicodeLabel, 1, 5)
)

// fitIDNALabels retains whole labels within both the DNS A-label budget and
// the wire UTF-8 budget. A first label always fits the callers' budgets.
func fitIDNALabels(labels []string, ascii bool, budget int) string {
	var out []string
	wire, dns := 0, 0
	for _, u := range labels {
		a := "xn--" + punycode(u)
		s := u
		if ascii {
			s = a
		}
		sep := 0
		if len(out) > 0 {
			sep = 1
		}
		if wire+sep+len(s) > budget || dns+sep+len(a) > 253 {
			break
		}
		out = append(out, s)
		wire += sep + len(s)
		dns += sep + len(a)
	}
	return strings.Join(out, ".")
}

// IDNAHostname generates ASCII host names with one to five A-labels
// (RFC 5890 §2.3.2.1): "xn--" followed by the RFC 3492 Punycode of a
// U-label. Every label contains a non-ASCII letter; labels are at most 63
// octets and the name at most 253 ASCII characters, without a final dot.
// U-labels begin with one of éöüαβγабвあいうアイウ一丁七 and continue with
// those letters, a-z or 0-9, up to 57 code points before fitting to the
// A-label limit. These fixed, precomposed PVALID letters (RFC 5892 §2.1)
// preserve NFC, need no contextual rules, and are left-to-right: RFC 5893's
// Bidi rule is not triggered. Other scripts, combining marks, hyphens,
// ASCII-only labels and registry assignment are not modeled. Values
// shrink toward "xn--9ca" (é).
func IDNAHostname() pbt.Gen[string] {
	return pbt.Map(unicodeLabels, func(ls []string) string { return fitIDNALabels(ls, true, 253) })
}

// UnicodeHostname generates the U-label form of IDNAHostname, with the
// same subset and bounds. In addition to the 253-octet A-label DNS limit,
// it bounds the UTF-8 spelling to 253 octets (and hence at most 253 Unicode
// characters). Complete trailing labels are dropped to fit either limit.
// Values shrink toward "é". net/url.Parse preserves these Unicode hosts;
// it does not convert them to A-labels.
func UnicodeHostname() pbt.Gen[string] {
	return pbt.Map(unicodeLabels, func(ls []string) string { return fitIDNALabels(ls, false, 253) })
}
