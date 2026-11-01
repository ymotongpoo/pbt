// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

// Package mutant keeps the planted bugs out of an ordinary test run.
//
// A test named TestXxxMutant runs a property against a planted bug and is
// supposed to fail. Such a test calls Only first, which skips it unless the
// environment variable PBT_MUTANTS is set. run_mutants.sh sets it and checks
// that every mutant test fails.
package mutant

import (
	"os"
	"testing"
)

// Only skips t unless PBT_MUTANTS is set.
func Only(t *testing.T) {
	t.Helper()
	if os.Getenv("PBT_MUTANTS") == "" {
		t.Skip("a planted bug that must fail; set PBT_MUTANTS=1 or run run_mutants.sh")
	}
}
