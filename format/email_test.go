// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package format

import (
	"net/mail"
	"net/netip"
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
	emailRe   = regexp.MustCompile(`^[A-Za-z0-9_+-]+(?:\.[A-Za-z0-9_+-]+)*@(?:` + labelRe + `\.)+[A-Za-z]{2,10}$`)
	htmlRe    = regexp.MustCompile("^[a-zA-Z0-9.!#$%&'*+\\/=?^_`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$")
	smtpLocal = regexp.MustCompile(`^(?:` + dotAtomRe + `|"(?:[\x20\x21\x23-\x5b\x5d-\x7e]|\\[\x20-\x7e])*")$`)
	smtpHost  = regexp.MustCompile(`^` + labelRe + `(?:\.` + labelRe + `)*$`)
	rfc5322L  = regexp.MustCompile(`^(?:` + dotAtomRe + `|"(?:[ \t]*(?:[\x21\x23-\x5b\x5d-\x7e]|\\[\x21-\x7e \t]))*[ \t]*")$`)
	rfc5322D  = regexp.MustCompile(`^(?:` + dotAtomRe + `|\[(?:[ \t]*[\x21-\x5a\x5e-\x7e])*[ \t]*\])$`)
)

// splitAddr splits an address at the @ after its local part: the end of
// a quoted string, whose escapes and contents may hold @, or the first @
// of a dot-atom, which has none. A domain literal may hold @ as well.
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
	if got := minimalOf(t, Email(), always); got != "a@a.aa" {
		t.Errorf("shrinks to %q", got)
	}
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
	if got := minimalOf(t, HTMLEmail(), always); got != "a@a" {
		t.Errorf("shrinks to %q", got)
	}
}

func TestRFC5321Mailbox(t *testing.T) {
	checkValid(t, RFC5321Mailbox(), func(s string) string {
		local, domain := splitAddr(s)
		switch {
		case !smtpLocal.MatchString(local):
			return "bad Local-part"
		case len(local) > 64 || len(s) > 254:
			return "too long"
		case strings.HasPrefix(domain, "["):
			lit := strings.TrimSuffix(strings.TrimPrefix(domain, "["), "]")
			if v6, ok := strings.CutPrefix(strings.ToLower(lit), "ipv6:"); ok {
				return checkRFC4291(v6, 2)
			}
			if a, err := netip.ParseAddr(lit); err != nil || !a.Is4() {
				return "bad IPv4 literal"
			}
		case !smtpHost.MatchString(domain):
			return "bad Domain"
		}
		return ""
	})
	pbt.Check(t, func(pt *pbt.T) {
		s := RFC5321Mailbox().Draw(pt, "s")
		local, domain := splitAddr(s)
		pt.Cover(strings.HasPrefix(local, `"`), 10, "quoted")
		pt.Cover(strings.HasPrefix(local, `"`) && strings.Contains(local, " "), 3, "space in quotes")
		pt.Cover(strings.Contains(local, `\`), 2, "escape")
		pt.Cover(strings.HasPrefix(domain, "[IPv6:") || strings.HasPrefix(domain, "[ipv6:"), 5, "IPv6 literal")
		pt.Cover(strings.HasPrefix(domain, "[") && !strings.Contains(domain, ":"), 5, "IPv4 literal")
	}, pbt.Seed(1), pbt.Checks(2000), pbt.NoSave(), pbt.CheckCoverage())
	shrinkTargets(t, RFC5321Mailbox(), "a@[0.0.0.0]")
}

func TestRFC5322AddrSpec(t *testing.T) {
	checkValid(t, RFC5322AddrSpec(), func(s string) string {
		local, domain := splitAddr(s)
		switch {
		case !rfc5322L.MatchString(local):
			return "bad local-part"
		case !rfc5322D.MatchString(domain):
			return "bad domain"
		}
		return ""
	})
	pbt.Check(t, func(pt *pbt.T) {
		s := RFC5322AddrSpec().Draw(pt, "s")
		local, domain := splitAddr(s)
		pt.Cover(strings.HasPrefix(local, `"`), 10, "quoted")
		pt.Cover(strings.Contains(local, "\t"), 1, "tab")
		pt.Cover(strings.HasPrefix(domain, "["), 10, "domain literal")
		pt.Cover(!strings.HasPrefix(domain, "[") && strings.ContainsAny(domain, "!#$%&'*+/=?^_`{|}~"), 5, "atext beyond host names")
	}, pbt.Seed(1), pbt.Checks(2000), pbt.NoSave(), pbt.CheckCoverage())
	shrinkTargets(t, RFC5322AddrSpec(), "a@[]")
}

// TestGoMailAddressDivergences records which addr-spec forms
// net/mail.ParseAddress rejects, for the godoc of RFC5322AddrSpec.
func TestGoMailAddressDivergences(t *testing.T) {
	rejected := map[string]int{}
	pbt.Check(t, func(pt *pbt.T) {
		s := RFC5322AddrSpec().Draw(pt, "s")
		if _, err := mail.ParseAddress(s); err != nil {
			local, domain := splitAddr(s)
			switch {
			case strings.HasPrefix(domain, "["):
				rejected["domain literal"]++
			case strings.HasPrefix(local, `"`):
				rejected["quoted local part"]++
			default:
				rejected["dot-atom"]++
			}
		}
	}, pbt.Seed(1), pbt.Checks(2000), pbt.NoSave())
	t.Logf("net/mail.ParseAddress rejected: %v", rejected)
}

// shrinkTargets checks where each form of local part and domain shrinks
// to, as the godoc of RFC5321Mailbox and RFC5322AddrSpec states.
func shrinkTargets(t *testing.T, g pbt.Gen[string], literal string) {
	t.Helper()
	cases := []struct {
		name  string
		fails func(local, domain string) bool
		want  string
	}{
		{"dot-atom and domain", func(l, d string) bool { return !strings.HasPrefix(l, `"`) && !strings.HasPrefix(d, "[") }, "a@a"},
		{"quoted", func(l, d string) bool { return strings.HasPrefix(l, `"`) && !strings.HasPrefix(d, "[") }, `""@a`},
		{"literal", func(l, d string) bool { return !strings.HasPrefix(l, `"`) && strings.HasPrefix(d, "[") }, literal},
	}
	for _, c := range cases {
		got := minimalOf(t, g, func(s string) bool { return c.fails(splitAddr(s)) })
		if got != c.want {
			t.Errorf("%s: %s shrinks to %q, want %q", g, c.name, got, c.want)
		}
	}
}
