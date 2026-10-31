// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"strings"
	"testing"
)

// TestGeneratedFixtureIsCurrent regenerates the fixture package and compares
// the result with the file checked in next to it, which the fixture's own
// tests compile and run.
func TestGeneratedFixtureIsCurrent(t *testing.T) {
	src, err := Generate("internal/fixture", []string{"Order", "Customer", "Tree", "Celsius"}, "pbt_gen.go", "Gen")
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

func TestGeneratedMutualFixtureIsCurrent(t *testing.T) {
	src, err := Generate("internal/fixture", []string{"Dept", "Employee"}, "mutual_gen.go", "Gen")
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("internal/fixture/mutual_gen.go")
	if err != nil {
		t.Fatal(err)
	}
	if string(src) != string(want) {
		t.Errorf("internal/fixture/mutual_gen.go is stale; run go generate ./internal/fixture\n--- generated ---\n%s", src)
	}
}

func TestGenerateRejectsUnknownTypes(t *testing.T) {
	if _, err := Generate("internal/fixture", []string{"Nope"}, "pbt_gen.go", "Gen"); err == nil {
		t.Error("expected an error for an unknown type")
	}
}

// The fixture declares GenTemperature by hand, so generating Temperature
// with the default prefix would redeclare it.
func TestGenerateRejectsNamesThePackageDeclares(t *testing.T) {
	_, err := Generate("internal/fixture", []string{"Temperature"}, "pbt_gen.go", "Gen")
	if err == nil || !strings.Contains(err.Error(), "already declares it at") || !strings.Contains(err.Error(), "gens.go:") {
		t.Errorf("err = %v", err)
	}
	if _, err := Generate("internal/fixture", []string{"Temperature"}, "pbt_gen.go", ""); err == nil {
		t.Error("expected an error for an empty prefix")
	}
}

func TestGeneratePrefixNamesTheFunctions(t *testing.T) {
	src, err := Generate("internal/fixture", []string{"Temperature", "Tree"}, "pbt_gen.go", "Any")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"// AnyTemperature generates Temperature values.", "func AnyTemperature() pbt.Gen[Temperature] {",
		"func AnyTree() pbt.Gen[Tree] {", "pbt.Lazy(AnyTree)"} {
		if !strings.Contains(string(src), want) {
			t.Errorf("generated code lacks %q:\n%s", want, src)
		}
	}
	if strings.Contains(string(src), "GenTree") {
		t.Errorf("generated code still uses the default prefix:\n%s", src)
	}
}

// ignored.go in the fixture redeclares Order under a build constraint that
// never holds; pbtgen must leave it out as go build does.
func TestGenerateHonorsBuildConstraints(t *testing.T) {
	data, err := os.ReadFile("internal/fixture/ignored.go")
	if err != nil || !strings.Contains(string(data), "//go:build ignore") || !strings.Contains(string(data), "type Order ") {
		t.Fatalf("the fixture lost its constrained file: %v", err)
	}
	if _, err := Generate("internal/fixture", []string{"Order"}, "pbt_gen.go", "Gen"); err != nil {
		t.Errorf("a file excluded by its build constraint broke the generation: %v", err)
	}
}
