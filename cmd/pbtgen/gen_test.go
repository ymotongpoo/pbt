// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"testing"
)

// TestGeneratedFixtureIsCurrent regenerates the fixture package and compares
// the result with the file checked in next to it, which the fixture's own
// tests compile and run.
func TestGeneratedFixtureIsCurrent(t *testing.T) {
	src, err := Generate("internal/fixture", []string{"Order", "Customer", "Tree", "Celsius"}, "pbt_gen.go")
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("internal/fixture/pbt_gen.go")
	if err != nil {
		t.Fatal(err)
	}
	if string(src) != string(want) {
		t.Errorf("internal/fixture/pbt_gen.go is stale; run go generate ./internal/fixture\n--- generated ---\n%s", src)
	}
}

func TestGenerateRejectsUnknownTypes(t *testing.T) {
	if _, err := Generate("internal/fixture", []string{"Nope"}, "pbt_gen.go"); err == nil {
		t.Error("expected an error for an unknown type")
	}
}
