// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package format

import (
	"strings"
	"sync"

	"go.ymotongpoo.com/pbt"
)

// Character sets of the e-mail grammars, the simplest character first.
const (
	// The atoms of Email: letters, digits and the punctuation that most
	// validators accept in a local part.
	commonAtext = alnum + "_+-"
	// qtextSMTP of RFC 5321 §4.1.2: printable ASCII and space, without
	// the double quote and the backslash.
	qtextSMTP = alnum + " !#$%&'()*+,-./:;<=>?@[]^_`{|}~"
	// qtext of RFC 5322 §3.2.4: the same without the space, which RFC
	// 5322 writes as folding white space instead.
	qtext5322 = alnum + "!#$%&'()*+,-./:;<=>?@[]^_`{|}~"
	// VCHAR, the characters a quoted-pair can escape besides white space.
	vchar = alnum + "!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~"
	// dtext of RFC 5322 §3.4.1: printable ASCII without [, ] and \.
	dtext = alnum + "!\"#$%&'()*+,-./:;<=>?@^_`{|}~"
)

// atomGens caches the generators of atomOf.
var atomGens sync.Map // [2]any{set, maxLen} -> pbt.Gen[string]

// atomOf generates atoms of 1 to maxLen characters of set, with a length
// drawn like an integer so that maxLen is common, each in a span of its own
// whose first choice is the length.
func atomOf(set string, maxLen int) pbt.Gen[string] {
	key := [2]any{set, maxLen}
	if g, ok := atomGens.Load(key); ok {
		return g.(pbt.Gen[string])
	}
	lens := pbt.IntRange(1, maxLen)
	g, _ := atomGens.LoadOrStore(key, pbt.Custom("atom", func(t *pbt.T) string {
		return drawChars(t, set, lens.Draw(t, ""))
	}))
	return g.(pbt.Gen[string])
}

// joinAtoms joins atoms with dots, keeping as many as fit in maxLen
// characters and cutting the first to maxLen when it alone is longer.
func joinAtoms(atoms []string, maxLen int) string {
	var b strings.Builder
	for i, a := range atoms {
		if i > 0 {
			if b.Len()+1+len(a) > maxLen {
				break
			}
			b.WriteByte('.')
		}
		b.WriteString(a[:min(len(a), maxLen-b.Len())])
	}
	return b.String()
}

var (
	commonLocal = pbt.Frequency(
		pbt.W(15, pbt.Map(pbt.SliceOfN(atomOf(commonAtext, 20), 1, 4), func(as []string) string { return joinAtoms(as, 64) })),
		pbt.W(1, concat(fixedString(commonAtext, 31), pbt.Just("."), fixedString(commonAtext, 32))),
	)
	emailLabels = pbt.SliceOfN(labelOf(alnum), 1, 3)
	tldLen      = pbt.IntRange(2, 10)
	tldGen      = pbt.Custom("tld", func(t *pbt.T) string {
		return drawChars(t, alpha, tldLen.Draw(t, ""))
	})
)

// Email generates e-mail addresses in the form most validators accept,
// which is also valid under RFC 5321, RFC 5322 and the HTML standard: a
// local part of one to four atoms of letters, digits, "_", "+" and "-"
// joined by single dots, at most 64 characters (RFC 5321 §4.5.3.1.1) and
// exactly 64 in one case in sixteen, and a domain of one to three labels
// of letters, digits and hyphens followed by a top-level label of two to
// ten letters, at most 254 characters in all. Letters are in both cases.
// net/mail.ParseAddress reads every value. Values shrink toward "a@a.aa".
//
// For the whole syntax a mail system must accept, use RFC5321Mailbox or
// RFC5322AddrSpec.
func Email() pbt.Gen[string] {
	return pbt.Custom("Email()", func(t *pbt.T) string {
		local := commonLocal.Draw(t, "")
		labels := append(emailLabels.Draw(t, ""), tldGen.Draw(t, ""))
		budget := 254 - len(local) - 1
		for len(labels) > 2 && len(strings.Join(labels, ".")) > budget {
			labels = labels[1:]
		}
		return local + "@" + strings.Join(labels, ".")
	})
}

var (
	htmlLocal  = stringIn(atext+".", 1, 64)
	htmlLabels = pbt.SliceOfN(labelOf(alnum), 1, 5)
)

// HTMLEmail generates the valid e-mail addresses of the HTML standard
// (§4.10.5.1.5), which an input element of type email accepts: a local
// part of one to 64 characters of atext and dots in any arrangement,
// including a leading, trailing or doubled dot, which RFC 5322 forbids and
// the standard allows on purpose, and a domain of one to five labels of
// letters, digits and hyphens that start and end with a letter or a digit,
// at most 63 characters each. Values shrink toward "a@a".
func HTMLEmail() pbt.Gen[string] {
	return pbt.Custom("HTMLEmail()", func(t *pbt.T) string {
		return htmlLocal.Draw(t, "") + "@" + strings.Join(htmlLabels.Draw(t, ""), ".")
	})
}

// quotedOf returns the generator of quoted strings whose items are drawn
// by item, at most maxLen octets including the quotes; each item is in a
// span of its own.
func quotedOf(item pbt.Gen[string], maxLen int) pbt.Gen[string] {
	items := pbt.SliceOfN(item, 0, 30)
	return pbt.Custom("quoted-string", func(t *pbt.T) string {
		var kept []string
		n := 2
		for _, it := range items.Draw(t, "") {
			if n+len(it) > maxLen {
				break
			}
			kept = append(kept, it)
			n += len(it)
		}
		return `"` + joinWS(kept) + `"`
	})
}

func escaped(set string) pbt.Gen[string] {
	return pbt.Map(charIn(set), func(c byte) string { return `\` + string(c) })
}

func char(set string) pbt.Gen[string] {
	return pbt.Map(charIn(set), func(c byte) string { return string(c) })
}

var (
	// Quoted-string of RFC 5321 §4.1.2: qtextSMTP and quoted-pairSMTP,
	// a backslash and any of %d32-126.
	quotedSMTP = quotedOf(pbt.Frequency(pbt.W(9, char(qtextSMTP)), pbt.W(1, escaped(vchar+" "))), 64)
	// quoted-string of RFC 5322 §3.2.4: qtext, quoted-pair of VCHAR or
	// WSP, and folding white space.
	quoted5322 = quotedOf(pbt.Frequency(pbt.W(8, char(qtext5322)), pbt.W(1, escaped(vchar+" \t")), pbt.W(1, fws)), 130)

	smtpLocal = pbt.Frequency(
		pbt.W(4, pbt.Map(pbt.SliceOfN(atomOf(atext, 64), 1, 16), func(as []string) string { return joinAtoms(as, 64) })),
		pbt.W(1, quotedSMTP),
	)
	smtpLabels = hostLabels(alnum, alnum, 4)
	ipv6SMTP   = ipv6Text("IPv6-addr", 2, true)
	ipv6Tag    = literalCase("IPv6:", 9, 1)

	// smtpDomain draws a Domain or an address literal of RFC 5321; a
	// domain is fitted to the octets the local part leaves.
	smtpDomain = pbt.Frequency(
		pbt.W(5, pbt.Map(smtpLabels, func(ls []string) func(int) string {
			return func(budget int) string { return fitLabels(ls, budget) }
		})),
		pbt.W(1, pbt.Custom("IPv4-address-literal", func(t *pbt.T) func(int) string {
			b := ipv4Addrs.Draw(t, "").As4()
			var parts [4]string
			for i, o := range b {
				parts[i] = snum(o, octetPad.Draw(t, ""))
			}
			s := "[" + strings.Join(parts[:], ".") + "]"
			return func(int) string { return s }
		})),
		pbt.W(1, pbt.Custom("IPv6-address-literal", func(t *pbt.T) func(int) string {
			s := "[" + ipv6Tag.Draw(t, "") + ipv6SMTP.Draw(t, "") + "]"
			return func(int) string { return s }
		})),
	)
)

// RFC5321Mailbox generates mailboxes in the syntax of RFC 5321 §4.1.2, the
// address of SMTP's MAIL and RCPT commands. The Local-part is a Dot-string
// of one to sixteen atext atoms of up to 64 characters each, or a
// Quoted-string, which may contain spaces and backslash escapes, at most
// 64 octets in all (§4.5.3.1.1). The domain is a Domain of one to five
// labels of letters, digits and hyphens that start and end with a letter
// or a digit, any of them with a digit, as in "3com", at most 63
// characters each; or an address literal (§4.1.3): an IPv4 address in
// brackets whose octets may have leading zeros, as Snum allows, such as
// "[192.000.002.001]", or "IPv6:" in either case and an IPv6 address in
// any form of §4.1.3, where "::" stands for at least two zero groups and
// an embedded IPv4 address may also have leading zeros. A mailbox has at
// most 254 octets, the 256 of a path (§4.5.3.1.3) without the angle
// brackets. A General-address-literal needs a tag registered with IANA,
// and the only one is IPv6, which the IPv6 literal covers. Values shrink
// toward "a@a"; a part whose other forms would take more choices keeps
// its form, so a quoted local part can shrink toward the empty quoted
// string and an address literal toward [0.0.0.0] or [IPv6:::], as in ""@a
// and a@[0.0.0.0].
//
// net/mail.ParseAddress (Go 1.26) rejects some of these: the empty quoted
// string, IPv4 literals with leading zeros, and IPv6 literals.
func RFC5321Mailbox() pbt.Gen[string] {
	return pbt.Custom("RFC5321Mailbox()", func(t *pbt.T) string {
		local := smtpLocal.Draw(t, "")
		return local + "@" + smtpDomain.Draw(t, "")(min(253, 254-len(local)-1))
	})
}

var (
	local5322 = pbt.Frequency(
		pbt.W(4, pbt.Map(pbt.SliceOfN(atomOf(atext, 64), 1, 8), func(as []string) string { return strings.Join(as, ".") })),
		pbt.W(2, quoted5322),
	)
	domain5322 = pbt.Frequency(
		pbt.W(4, pbt.Map(pbt.SliceOfN(atomOf(atext, 63), 1, 8), func(as []string) string { return strings.Join(as, ".") })),
		pbt.W(1, pbt.Custom("domain-literal", func(t *pbt.T) string {
			return "[" + literalBody.Draw(t, "") + "]"
		})),
	)
	literalBody = pbt.Frequency(
		pbt.W(2, IPv4()),
		pbt.W(2, concat(pbt.Just("IPv6:"), ipv6Text("IPv6", 1, false))),
		pbt.W(1, pbt.Map(pbt.SliceOfN(pbt.Frequency(pbt.W(9, char(dtext)), pbt.W(1, fws)), 0, 30), joinWS)),
	)
	aroundCFWS = optional(9, 1, cfws)
)

// RFC5322AddrSpec generates addresses in the addr-spec syntax of RFC 5322
// §3.4.1. The local-part is a dot-atom of one to eight atext atoms of up to
// 64 characters, which RFC 5322 does not limit as RFC 5321 does, or a
// quoted-string with escapes and folding white space; the domain is a
// dot-atom of one to eight atoms of any atext, not only the letters, digits
// and hyphens of host names, or a domain-literal in brackets of dtext and
// folding white space, often an IPv4 address or "IPv6:" and an IPv6
// address. Each of the two parts has an optional CFWS on either side, in
// one case in ten each, with comments, nested comments and folded lines.
// The obsolete syntax of §4.4, which §4 forbids generators to produce, is
// left out. Values shrink toward "a@a"; a part whose other forms would take
// more choices keeps its form, so a quoted local part can shrink toward the
// empty quoted string and a domain literal toward [], [0.0.0.0] or
// [IPv6:::], as in ""@a and a@[].
//
// net/mail.ParseAddress (Go 1.26) rejects some of these: the empty quoted
// string, domain literals other than a plain IPv4 address, comments before
// the domain or around the local part, and folded lines.
func RFC5322AddrSpec() pbt.Gen[string] {
	return pbt.Custom("RFC5322AddrSpec()", func(t *pbt.T) string {
		return aroundCFWS.Draw(t, "") + local5322.Draw(t, "") + aroundCFWS.Draw(t, "") + "@" +
			aroundCFWS.Draw(t, "") + domain5322.Draw(t, "") + aroundCFWS.Draw(t, "")
	})
}
