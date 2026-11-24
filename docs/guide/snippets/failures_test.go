// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package snippets

import (
	"strconv"
	"strings"
	"testing"

	"go.ymotongpoo.com/pbt"
)

// parsePort has two bugs: it accepts port 0, and its upper bound lost a
// digit, so it rejects the ports from 6554 to 65535.
func parsePort(s string) (int, bool) {
	if s == "" {
		return 0, false
	}
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' || n > 65535 {
			return 0, false
		}
		n = n*10 + int(c-'0')
	}
	return n, n <= 6553
}

func validPort(s string) bool {
	n, err := strconv.Atoi(s)
	return err == nil && n >= 1 && n <= 65535
}

// snippet: failures
var portProperty = func(t *pbt.T) {
	s := pbt.StringOf(pbt.SampledFrom([]rune("0123456789"))).Draw(t, "s")
	n, ok := parsePort(s)
	if ok && !validPort(s) {
		t.Fatalf("parsePort(%q) accepted %d", s, n)
	}
	if !ok && validPort(s) {
		t.Fatalf("parsePort(%q) rejected a valid port", s)
	}
}

// snippet: end

func TestTwoBugsAreCaught(t *testing.T) {
	rec := &recorder{name: "TestPort"}
	pbt.Check(rec, portProperty, pbt.Seed(1), pbt.NoSave(), pbt.MaxFailures(2))
	if len(rec.errs) != 2 {
		t.Fatalf("got %d reports: %q", len(rec.errs), rec.errs)
	}
	t.Logf("reports:\n%s", strings.Join(rec.errs, "\n"))
	mustContain(t, rec.errs[0]+rec.errs[1], "failure 1 of 2", "failure 2 of 2", `parsePort("0") accepted 0`, `parsePort("7000") rejected a valid port`)
}
