// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package snippets

import (
	"fmt"
	"strings"
	"testing"

	"go.ymotongpoo.com/pbt"
)

// recorder is a TB that keeps what Check reports, so that a listing can show
// a planted bug being caught without failing the suite.
type recorder struct {
	name string
	logs []string
	errs []string
}

func (r *recorder) Helper()      {}
func (r *recorder) Name() string { return r.name }
func (r *recorder) Logf(format string, args ...any) {
	r.logs = append(r.logs, fmt.Sprintf(format, args...))
}
func (r *recorder) Errorf(format string, args ...any) {
	r.errs = append(r.errs, fmt.Sprintf(format, args...))
}

// failureReport runs prop, which must fail, and returns the report.
func failureReport(t *testing.T, name string, prop pbt.Property, opts ...pbt.Option) string {
	t.Helper()
	rec := &recorder{name: name}
	pbt.Check(rec, prop, append([]pbt.Option{pbt.NoSave()}, opts...)...)
	if len(rec.errs) != 1 {
		t.Fatalf("expected one failure report, got %d: %q (logs %q)", len(rec.errs), rec.errs, rec.logs)
	}
	t.Logf("report:\n%s", rec.errs[0])
	return rec.errs[0]
}

// passingLogs runs prop, which must pass, and returns what Check logged.
func passingLogs(t *testing.T, name string, prop pbt.Property, opts ...pbt.Option) string {
	t.Helper()
	rec := &recorder{name: name}
	pbt.Check(rec, prop, append([]pbt.Option{pbt.NoSave()}, opts...)...)
	if len(rec.errs) != 0 {
		t.Fatalf("unexpected failure: %q", rec.errs)
	}
	logs := strings.Join(rec.logs, "\n")
	t.Logf("logs:\n%s", logs)
	return logs
}

func mustContain(t *testing.T, text string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if !strings.Contains(text, w) {
			t.Errorf("missing %q in:\n%s", w, text)
		}
	}
}
