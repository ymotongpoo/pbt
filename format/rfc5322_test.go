// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package format

import "strings"

// parser5322 reads the current syntax of RFC 5322 §3.2 to §3.4, written
// for the tests from the ABNF and independently of the generators.
type parser5322 struct {
	s   string
	i   int
	bad bool
}

func (p *parser5322) peek(c byte) bool { return p.i < len(p.s) && p.s[p.i] == c }

func (p *parser5322) in(set string) bool {
	if p.i < len(p.s) && strings.IndexByte(set, p.s[p.i]) >= 0 {
		p.i++
		return true
	}
	return false
}

// fws reads FWS = ([*WSP CRLF] 1*WSP) and reports whether it read any.
func (p *parser5322) fws() bool {
	start := p.i
	for p.in(" \t") {
	}
	if strings.HasPrefix(p.s[p.i:], "\r\n") {
		p.i += 2
		if !p.in(" \t") {
			p.bad = true // CRLF must be followed by white space
			return true
		}
		for p.in(" \t") {
		}
	}
	return p.i > start
}

// comment reads comment = "(" *([FWS] ccontent) [FWS] ")".
func (p *parser5322) comment() {
	p.i++ // "("
	for !p.bad {
		p.fws()
		switch {
		case p.i >= len(p.s):
			p.bad = true
		case p.peek(')'):
			p.i++
			return
		case p.peek('('):
			p.comment()
		case p.peek('\\'):
			p.i++
			if !p.in(vchar + " \t") {
				p.bad = true
			}
		case !p.in(ctext):
			p.bad = true
		}
	}
}

// cfws reads an optional CFWS = (1*([FWS] comment) [FWS]) / FWS.
func (p *parser5322) cfws() {
	for !p.bad {
		save := p.i
		p.fws()
		if !p.peek('(') {
			p.i = save
			break
		}
		p.comment()
	}
	p.fws()
}

// quoted reads quoted-string = DQUOTE *([FWS] qcontent) [FWS] DQUOTE.
func (p *parser5322) quoted() {
	p.i++ // DQUOTE
	for !p.bad {
		p.fws()
		switch {
		case p.i >= len(p.s):
			p.bad = true
		case p.peek('"'):
			p.i++
			return
		case p.peek('\\'):
			p.i++
			if !p.in(vchar + " \t") {
				p.bad = true
			}
		case !p.in(qtext5322):
			p.bad = true
		}
	}
}

// dotAtom reads dot-atom-text = 1*atext *("." 1*atext).
func (p *parser5322) dotAtom() {
	for {
		start := p.i
		for p.in(atext) {
		}
		if p.i == start {
			p.bad = true
			return
		}
		if !p.in(".") {
			return
		}
	}
}

// domainLiteral reads "[" *([FWS] dtext) [FWS] "]".
func (p *parser5322) domainLiteral() {
	p.i++ // "["
	for !p.bad {
		p.fws()
		switch {
		case p.i >= len(p.s):
			p.bad = true
		case p.peek(']'):
			p.i++
			return
		case !p.in(dtext):
			p.bad = true
		}
	}
}

// parseAddrSpec checks addr-spec = local-part "@" domain of §3.4.1, with
// the CFWS that dot-atom, quoted-string and domain-literal allow on either
// side.
func parseAddrSpec(s string) string {
	p := &parser5322{s: s}
	p.cfws()
	if p.peek('"') {
		p.quoted()
	} else {
		p.dotAtom()
	}
	p.cfws()
	if p.bad || !p.in("@") {
		return "bad local-part"
	}
	p.cfws()
	if p.peek('[') {
		p.domainLiteral()
	} else {
		p.dotAtom()
	}
	p.cfws()
	if p.bad || p.i != len(s) {
		return "bad domain"
	}
	return ""
}
