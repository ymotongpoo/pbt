// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package format

import "go.ymotongpoo.com/pbt"

// PhoneNumber generates the common compact international notation: "+"
// followed by one to fifteen digits, the first from 1 to 9. The length
// bound is from ITU-T E.164 §6.1 and the plus sign from E.123 §7.1.
// Country codes are not checked against the ITU list; national numbering
// plans, assigned subscribers, extensions and display grouping are not
// modeled. This is a syntax generator, not a generator of assigned numbers.
// Values shrink toward "+1".
func PhoneNumber() pbt.Gen[string] {
	return named(concat(pbt.Just("+"), char("123456789"), stringIn(digits, 0, 14)), "PhoneNumber()")
}

// PaymentCard generates eight to nineteen decimal digits with a Luhn
// check digit, following the PAN layout of ISO/IEC 7812-1:2006 §4 and
// Annex B. The first digit is the major industry identifier, from 0 to 9;
// neither it nor the issuer identification number is checked for assignment.
// The eight-digit minimum includes legacy six-digit IINs; the 2017 edition
// requires at least ten digits for eight-digit IINs. These are checksum-valid
// strings, not necessarily issued payment cards. Values shrink toward
// "00000000". There are no separators.
func PaymentCard() pbt.Gen[string] {
	length := pbt.IntRange(7, 18)
	return pbt.Custom("PaymentCard()", func(t *pbt.T) string {
		body := drawChars(t, digits, length.Draw(t, ""))
		return body + string(luhnDigit(body))
	})
}

func luhnDigit(body string) byte {
	sum := 0
	for i := len(body) - 1; i >= 0; i-- {
		d := int(body[i] - '0')
		if (len(body)-1-i)%2 == 0 {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
	}
	return byte((10-sum%10)%10) + '0'
}

// ISBN13 generates unhyphenated ISBN-13 strings with the check digit of
// ISO 2108:2017 §4.6 and Annex C (International ISBN Agency Users' Manual, Appendix 1).
// The prefix is 978 or 979 with equal weight, followed by nine digits and
// the modulo-10 check digit. Registration groups, registrants and titles
// are not checked for assignment. Hyphenation requires registry ranges
// and is not generated. Values shrink toward "9780000000002"; when the
// 979 prefix is required, the target is "9790000000001".
func ISBN13() pbt.Gen[string] {
	body := concat(pbt.OneOf(pbt.Just("978"), pbt.Just("979")), fixedString(digits, 9))
	return named(pbt.Map(body, func(s string) string {
		sum := 0
		for i := range len(s) {
			sum += int(s[i]-'0') * (1 + 2*(i%2))
		}
		return s + string(byte((10-sum%10)%10)+'0')
	}), "ISBN13()")
}

// ISBN10 generates unhyphenated legacy ISBN-10 strings (ISO 2108:1992
// §3.4): nine digits and a modulo-11 check character, with uppercase X for
// ten. X occurs in roughly one case in eleven. Assignment and hyphenation
// are not modeled, for the same reasons as ISBN13. Values shrink toward
// "0000000000".
func ISBN10() pbt.Gen[string] {
	return named(pbt.Map(fixedString(digits, 9), func(s string) string {
		sum := 0
		for i := range len(s) {
			sum += int(s[i]-'0') * (10 - i)
		}
		return s + string("0123456789X"[(11-sum%11)%11])
	}), "ISBN10()")
}
