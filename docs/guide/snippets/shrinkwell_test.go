// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package snippets

import (
	"fmt"
	"strings"
	"testing"

	"go.ymotongpoo.com/pbt"
)

// snippet: shrink-well
// Each alternative draws its whole value, so the choice of the alternative
// and the choices of its value sit in one span, and the shrinker can switch
// from the phone number to the shorter e-mail address without disturbing
// the draws that follow.
var contact = pbt.OneOf(
	pbt.Custom("email", func(t *pbt.T) string {
		return pbt.SampledFrom([]string{"alice", "bob", "carol"}).Draw(t, "") + "@example.com"
	}),
	pbt.Custom("phone", func(t *pbt.T) string {
		return fmt.Sprintf("+81-%d-%04d", pbt.IntRange(10, 99).Draw(t, ""), pbt.IntRange(0, 9999).Draw(t, ""))
	}),
)

var contactProperty = func(t *pbt.T) {
	c := contact.Draw(t, "contact")
	retries := pbt.IntRange(0, 10).Draw(t, "retries")
	if retries > 3 && !strings.Contains(c, "+81-99") {
		t.Fatalf("gave up after %d retries for %s", retries, c)
	}
}

// snippet: end

func TestContactShrinksToTheFirstAlternative(t *testing.T) {
	for seed := uint64(1); seed <= 5; seed++ {
		report := failureReport(t, "TestContact", contactProperty, pbt.Seed(seed))
		mustContain(t, report, `contact (OneOf(email, phone)) = "alice@example.com"`, "retries (IntRange(0, 10)) = 4")
	}
}
