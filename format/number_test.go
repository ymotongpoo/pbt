// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package format

import (
	"regexp"
	"strings"
	"testing"

	"go.ymotongpoo.com/pbt"
)

// luhnValid uses a digit lookup and left-to-right parity, independently of
// the encoder's right-to-left doubling.
func luhnValid(s string) bool {
	double := [...]int{0, 2, 4, 6, 8, 1, 3, 5, 7, 9}
	sum := 0
	for i, c := range s {
		if c < '0' || c > '9' {
			return false
		}
		d := int(c - '0')
		if i%2 == len(s)%2 {
			d = double[d]
		}
		sum += d
	}
	return sum%10 == 0
}

func isbnValid(s string) bool {
	sum := 0
	for i, c := range s {
		d := int(c - '0')
		if len(s) == 10 && i == 9 && c == 'X' {
			d = 10
		} else if d < 0 || d > 9 {
			return false
		}
		if len(s) == 10 {
			sum += (i + 1) * d
		} else {
			sum += (1 + 2*(i%2)) * d
		}
	}
	if len(s) == 10 {
		return sum%11 == 0
	}
	return len(s) == 13 && (strings.HasPrefix(s, "978") || strings.HasPrefix(s, "979")) && sum%10 == 0
}

func TestNumberFormats(t *testing.T) {
	phone := regexp.MustCompile(`^\+[1-9][0-9]{0,14}$`)
	for _, tc := range []struct {
		g      pbt.Gen[string]
		valid  func(string) bool
		target string
	}{
		{PhoneNumber(), phone.MatchString, "+1"},
		{PaymentCard(), func(s string) bool { return len(s) >= 8 && len(s) <= 19 && luhnValid(s) }, "00000000"},
		{ISBN13(), isbnValid, "9780000000002"},
		{ISBN10(), isbnValid, "0000000000"},
	} {
		t.Run(tc.g.String(), func(t *testing.T) {
			checkValid(t, tc.g, func(s string) string {
				if !tc.valid(s) {
					return "invalid number: " + s
				}
				return ""
			})
			shrinksTo(t, tc.g, always[string], tc.target)
		})
	}
	shrinksTo(t, ISBN13(), func(s string) bool { return strings.HasPrefix(s, "979") }, "9790000000001")
}

func TestNumberPublishedExamples(t *testing.T) {
	// Stripe's public testing cards, https://docs.stripe.com/testing#cards
	for _, s := range []string{"4242424242424242", "4000002500003155", "5555555555554444", "378282246310005"} {
		if !luhnValid(s) || luhnDigit(s[:len(s)-1]) != s[len(s)-1] {
			t.Errorf("Luhn: %s", s)
		}
	}
	// International ISBN Agency Users' Manual: 978-0-11-000222-4;
	// ISBN-10 example in the Library of Congress MARC 21 020 documentation.
	for _, s := range []string{"9780110002224", "0491001304", "9780060723804"} {
		if !isbnValid(s) {
			t.Errorf("ISBN: %s", s)
		}
	}
}

func TestNumberCoverage(t *testing.T) {
	phone, card, i13, i10 := PhoneNumber(), PaymentCard(), ISBN13(), ISBN10()
	pbt.Check(t, func(pt *pbt.T) {
		p, c, a, b := phone.Draw(pt, ""), card.Draw(pt, ""), i13.Draw(pt, ""), i10.Draw(pt, "")
		pt.Cover(len(p) == 2, 1, "one digit phone")
		pt.Cover(len(p) == 16, 1, "15 digit phone")
		pt.Cover(len(c) == 8, 1, "8 digit PAN")
		pt.Cover(len(c) == 19, 1, "19 digit PAN")
		for d := '0'; d <= '9'; d++ {
			pt.Cover(rune(c[0]) == d, 3, "MII "+string(d))
		}
		pt.Cover(a[:3] == "978", 35, "978")
		pt.Cover(a[:3] == "979", 35, "979")
		pt.Cover(b[9] == 'X', 4, "ISBN X")
		pt.Classify(b[9] == 'X', "ISBN X")
		pt.Classify(a[:3] == "979", "979 prefix")
	}, pbt.Seed(1), pbt.Checks(4000), pbt.CheckCoverage(), pbt.NoSave())
}
