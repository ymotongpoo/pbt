// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package snippets

import (
	"strings"
	"testing"

	"go.ymotongpoo.com/pbt"
)

// snippet: health
// Most strings that String draws at small sizes are shorter than eight
// runes, so Assume throws most cases away.
var longNameProperty = func(t *pbt.T) {
	name := pbt.String().Draw(t, "name")
	t.Assume(len([]rune(name)) >= 8)
	if short := []rune(name)[:8]; len(short) != 8 {
		t.Fatalf("prefix %q", string(short))
	}
}

// snippet: end

func TestHealthWarning(t *testing.T) {
	logs := passingLogs(t, "TestLongNames", longNameProperty, pbt.Seed(1))
	mustContain(t, logs, "[pbt] health: HealthFilter:", "most by Assume at health_test.go:")
	rec := &recorder{name: "TestLongNames"}
	pbt.Check(rec, longNameProperty, pbt.Seed(1), pbt.NoSave(), pbt.CheckHealth())
	if len(rec.errs) != 1 || !strings.Contains(rec.errs[0], "HealthFilter") {
		t.Fatalf("CheckHealth did not fail the run: %q", rec.errs)
	}
}
