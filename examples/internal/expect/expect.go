// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

// Package expect runs a property that is supposed to fail and returns its
// report, so that the examples can show pbt catching a planted bug without
// failing the test suite.
package expect

import (
	"fmt"
	"testing"

	"go.ymotongpoo.com/pbt"
)

type recorder struct {
	name   string
	logs   []string
	errors []string
}

func (r *recorder) Helper()      {}
func (r *recorder) Name() string { return r.name }
func (r *recorder) Logf(format string, args ...any) {
	r.logs = append(r.logs, fmt.Sprintf(format, args...))
}
func (r *recorder) Errorf(format string, args ...any) {
	r.errors = append(r.errors, fmt.Sprintf(format, args...))
}

// Failure runs prop with pbt.Check against a recording TB and returns the
// failure report. It fails t when the property passes. Nothing is saved
// under testdata.
func Failure(t *testing.T, prop pbt.Property, opts ...pbt.Option) string {
	t.Helper()
	rec := &recorder{name: t.Name()}
	pbt.Check(rec, prop, append([]pbt.Option{pbt.NoSave()}, opts...)...)
	if len(rec.errors) == 0 {
		t.Fatalf("the property was expected to fail but passed; logs: %q", rec.logs)
	}
	report := rec.errors[0]
	t.Logf("pbt caught the planted bug:\n%s", report)
	return report
}
