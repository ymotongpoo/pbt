// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package format

import (
	"net/mail"
	"strings"
	"testing"
	"unicode/utf8"

	"go.ymotongpoo.com/pbt"
)

func checkSMTPUTF8(s string) string {
	if !utf8.ValidString(s) || len(s) > 254 {
		return "UTF-8 or mailbox octet limit"
	}
	local, domain := splitAddr(s)
	if len(local) == 0 || len(local) > 64 {
		return "local length"
	}
	if local[0] == '"' {
		if len(local) < 2 || local[len(local)-1] != '"' {
			return "quotes"
		}
		body := []rune(local[1 : len(local)-1])
		for i := 0; i < len(body); i++ {
			r := body[i]
			if r == '\\' {
				i++
				if i >= len(body) || body[i] < 32 || body[i] > 126 {
					return "quoted pair"
				}
			} else if r < 128 && !(r == 32 || r == 33 || r >= 35 && r <= 91 || r >= 93 && r <= 126) {
				return "qtext"
			}
		}
	} else {
		for _, atom := range strings.Split(local, ".") {
			if atom == "" {
				return "empty atom"
			}
			for _, r := range atom {
				if r >= 128 || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("!#$%&'*+-/=?^_`{|}~", r) {
					continue
				}
				return "atext"
			}
		}
	}
	if strings.IndexFunc(domain, func(r rune) bool { return r >= 128 }) >= 0 {
		return checkIDNAName(domain, false)
	}
	return checkMailbox("a@" + domain)
}

func TestSMTPUTF8Mailbox(t *testing.T) {
	g := SMTPUTF8Mailbox()
	checkValid(t, g, checkSMTPUTF8)
	var targets []string
	for _, l := range []string{"a", `""`} {
		for _, d := range []string{"a", "é", "[0.0.0.0]", "[IPv6:::]"} {
			targets = append(targets, l+"@"+d)
			quoted, domain := l == `""`, d
			// Constrain each structural form, rather than assuming all are
			// encountered by the ten unconditional failure seeds.
			shrinksTo(t, g, func(s string) bool {
				a, b := splitAddr(s)
				sameDomain := !strings.HasPrefix(b, "[") && !hasNonASCII(b)
				switch domain {
				case "é":
					sameDomain = hasNonASCII(b)
				case "[0.0.0.0]":
					sameDomain = strings.HasPrefix(b, "[") && !strings.Contains(b, ":")
				case "[IPv6:::]":
					sameDomain = strings.HasPrefix(b, "[") && strings.Contains(b, ":")
				}
				return strings.HasPrefix(a, `"`) == quoted && sameDomain
			}, l+"@"+d)
		}
	}
	shrinksTo(t, g, always[string], targets...)
}

func hasNonASCII(s string) bool {
	return strings.IndexFunc(s, func(r rune) bool { return r > 127 }) >= 0
}

func TestSMTPUTF8Coverage(t *testing.T) {
	g := SMTPUTF8Mailbox()
	pbt.Check(t, func(pt *pbt.T) {
		s := g.Draw(pt, "")
		l, d := splitAddr(s)
		quoted := strings.HasPrefix(l, `"`)
		pt.Classify(quoted, "quoted local")
		pt.Classify(hasNonASCII(d), "U-label domain")
		pt.Classify(len(s) == 254, "254 octets")
		pt.Cover(quoted, 12, "quoted")
		pt.Cover(!quoted, 65, "dot string")
		pt.Cover(hasNonASCII(d), 30, "U-label domain")
		pt.Cover(!hasNonASCII(d) && !strings.HasPrefix(d, "["), 30, "ASCII domain")
		pt.Cover(strings.HasPrefix(d, "[") && !strings.Contains(d, ":"), 4, "IPv4")
		pt.Cover(strings.HasPrefix(d, "[") && strings.Contains(d, ":"), 4, "IPv6")
		pt.Cover(quoted && hasNonASCII(l), 5, "UTF-8 qtext")
		pt.Cover(!quoted && hasNonASCII(l), 35, "UTF-8 atext")
		pt.Cover(quoted && strings.Contains(l, `\`), 3, "quoted-pair")
		pt.Cover(len(l) == 64, 5, "64 octets")
		pt.Cover(len(s) == 254, 0.05, "254 octets")
		for n := 2; n <= 4; n++ {
			found := false
			for _, r := range l {
				if utf8.RuneLen(r) == n {
					found = true
				}
			}
			pt.Cover(found, 10, string(rune('0'+n))+"-byte scalar")
		}
	}, pbt.Seed(1), pbt.Checks(4000), pbt.NoSave(), pbt.CheckCoverage())
}

func TestSMTPUTF8NetMail(t *testing.T) {
	for _, s := range []string{"é@é", `"あ b"@é`, "a@[127.0.0.1]"} {
		if _, err := mail.ParseAddress(s); err != nil {
			t.Errorf("ParseAddress(%q): %v", s, err)
		}
	}
	for _, s := range []string{`""@é`, "a@[IPv6:::]", "a@[001.2.3.4]"} {
		if _, err := mail.ParseAddress(s); err == nil {
			t.Errorf("ParseAddress(%q) unexpectedly accepted", s)
		}
	}
}

func TestSMTPUTF8OctetBoundaries(t *testing.T) {
	for _, s := range []string{strings.Repeat("é", 33), strings.Repeat("あ", 22), strings.Repeat("𐀀", 17)} {
		got := utf8Prefix(s, 64)
		if !utf8.ValidString(got) || len(got) > 64 || len(got) < 61 {
			t.Fatalf("prefix %q", got)
		}
	}
	// These are grammatically valid but too long in octets, despite short
	// rune counts. Pin the checker's ability to detect byte/rune confusion.
	for _, s := range []string{strings.Repeat("é", 33) + "@a", "a@" + strings.Repeat("é", 127)} {
		if checkSMTPUTF8(s) == "" {
			t.Fatalf("checker accepted %q", s)
		}
	}
}
