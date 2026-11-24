// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package format

import (
	"net/mail"
	"regexp"
	"strings"
	"testing"

	"go.ymotongpoo.com/pbt"
)

const (
	atextRe   = "[A-Za-z0-9!#$%&'*+/=?^_`{|}~-]"
	dotAtomRe = atextRe + `+(?:\.` + atextRe + `+)*`
	labelRe   = `[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?`
)

var (
	emailRe     = regexp.MustCompile(`^[A-Za-z0-9_+-]+(?:\.[A-Za-z0-9_+-]+)*@(?:` + labelRe + `\.)+[A-Za-z]{2,10}$`)
	htmlRe      = regexp.MustCompile("^[a-zA-Z0-9.!#$%&'*+\\/=?^_`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$")
	smtpLocalRe = regexp.MustCompile(`^(?:` + dotAtomRe + `|"(?:[\x20\x21\x23-\x5b\x5d-\x7e]|\\[\x20-\x7e])*")$`)
	smtpHostRe  = regexp.MustCompile(`^` + labelRe + `(?:\.` + labelRe + `)*$`)
	ipv6TagRe   = regexp.MustCompile(`^\[(?i:IPv6):(.*)\]$`)
)

// splitAddr splits an address at the @ after its local part: the end of
// a quoted string, whose escapes and contents may hold @, or the first @
// of a dot-atom, which has none.
func splitAddr(s string) (local, domain string) {
	if strings.HasPrefix(s, `"`) {
		for i := 1; i < len(s); i++ {
			switch s[i] {
			case '\\':
				i++
			case '"':
				return s[:i+1], s[i+2:]
			}
		}
	}
	i := strings.IndexByte(s, '@')
	return s[:i], s[i+1:]
}

func TestEmail(t *testing.T) {
	checkValid(t, Email(), func(s string) string {
		local, _ := splitAddr(s)
		switch {
		case !emailRe.MatchString(s):
			return "not in the common form"
		case len(local) > 64 || len(s) > 254:
			return "too long"
		case !htmlRe.MatchString(s):
			return "not a valid HTML e-mail address"
		}
		if _, err := mail.ParseAddress(s); err != nil {
			return "net/mail rejects it: " + err.Error()
		}
		return ""
	})
	pbt.Check(t, func(pt *pbt.T) {
		s := Email().Draw(pt, "s")
		local, _ := splitAddr(s)
		pt.Cover(strings.Contains(local, "."), 20, "dotted local part")
		pt.Cover(strings.Contains(local, "+"), 5, "plus")
		pt.Cover(len(local) == 64, 2, "64-character local part")
		pt.Cover(strings.ContainsAny(s, upper), 20, "upper case")
	}, pbt.Seed(1), pbt.Checks(2000), pbt.NoSave(), pbt.CheckCoverage())
	shrinksTo(t, Email(), always, "a@a.aa")
}

func TestHTMLEmail(t *testing.T) {
	checkValid(t, HTMLEmail(), func(s string) string {
		if !htmlRe.MatchString(s) {
			return "not a valid HTML e-mail address"
		}
		return ""
	})
	pbt.Check(t, func(pt *pbt.T) {
		local, _ := splitAddr(HTMLEmail().Draw(pt, "s"))
		pt.Cover(strings.HasPrefix(local, ".") || strings.HasSuffix(local, ".") || strings.Contains(local, ".."), 2, "dots RFC 5322 forbids")
	}, pbt.Seed(1), pbt.Checks(2000), pbt.NoSave(), pbt.CheckCoverage())
	shrinksTo(t, HTMLEmail(), always, "a@a")
}

// checkMailbox checks a Mailbox of RFC 5321 §4.1.2 and §4.1.3.
func checkMailbox(s string) string {
	local, domain := splitAddr(s)
	switch {
	case !smtpLocalRe.MatchString(local):
		return "bad Local-part"
	case len(local) > 64 || len(s) > 254:
		return "too long"
	case strings.HasPrefix(domain, "["):
		if m := ipv6TagRe.FindStringSubmatch(domain); m != nil {
			return checkRFC4291(m[1], 2, true)
		}
		if _, ok := checkSnum(strings.Trim(domain, "[]"), true); !ok {
			return "bad IPv4 literal"
		}
	case !smtpHostRe.MatchString(domain) || len(domain) > 253:
		return "bad Domain"
	}
	return ""
}

func TestRFC5321Mailbox(t *testing.T) {
	checkValid(t, RFC5321Mailbox(), checkMailbox)
	longAtom := regexp.MustCompile(`(^|\.)` + atextRe + `{21,}`)
	pbt.Check(t, func(pt *pbt.T) {
		s := RFC5321Mailbox().Draw(pt, "s")
		local, domain := splitAddr(s)
		quoted := strings.HasPrefix(local, `"`)
		pt.Cover(quoted, 10, "quoted")
		pt.Cover(quoted && strings.Contains(local, " "), 3, "space in quotes")
		pt.Cover(strings.Contains(local, `\`), 2, "escape")
		pt.Cover(!quoted && longAtom.MatchString(local), 5, "atom of more than 20 characters")
		pt.Cover(!quoted && strings.Count(local, ".") >= 4, 1, "five atoms or more")
		pt.Cover(len(local) == 64, 2, "64-octet local part")
		pt.Cover(ipv6TagRe.MatchString(domain), 5, "IPv6 literal")
		pt.Cover(strings.HasPrefix(domain, "[") && !strings.Contains(domain, ":"), 5, "IPv4 literal")
		pt.Cover(regexp.MustCompile(`[\[.:]0\d`).MatchString(domain), 1, "leading zero in an octet")
		pt.Cover(!strings.HasPrefix(domain, "[") && regexp.MustCompile(`(^|\.)\d[^.]*$`).MatchString(domain), 2, "last label starting with a digit")
	}, pbt.Seed(1), pbt.Checks(2000), pbt.NoSave(), pbt.CheckCoverage())
	var all []string
	for _, l := range []string{"a", `""`} {
		for _, d := range []string{"a", "[0.0.0.0]", "[IPv6:::]"} {
			all = append(all, l+"@"+d)
		}
	}
	shrinksTo(t, RFC5321Mailbox(), always, all...)
	shrinksTo(t, RFC5321Mailbox(), func(s string) bool {
		l, d := splitAddr(s)
		return !strings.HasPrefix(l, `"`) && !strings.HasPrefix(d, "[")
	}, "a@a")
}

func TestRFC5322AddrSpec(t *testing.T) {
	checkValid(t, RFC5322AddrSpec(), parseAddrSpec)
	pbt.Check(t, func(pt *pbt.T) {
		s := RFC5322AddrSpec().Draw(pt, "s")
		pt.Cover(strings.Contains(s, "("), 3, "comment")
		pt.Cover(strings.Contains(s, "\r\n"), 1, "folded line")
		pt.Cover(regexp.MustCompile(`^"`).MatchString(s), 10, "quoted local part")
		m := regexp.MustCompile(`^(` + dotAtomRe + `)@`).FindStringSubmatch(s)
		pt.Cover(m != nil && len(m[1]) > 64, 2, "unquoted local part beyond 64 characters")
		pt.Cover(strings.Contains(s, "@["), 10, "domain literal")
		pt.Cover(regexp.MustCompile("@[a-zA-Z0-9.-]*[!#$%&'*+/=?^_`{|}~]").MatchString(s), 5, "atext beyond host names in the domain")
	}, pbt.Seed(1), pbt.Checks(2000), pbt.NoSave(), pbt.CheckCoverage())
	var all []string
	for _, l := range []string{"a", `""`} {
		for _, d := range []string{"a", "[]", "[0.0.0.0]", "[IPv6:::]"} {
			all = append(all, l+"@"+d)
		}
	}
	shrinksTo(t, RFC5322AddrSpec(), always, all...)
}

// TestGoMailAddressDivergences pins what the godoc of RFC5321Mailbox and
// RFC5322AddrSpec says about net/mail.ParseAddress.
func TestGoMailAddressDivergences(t *testing.T) {
	for _, s := range []string{`""@a`, `a@[001.002.003.004]`, `a@[IPv6:::]`, `a@[IPv6:1::2]`, `a@[]`, `a@[a b]`, `(c)a@b`, `a(c)@b`, `a@(c)b`, "a@b\r\n (c)"} {
		if checkMailbox(s) != "" && parseAddrSpec(s) != "" {
			t.Fatalf("%q is valid under neither RFC 5321 nor RFC 5322", s)
		}
		if _, err := mail.ParseAddress(s); err == nil {
			t.Errorf("net/mail.ParseAddress now accepts %q; update the godoc", s)
		}
	}
	for _, s := range []string{`a@b(c)`, `a@[1.2.3.4]`, `"a b"@c`, "\"a\tb\"@c", `a@3com`, `a@b!c`} {
		if _, err := mail.ParseAddress(s); err != nil {
			t.Errorf("net/mail.ParseAddress now rejects %q: %v; update the godoc", s, err)
		}
	}
}
