// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"strings"
	"testing"
	"unicode"

	"go.ymotongpoo.com/pbt"
)

func Tokenize(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool {
		return unicode.IsSpace(r) || r == ',' || r == '.'
	})
}

func TestTokenizeInvariants(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		s := pbt.String().Draw(t, "s")
		for _, tok := range Tokenize(s) {
			if tok == "" {
				t.Fatalf("Tokenize(%q) produced an empty token", s)
			}
			if strings.ContainsAny(tok, " \t\n,.") {
				t.Fatalf("Tokenize(%q) produced %q, which contains a separator", s, tok)
			}
		}
	})
}

func TestTokenizeRobust(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		b := pbt.Bytes().Draw(t, "b")
		_ = Tokenize(string(b))
	})
}

func TestTokenizeNoPanic(t *testing.T) {
	pbt.Check(t, pbt.NoPanic(Tokenize))
}
