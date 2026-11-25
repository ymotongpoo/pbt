// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package snippets

import (
	"strings"
	"testing"

	"go.ymotongpoo.com/pbt"
)

// snippet: duplicate
func checkSwitch(tb pbt.TB) {
	pbt.Check(tb, func(t *pbt.T) {
		enabled := pbt.Bool().Draw(t, "enabled")
		t.Classify(enabled, "enabled")
	}, pbt.Seed(1), pbt.Checks(20), pbt.WarnHealth(pbt.HealthDuplicate))
}

// snippet: end

func TestDuplicateWarning(t *testing.T) {
	rec := &recorder{name: "TestSwitch"}
	checkSwitch(rec)
	if len(rec.errs) != 0 {
		t.Fatal(rec.errs)
	}
	logs := strings.Join(rec.logs, "\n")
	t.Logf("logs:\n%s", logs)
	mustContain(t, logs, "health: HealthDuplicate:", "2 of 20")
}

// snippet: signature
// parsePort is the parser with two bugs from chapter 4.
var namedPortProperty = func(t *pbt.T) {
	s := pbt.StringOf(pbt.SampledFrom([]rune("0123456789"))).Draw(t, "s")
	_, accepted := parsePort(s)
	want := validPort(s)
	if accepted != want {
		cause := "rejected valid port"
		if accepted {
			cause = "accepted invalid port"
		}
		t.Signature(cause)
		t.Fatalf("parsePort(%q): %s", s, cause)
	}
}

// snippet: end

func TestSignatureSeparatesCauses(t *testing.T) {
	rec := &recorder{name: "TestNamedPort"}
	pbt.Check(rec, namedPortProperty, pbt.Seed(1), pbt.NoSave(), pbt.MaxFailures(2))
	if len(rec.errs) != 2 {
		t.Fatalf("want two causes, got %q", rec.errs)
	}
	reports := strings.Join(rec.errs, "\n")
	t.Logf("reports:\n%s", reports)
	mustContain(t, reports,
		"failure 1 of 2: property failed after 6 passed checks and 9 shrink steps (size 4)",
		"failure 2 of 2: property failed after 6 passed checks and 90 shrink steps (size 8)",
		`failure was named "accepted invalid port"`, `failure was named "rejected valid port"`,
		`parsePort("0"): accepted invalid port`, `parsePort("7000"): rejected valid port`)
}
