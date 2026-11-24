// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package snippets

import (
	"encoding/json"
	"net/mail"
	"net/url"
	"testing"
	"time"

	"go.ymotongpoo.com/pbt"
	"go.ymotongpoo.com/pbt/format"
)

// snippet: parser
// Every timestamp of RFC 3339 should parse with the RFC3339 layout of
// package time.
var rfc3339Property = func(t *pbt.T) {
	s := format.RFC3339().Draw(t, "")
	if _, err := time.Parse(time.RFC3339, s); err != nil {
		t.Fatalf("time.Parse rejects %s: %v", s, err)
	}
}

// snippet: end

func TestTimeParseIsCaught(t *testing.T) {
	report := failureReport(t, "TestRFC3339", rfc3339Property, pbt.Seed(1))
	mustContain(t, report, `s (RFC3339()) = "1970-01-01T00:00:00z"`, `cannot parse "z" as "Z07:00"`)
}

// snippet: producer
// A sign-up form stores the address it receives, and the confirmation mail
// must reach the same mailbox.
func TestSignupAddresses(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		email := format.Email().Draw(t, "email")
		addr, err := mail.ParseAddress(email)
		if err != nil || addr.Address != email {
			t.Fatalf("mail.ParseAddress(%q) = %v, %v", email, addr, err)
		}
	})
}

// snippet: end

// snippet: roundtrip
func TestDecoders(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		doc := format.JSON().Draw(t, "doc")
		if !json.Valid([]byte(doc)) {
			t.Fatalf("json.Valid rejects %s", doc)
		}
		link := format.URL().Draw(t, "link")
		u, err := url.Parse(link)
		if err != nil || u.Host == "" {
			t.Fatalf("url.Parse(%q) = %v, %v", link, u, err)
		}
		at := format.Time(time.RFC1123Z).Draw(t, "at")
		if _, err := time.Parse(time.RFC1123Z, at); err != nil {
			t.Fatal(err)
		}
	})
}

// snippet: end
