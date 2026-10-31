// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package snippets

import (
	"strings"
	"testing"
	"unicode"

	"go.ymotongpoo.com/pbt"
)

// snippet: user
type User struct {
	Name  string
	Age   int
	Email string
}

// userGen builds a User from smaller generators. Labeled draws inside a
// generator appear in the report, indented under the value they build.
var userGen = pbt.Custom("user", func(t *pbt.T) User {
	name := pbt.StringOfN(pbt.SampledFrom([]rune("abcdefghijklmnopqrstuvwxyz")), 1, 12).Draw(t, "name")
	return User{
		Name:  name,
		Age:   pbt.IntRange(0, 120).Draw(t, "age"),
		Email: name + "@example.com",
	}
})

// snippet: end

// snippet: compose
// Map transforms a value, Filter keeps the values that satisfy a predicate,
// and Bind picks the next generator from a value.
var (
	even    = pbt.Map(pbt.IntRange(0, 500), func(n int) int { return 2 * n })
	nonZero = pbt.Filter(pbt.Int(), func(n int) bool { return n != 0 })
	matrix  = pbt.Bind(pbt.IntRange(1, 4), func(n int) pbt.Gen[[][]int] {
		row := pbt.SliceOfN(pbt.Int(), n, n) // every row has the same length
		return pbt.SliceOfN(row, 1, 4)
	})
)

// snippet: end

func TestCompose(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		if e := even.Draw(t, "even"); e%2 != 0 {
			t.Fatalf("%d is odd", e)
		}
		if n := nonZero.Draw(t, "nonZero"); n == 0 {
			t.Fatal("zero")
		}
		m := matrix.Draw(t, "matrix")
		for _, row := range m {
			if len(row) != len(m[0]) {
				t.Fatalf("ragged matrix %v", m)
			}
		}
	})
}

// snippet: example
func TestExample(t *testing.T) {
	u := userGen.Example(42) // a sample outside any property; the seed fixes it
	if u.Age < 0 || !strings.HasSuffix(u.Email, "@example.com") {
		t.Fatalf("unexpected user %+v", u)
	}
}

// snippet: end

// snippet: user-report
var displayNameProperty = func(t *pbt.T) {
	u := userGen.Draw(t, "user")
	display := strings.ToUpper(u.Name[:1]) + u.Name[1:]
	if unicode.IsLower(rune(display[0])) {
		t.Fatalf("display name %q is not capitalized", display)
	}
	if len(display) > 8 {
		t.Fatalf("display name %q is too long for the badge", display)
	}
}

// snippet: end

func TestUserReport(t *testing.T) {
	report := failureReport(t, "TestDisplayName", displayNameProperty, pbt.Seed(1))
	mustContain(t, report, "user (user) = snippets.User{", "  name (StringOfN(SampledFrom(26 values), 1, 12)) = \"aaaaaaaaa\"", "  age (IntRange(0, 120)) = 0", "too long for the badge")
}
