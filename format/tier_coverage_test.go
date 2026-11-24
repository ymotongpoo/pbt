// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package format

import (
	"regexp"
	"strings"
	"testing"

	"go.ymotongpoo.com/pbt"
)

func TestCommonMediaRestrictedNames(t *testing.T) {
	// RFC 6838 §4.2, independent of the generator's character sets.
	name := `[A-Za-z0-9][A-Za-z0-9!#$&^_.+\-]{0,126}`
	re := regexp.MustCompile(`^` + name + `/` + name + `$`)
	checkValid(t, MediaType(), func(s string) string {
		typ, _, _ := strings.Cut(s, ";")
		if !re.MatchString(typ) {
			return "restricted-name"
		}
		return ""
	})
}

func TestGrandfatheredCoverage(t *testing.T) {
	pbt.Check(t, func(pt *pbt.T) {
		s := RFC5646LanguageTag().Draw(pt, "")
		for _, tag := range strings.Fields(grandfatheredCheck) {
			pt.Cover(strings.EqualFold(s, tag), 0.2, tag)
		}
		pt.Cover(s != strings.ToLower(s) && s != strings.ToUpper(s), 30, "mixed case")
		pt.Cover(s == strings.ToUpper(s), 1, "upper case")
	}, pbt.Seed(1), pbt.Checks(4000), pbt.NoSave(), pbt.CheckCoverage())
}

func TestTierShrinkForms(t *testing.T) {
	for _, tc := range []struct {
		g     pbt.Gen[string]
		fails func(string) bool
		want  string
	}{
		{csvFieldOf(true), always[string], `""`},
		{SemVer(), func(s string) bool { return strings.Contains(s, "-") }, "0.0.0-0"},
		{SemVer(), func(s string) bool { return strings.Contains(s, "+") }, "0.0.0+a"},
		{RFC5646LanguageTag(), func(s string) bool { return strings.HasPrefix(strings.ToLower(s), "x-") }, "x-a"},
		{LanguageTag(), isGrandfathered, "i-ami"},
		{RFC5646LanguageTag(), isGrandfathered, "i-ami"},
		{RFC6265Cookie(), func(s string) bool { return strings.Contains(s, "\"") }, `a=""`},
		{JSONPointer(), func(s string) bool { return s != "" }, "/"},
		{JSONPointerFragment(), func(s string) bool { return s != "#" }, "#/"},
	} {
		for seed := uint64(1); seed <= 10; seed++ {
			got := ""
			found := false
			pbt.Check(quietTB{t.Name()}, func(pt *pbt.T) {
				s := tc.g.Draw(pt, "")
				pt.OnFail(func() { got = s; found = true })
				if tc.fails(s) {
					pt.Fail()
				}
			}, pbt.Seed(seed), pbt.Checks(2000), pbt.NoSave())
			if !found || got != tc.want {
				t.Errorf("%s seed %d: %q, want %q", tc.g, seed, got, tc.want)
			}
		}
	}
}
