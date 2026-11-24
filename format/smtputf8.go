// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package format

import (
	"strings"
	"unicode/utf8"

	"go.ymotongpoo.com/pbt"
)

var (
	// UTF8-non-ascii of RFC 6532 §3.1: all scalar values above ASCII,
	// including noncharacters, but never surrogate code points.
	smtpNonASCII = pbt.Map(pbt.OneOf(
		pbt.IntRange(0x80, 0x7ff),
		pbt.IntRange(0x800, 0xd7ff),
		pbt.IntRange(0xe000, 0xffff),
		pbt.IntRange(0x10000, 0x10ffff),
	), func(r int) string { return string(rune(r)) })
	smtpUTF8AtomChar   = pbt.Frequency(pbt.W(3, char(atext)), pbt.W(1, smtpNonASCII))
	smtpUTF8AtomLength = pbt.IntRange(1, 64)
	smtpUTF8Atom       = pbt.Custom("UTF8-atom", func(t *pbt.T) string {
		n := smtpUTF8AtomLength.Draw(t, "")
		var b strings.Builder
		for range n {
			b.WriteString(smtpUTF8AtomChar.Draw(t, ""))
		}
		return utf8Prefix(b.String(), 64)
	})
	smtpUTF8Local = pbt.Frequency(
		pbt.W(4, pbt.Map(pbt.SliceOfN(smtpUTF8Atom, 1, 16), func(as []string) string { return joinAtoms(as, 64) })),
		pbt.W(1, quotedOf(pbt.Frequency(pbt.W(6, char(qtextSMTP)), pbt.W(2, smtpNonASCII), pbt.W(1, escaped(vchar+" "))), 64)),
	)
	smtpUTF8Domain = pbt.Frequency(
		pbt.W(3, smtpDomain),
		pbt.W(2, pbt.Map(unicodeLabels, func(ls []string) func(int) string {
			return func(budget int) string { return fitIDNALabels(ls, false, budget) }
		})),
	)
)

// utf8Prefix cuts only at a code-point boundary; its input is valid UTF-8.
func utf8Prefix(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for !utf8.RuneStart(s[n]) {
		n--
	}
	return s[:n]
}

// SMTPUTF8Mailbox generates mailboxes using the RFC 6531 §3.3 extension
// of RFC 5321 §4.1.2. Local parts are dot strings (four cases in five) or
// quoted strings (one in five), with every UTF8-non-ascii scalar value
// permitted by RFC 6532 §3.1 available in both forms. ASCII atext,
// qtextSMTP and quoted-pairSMTP are also generated. There are one to
// sixteen atoms, each drawn with one to 64 characters, or zero to thirty
// quoted items; complete characters/items are kept within 64 UTF-8 octets.
//
// Two domains in five use the U-label subset of UnicodeHostname. The
// others use RFC5321Mailbox's ASCII domain and IPv4/IPv6 literals. No
// other U-labels, mixed ASCII/U-label domains or general-address-literal
// tags are generated. The plain name reflects that IDNA subset. DNS
// limits apply to the A-label encoding; the entire mailbox is at most
// 254 UTF-8 octets, not code points (RFC 5321 §4.5.3.1).
//
// Values shrink toward a local part of "a" or the empty quoted string,
// and a domain of "a", "é", "[0.0.0.0]" or "[IPv6:::]", keeping the
// selected forms. net/mail.ParseAddress accepts ordinary Unicode local
// parts and U-label domains but, as for RFC5321Mailbox, rejects empty
// quoted locals and some address literals. It is not an SMTP validator.
func SMTPUTF8Mailbox() pbt.Gen[string] {
	return pbt.Custom("SMTPUTF8Mailbox()", func(t *pbt.T) string {
		local := smtpUTF8Local.Draw(t, "")
		return local + "@" + smtpUTF8Domain.Draw(t, "")(254-len(local)-1)
	})
}
