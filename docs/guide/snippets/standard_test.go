// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package snippets

import (
	"fmt"
	"net/netip"
	"regexp"
	"testing"
	"time"
	"unicode"

	"go.ymotongpoo.com/pbt"
)

// nextBillingDate moves a date one month ahead. AddDate normalizes the
// result, so January 31 becomes March 3, and the month is skipped.
func nextBillingDate(d time.Time) time.Time { return d.AddDate(0, 1, 0) }

// snippet: billing
var billingProperty = func(t *pbt.T) {
	start := pbt.Time().Draw(t, "")
	next := nextBillingDate(start)
	if want := (start.Month() % 12) + 1; next.Month() != want {
		t.Fatalf("next billing date %s is not in %s", next.Format(time.DateOnly), want)
	}
}

// snippet: end

func TestBillingIsCaught(t *testing.T) {
	report := failureReport(t, "TestBilling", billingProperty, pbt.Seed(1))
	mustContain(t, report, "start (Time()) = time.Date(1970, time.January, 31, 23, 59, 59, 999999999, time.UTC)",
		"next billing date 1970-03-03 is not in February")
}

// snippet: network
// The client of a request is an address inside the private networks of
// RFC 1918, and the rate limiter keys clients by their /24.
func TestClientKey(t *testing.T) {
	private := pbt.AddrIn(
		netip.MustParsePrefix("10.0.0.0/8"),
		netip.MustParsePrefix("172.16.0.0/12"),
		netip.MustParsePrefix("192.168.0.0/16"),
	)
	pbt.Check(t, func(t *pbt.T) {
		client := private.Draw(t, "client")
		key := netip.PrefixFrom(client, 24).Masked()
		if !key.Contains(client) || !key.Addr().IsPrivate() {
			t.Fatalf("key %s does not cover %s", key, client)
		}
	})
}

// snippet: end

// snippet: matching
// An order number has two capital letters, a dash and four digits.
var orderID = regexp.MustCompile(`^[A-Z]{2}-\d{4}$`)

func TestOrderIDs(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		id := pbt.StringMatching(`[A-Z]{2}-\d{4}`).Draw(t, "id")
		if !orderID.MatchString(id) {
			t.Fatalf("%q is not an order number", id)
		}
		name := pbt.StringOfN(pbt.RuneIn(unicode.Hiragana, unicode.Katakana, unicode.Han), 1, 8).Draw(t, "name")
		if label := fmt.Sprintf("%s %s", id, name); len([]rune(label)) > 16 {
			t.Fatalf("label %q is too long", label)
		}
	})
}

// snippet: end
