// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package format

import (
	"go.ymotongpoo.com/pbt"
	"mime"
	"strings"
	"testing"
)

func isHTTPTokenByte(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.ContainsRune("!#$%&'*+-.^_`|~", rune(c))
}
func takeHTTPToken(s string) string {
	i := 0
	for i < len(s) && isHTTPTokenByte(s[i]) {
		i++
	}
	return s[:i]
}
func checkMedia(s string) string {
	token := takeHTTPToken(s)
	if token == "" {
		return "type"
	}
	s = s[len(token):]
	if !strings.HasPrefix(s, "/") {
		return "slash"
	}
	s = s[1:]
	token = takeHTTPToken(s)
	if token == "" {
		return "subtype"
	}
	s = s[len(token):]
	for len(s) > 0 {
		s = strings.TrimLeft(s, " \t")
		if !strings.HasPrefix(s, ";") {
			return "semicolon"
		}
		s = strings.TrimLeft(s[1:], " \t")
		if s == "" || s[0] == ';' {
			continue
		}
		token = takeHTTPToken(s)
		if token == "" {
			return "parameter name"
		}
		s = s[len(token):]
		if !strings.HasPrefix(s, "=") {
			return "equals"
		}
		s = s[1:]
		if strings.HasPrefix(s, "\"") {
			s = s[1:]
			closed := false
			for len(s) > 0 {
				c := s[0]
				s = s[1:]
				if c == '"' {
					closed = true
					break
				}
				if c == '\\' {
					if len(s) == 0 {
						return "escape"
					}
					c = s[0]
					s = s[1:]
					if !(c == 9 || c >= 32 && c != 127) {
						return "quoted-pair"
					}
				} else if !(c == 9 || c == 32 || c == 33 || c >= 35 && c <= 91 || c >= 93 && c != 127) {
					return "qdtext"
				}
			}
			if !closed {
				return "unclosed quote"
			}
		} else {
			token = takeHTTPToken(s)
			if token == "" {
				return "value"
			}
			s = s[len(token):]
		}
	}
	return ""
}
func TestMediaTypes(t *testing.T) {
	for _, g := range []pbt.Gen[string]{RFC9110MediaType(), MediaType()} {
		t.Run(g.String(), func(t *testing.T) {
			checkMinimumSeeds(t, g, "a/a")
			checkValid(t, g, func(s string) string {
				if e := checkMedia(s); e != "" {
					return e
				}
				if g.String() == "MediaType()" {
					if _, _, e := mime.ParseMediaType(s); e != nil {
						return e.Error()
					}
				}
				return ""
			})
			pbt.Check(t, func(pt *pbt.T) {
				s := g.Draw(pt, "")
				pt.Cover(strings.Contains(s, ";"), 50, "parameters")
				pt.Cover(strings.Contains(s, "\""), 20, "quoted value")
				pt.Cover(strings.Contains(s, "\\"), 5, "quoted-pair")
				if g.String() == "RFC9110MediaType()" {
					obs := false
					for i := range s {
						obs = obs || s[i] >= 128
					}
					pt.Cover(obs, 10, "obs-text")
					pt.Cover(strings.Contains(s, "\t"), 10, "HTAB")
				}
			}, pbt.Seed(1), pbt.Checks(2000), pbt.NoSave(), pbt.CheckCoverage())
			if s := minimalOf(t, g, always); s != "a/a" {
				t.Errorf("minimum %q", s)
			}
		})
	}
}
func TestMediaParserDifferences(t *testing.T) {
	for _, s := range []string{"a/a;;b=c", "a/a;a=b;a=c"} {
		if checkMedia(s) != "" {
			t.Fatal("bad witness")
		}
		if _, _, e := mime.ParseMediaType(s); e == nil {
			t.Errorf("accepted %q", s)
		}
	}
	for _, s := range []string{"a/a;", "a/a;a=\"\xff\"", "a/a;a=\"\\\xff\""} {
		if _, _, e := mime.ParseMediaType(s); e != nil {
			t.Errorf("rejected %q: %v", s, e)
		}
	}
	_, escaped, err := mime.ParseMediaType("a/a;a=\"\\\xff\"")
	if err != nil || escaped["a"] != "\\\xff" {
		t.Errorf("obs quoted-pair: %q %v", escaped, err)
	}
	_, params, e := mime.ParseMediaType("a/a;filename*=utf-8''hello%20world")
	if e != nil || params["filename"] != "hello world" {
		t.Errorf("RFC2231: %v %v", params, e)
	}
}
