// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package format

import (
	"testing"
	"time"

	"go.ymotongpoo.com/pbt"
)

// TestNames checks that every generator of the package names itself after
// its constructor, which is how reports show it.
func TestNames(t *testing.T) {
	gens := map[string]pbt.Gen[string]{
		"Base16()":                          Base16(),
		"Base32()":                          Base32(),
		"Base32Hex()":                       Base32Hex(),
		"Base64()":                          Base64(),
		"Base64URL()":                       Base64URL(),
		"CIDR()":                            CIDR(),
		"Email()":                           Email(),
		"Hex()":                             Hex(),
		"Hostname()":                        Hostname(),
		"HTMLEmail()":                       HTMLEmail(),
		"HTTPDate()":                        HTTPDate(),
		"HTTPFieldName()":                   HTTPFieldName(),
		"HTTPFieldValue()":                  HTTPFieldValue(),
		"HTTPToken()":                       HTTPToken(),
		"IDNAHostname()":                    IDNAHostname(),
		"IJSON()":                           IJSON(),
		"IPv4()":                            IPv4(),
		"ISBN10()":                          ISBN10(),
		"ISBN13()":                          ISBN13(),
		"ISO8601Duration()":                 ISO8601Duration(),
		"JSON()":                            JSON(),
		"JSONPointer()":                     JSONPointer(),
		"JSONPointerFragment()":             JSONPointerFragment(),
		"LanguageTag()":                     LanguageTag(),
		"MAC()":                             MAC(),
		"MediaType()":                       MediaType(),
		"PaymentCard()":                     PaymentCard(),
		"PhoneNumber()":                     PhoneNumber(),
		"RawBase32()":                       RawBase32(),
		"RawBase32Hex()":                    RawBase32Hex(),
		"RawBase64()":                       RawBase64(),
		"RawBase64URL()":                    RawBase64URL(),
		"RFC1035Domain()":                   RFC1035Domain(),
		"RFC3339()":                         RFC3339(),
		"RFC3339Duration()":                 RFC3339Duration(),
		"RFC3339FullDate()":                 RFC3339FullDate(),
		"RFC3339FullTime()":                 RFC3339FullTime(),
		"RFC3986Reference()":                RFC3986Reference(),
		"RFC3986URI()":                      RFC3986URI(),
		"RFC4180CSV()":                      RFC4180CSV(),
		"RFC4291IPv6()":                     RFC4291IPv6(),
		"RFC5321Mailbox()":                  RFC5321Mailbox(),
		"RFC5322AddrSpec()":                 RFC5322AddrSpec(),
		"RFC5322DateTime()":                 RFC5322DateTime(),
		"RFC5646LanguageTag()":              RFC5646LanguageTag(),
		"RFC5952IPv6()":                     RFC5952IPv6(),
		"RFC6265Cookie()":                   RFC6265Cookie(),
		"RFC6265SetCookie()":                RFC6265SetCookie(),
		"RFC9110FieldValue()":               RFC9110FieldValue(),
		"RFC9110HTTPDate()":                 RFC9110HTTPDate(),
		"RFC9110MediaType()":                RFC9110MediaType(),
		"SemVer()":                          SemVer(),
		"SMTPUTF8Mailbox()":                 SMTPUTF8Mailbox(),
		"UnicodeHostname()":                 UnicodeHostname(),
		"URL()":                             URL(),
		`Time("2006-01-02T15:04:05Z07:00")`: Time(time.RFC3339),
		"UUID()":                            UUID(),
		"UUID(4, 7)":                        UUID(4, 7),
	}
	for want, g := range gens {
		if got := g.String(); got != want {
			t.Errorf("%s is named %q", want, got)
		}
	}
}
