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

var (
	localMode = choice(15, 1) // atoms as drawn, a local part of exactly 64 characters
	atomCount = pbt.IntRange(0, 3)
	atomLen   = pbt.IntRange(1, 20)
	tldLen    = pbt.IntRange(2, 10)
	domainN   = pbt.IntRange(1, 3)
)

// Email generates e-mail addresses in the form most validators accept,
// which is also valid under RFC 5321, RFC 5322 and the HTML standard: a
// local part of atoms of letters, digits, "_", "+" and "-" joined by
// single dots, at most 64 characters (RFC 5321 §4.5.3.1.1), and a domain
// of one to three labels of letters, digits and hyphens followed by a
// top-level label of two to ten letters, at most 254 characters in all.
// Letters are in both cases. Values shrink toward "a@a.aa".
//
// For the whole syntax a mail system must accept, use RFC5321Mailbox or
// RFC5322AddrSpec.
func Email() pbt.Gen[string] {
	return pbt.Custom("Email()", func(t *pbt.T) string {
		local := dotAtom(t, commonAtext, 64)
		budget := 254 - len(local) - 1
		n := domainN.Draw(t, "")
		labels := make([]string, 0, n+1)
		for range n {
			labels = append(labels, labelOf(alnum).Draw(t, ""))
		}
		tld := tldGen.Draw(t, "")
		for len(labels) > 1 && len(strings.Join(labels, "."))+1+len(tld) > budget {
			labels = labels[1:]
		}
		return local + "@" + strings.Join(append(labels, tld), ".")
	})
}

// tldGen generates the top-level label of Email: two to ten letters.
var tldGen = pbt.Custom("tld", func(t *pbt.T) string {
	b := make([]byte, tldLen.Draw(t, ""))
	for i := range b {
		b[i] = charIn(alpha).Draw(t, "")
	}
	return string(b)
})

// atomGens caches the generators of atomOf.
var atomGens sync.Map // string -> pbt.Gen[string]

// atomOf generates atoms of 1 to 20 characters of set, each in a span of
// its own whose first choice is the length.
func atomOf(set string) pbt.Gen[string] {
	if g, ok := atomGens.Load(set); ok {
		return g.(pbt.Gen[string])
	}
	g, _ := atomGens.LoadOrStore(set, pbt.Custom("atom", func(t *pbt.T) string {
		b := make([]byte, atomLen.Draw(t, ""))
		for i := range b {
			b[i] = charIn(set).Draw(t, "")
		}
		return string(b)
	}))
	return g.(pbt.Gen[string])
}

// dotAtom draws atoms of set joined by single dots, at most maxLen
// characters, and in a share of the cases exactly maxLen.
func dotAtom(t *pbt.T, set string, maxLen int) string {
	var atoms []string
	if localMode.Draw(t, "") == 1 {
		for _, n := range []int{maxLen/2 - 1, maxLen - maxLen/2} {
			b := make([]byte, n)
			for i := range b {
				b[i] = charIn(set).Draw(t, "")
			}
			atoms = append(atoms, string(b))
		}
	} else {
		for range atomCount.Draw(t, "") + 1 {
			atoms = append(atoms, atomOf(set).Draw(t, ""))
		}
	}
	var b strings.Builder
	for i, a := range atoms {
		if i > 0 {
			if b.Len()+2 > maxLen {
				break
			}
			b.WriteByte('.')
		}
		b.WriteString(a[:min(len(a), maxLen-b.Len())])
	}
	return b.String()
}

// HTMLEmail generates the valid e-mail addresses of the HTML standard
// (§4.10.5.1.5), which an input element of type email accepts: a local
// part of one to 64 characters of atext and dots in any arrangement,
// including a leading, trailing or doubled dot, which RFC 5322 forbids and
// the standard allows on purpose, and a domain of one to five labels of
// letters, digits and hyphens that start and end with a letter or a digit,
// at most 63 characters each. Values shrink toward "a@a".
func HTMLEmail() pbt.Gen[string] {
	local := stringIn(atext+".", 1, 64)
	return pbt.Custom("HTMLEmail()", func(t *pbt.T) string {
		l := local.Draw(t, "")
		n := labelCount.Draw(t, "")
		labels := make([]string, n+1)
		for i := range labels {
			labels[i] = labelOf(alnum).Draw(t, "")
		}
		return l + "@" + strings.Join(labels, ".")
	})
}

var (
	localForm   = choice(4, 1)    // dot-string or dot-atom, quoted string
	domainForm  = choice(5, 1, 1) // a domain, an IPv4 literal, an IPv6 literal
	quotedItem  = choice(8, 1, 1) // a character, an escaped character, white space
	quotedLen   = pbt.IntRange(0, 30)
	ipv6TagCase = choice(9, 1) // "IPv6" as RFC 5321 writes it, in lower case
)

// RFC5321Mailbox generates mailboxes in the syntax of RFC 5321 §4.1.2, the
// address of SMTP's MAIL and RCPT commands: a Local-part that is a
// Dot-string of atext atoms or a Quoted-string, which may contain spaces
// and backslash escapes, at most 64 octets (§4.5.3.1.1), then "@" and a
// Domain of labels of letters, digits and hyphens, at most 63 characters
// each, or an address literal: an IPv4 address in brackets, such as
// "[192.0.2.1]", or "IPv6:" and an IPv6 address in any form of §4.1.3,
// where "::" stands for at least two zero groups. A mailbox has at most
// 254 octets, the 256 of a path (§4.5.3.1.3) without the angle brackets.
// The General-address-literal, which no registered tag uses, is not
// generated. Values shrink toward "a@a"; the shrinker keeps the form of
// each part, so a quoted local part shrinks toward the empty quoted string
// and an address literal toward [0.0.0.0], as in ""@a and a@[0.0.0.0].
func RFC5321Mailbox() pbt.Gen[string] {
	addrs6, addrs4 := pbt.IPv6(), pbt.IPv4()
	return pbt.Custom("RFC5321Mailbox()", func(t *pbt.T) string {
		var local string
		if localForm.Draw(t, "") == 0 {
			local = dotAtom(t, atext, 64)
		} else {
			local = quotedSMTP.Draw(t, "")
		}
		budget := 254 - len(local) - 1
		var domain string
		switch domainForm.Draw(t, "") {
		case 0:
			domain = hostname(t, alnum, min(budget, 253))
		case 1:
			domain = "[" + addrs4.Draw(t, "").String() + "]"
		default:
			tag := "IPv6:"
			if ipv6TagCase.Draw(t, "") == 1 {
				tag = "ipv6:"
			}
			domain = "[" + tag + ipv6Text(t, addrs6, 2) + "]"
		}
		return local + "@" + domain
	})
}

// The quoted strings of RFC 5321 and RFC 5322, each in a span of its own
// whose first choice is the number of items.
var (
	quotedSMTP = pbt.Custom("quoted", func(t *pbt.T) string { return quoted(t, qtextSMTP, false, 64) })
	quoted5322 = pbt.Custom("quoted", func(t *pbt.T) string { return quoted(t, qtext5322, true, 66) })
)

// quoted draws a quoted string of at most maxLen octets: characters of
// set, backslash escapes, and, when wsp is set, runs of spaces and tabs,
// which RFC 5322 writes as folding white space. An escape takes a
// character of VCHAR, a space or a tab.
func quoted(t *pbt.T, set string, wsp bool, maxLen int) string {
	var b strings.Builder
	b.WriteByte('"')
	for range quotedLen.Draw(t, "") {
		var item string
		switch quotedItem.Draw(t, "") {
		case 0:
			item = string(charIn(set).Draw(t, ""))
		case 1:
			item = "\\" + string(charIn(vchar+" \t").Draw(t, ""))
			if !wsp && item == "\\\t" {
				item = "\\ " // quoted-pairSMTP takes %d32-126, no tab
			}
		default:
			if wsp {
				item = stringIn(" \t", 1, 3).Draw(t, "")
			} else {
				item = " "
			}
		}
		if b.Len()+len(item)+1 > maxLen {
			break
		}
		b.WriteString(item)
	}
	b.WriteByte('"')
	return b.String()
}

var literalForm = choice(2, 2, 1) // an IPv4 address, an IPv6 address, any dtext

// RFC5322AddrSpec generates addresses in the addr-spec syntax of RFC 5322
// §3.4.1: a local-part that is a dot-atom of atext or a quoted-string,
// which may hold escapes and runs of spaces and tabs, then "@" and a
// domain that is a dot-atom, whose atoms may hold any atext and not only
// the letters, digits and hyphens of host names, or a domain-literal in
// brackets of dtext and white space, often an IPv4 address or "IPv6:" and
// an IPv6 address. RFC 5322 sets no length limits, and comments, line
// folding and the obsolete syntax of §4.4 are not generated. Values shrink
// toward "a@a"; the shrinker keeps the form of each part, so a quoted local
// part shrinks toward the empty quoted string and a domain literal toward
// the empty literal, as in ""@a and a@[].
func RFC5322AddrSpec() pbt.Gen[string] {
	addrs6, addrs4 := pbt.IPv6(), pbt.IPv4()
	return pbt.Custom("RFC5322AddrSpec()", func(t *pbt.T) string {
		var local string
		if localForm.Draw(t, "") == 0 {
			local = dotAtom(t, atext, 64)
		} else {
			local = quoted5322.Draw(t, "")
		}
		var domain string
		switch domainForm.Draw(t, "") {
		case 0:
			domain = dotAtom(t, atext, 253)
		default:
			var body string
			switch literalForm.Draw(t, "") {
			case 0:
				body = addrs4.Draw(t, "").String()
			case 1:
				body = "IPv6:" + ipv6Text(t, addrs6, 1)
			default:
				var b strings.Builder
				for range quotedLen.Draw(t, "") {
					if quotedItem.Draw(t, "") == 2 {
						b.WriteString(stringIn(" \t", 1, 3).Draw(t, ""))
					} else {
						b.WriteByte(charIn(dtext).Draw(t, ""))
					}
				}
				body = b.String()
			}
			domain = "[" + body + "]"
		}
		return local + "@" + domain
	})
}
