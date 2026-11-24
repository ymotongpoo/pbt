// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package format

import (
	"go.ymotongpoo.com/pbt"
	"regexp"
	"strings"
	"testing"
)

var languageRE = regexp.MustCompile(`(?i)^(?:([a-z]{2,3}(?:-[a-z]{3}){0,3}|[a-z]{4}|[a-z]{5,8})(?:-([a-z]{4}))?(?:-([a-z]{2}|[0-9]{3}))?((?:-(?:[a-z0-9]{5,8}|[0-9][a-z0-9]{3}))*)((?:-[0-9a-wy-z](?:-[a-z0-9]{2,8})+)*)(?:-x(?:-[a-z0-9]{1,8})+)?|x(?:-[a-z0-9]{1,8})+)$`)

const grandfatheredCheck = "en-gb-oed i-ami i-bnn i-default i-enochian i-hak i-klingon i-lux i-mingo i-navajo i-pwn i-tao i-tay i-tsu sgn-be-fr sgn-be-nl sgn-ch-de art-lojban cel-gaulish no-bok no-nyn zh-guoyu zh-hakka zh-min zh-min-nan zh-xiang"

func isGrandfathered(s string) bool {
	return strings.Contains(" "+grandfatheredCheck+" ", " "+strings.ToLower(s)+" ")
}
func checkLanguage(s string) string {
	if isGrandfathered(s) {
		return ""
	}
	m := languageRE.FindStringSubmatch(s)
	if m == nil {
		return "not Language-Tag"
	}
	seen := map[string]bool{}
	for _, v := range strings.Split(strings.TrimPrefix(strings.ToLower(m[4]), "-"), "-") {
		if v == "" {
			continue
		}
		if seen[v] {
			return "repeated variant"
		}
		seen[v] = true
	}
	seen = map[string]bool{}
	for _, v := range strings.Split(strings.ToLower(m[5]), "-") {
		if len(v) != 1 {
			continue
		}
		if seen[v] {
			return "repeated singleton"
		}
		seen[v] = true
	}
	return ""
}
func TestLanguageTags(t *testing.T) {
	for _, g := range []pbt.Gen[string]{RFC5646LanguageTag(), LanguageTag()} {
		t.Run(g.String(), func(t *testing.T) {
			checkMinimumSeeds(t, g, "aa")
			checkMinimumSeeds(t, casedTag(langtag, g.String() == "LanguageTag()"), "aa")
			checkMinimumSeeds(t, casedTag(pbt.SampledFrom(grandfatheredTags), g.String() == "LanguageTag()"), "i-ami")
			checkValid(t, g, checkLanguage)
			wantGrand := "i-ami"
			if g.String() == "LanguageTag()" {
				wantGrand = "i-ami"
			}
			if s := minimalOf(t, g, isGrandfathered); s != wantGrand {
				t.Errorf("grandfathered minimum %q", s)
			}
			pbt.Check(t, func(pt *pbt.T) {
				s := g.Draw(pt, "")
				lo := strings.ToLower(s)
				m := languageRE.FindStringSubmatch(s)
				pt.Cover(isGrandfathered(s), 15, "grandfathered")
				pt.Cover(strings.HasPrefix(lo, "x-"), 15, "private only")
				if m == nil {
					m = make([]string, 6)
				}
				pt.Cover(m[2] != "", 5, "script")
				pt.Cover(m[3] != "", 5, "region")
				pt.Cover(m[4] != "", 10, "variants")
				pt.Cover(m[5] != "", 10, "extensions")
				pt.Cover(strings.Contains(m[1], "-"), 1, "extlang")
				pt.Cover(strings.Contains(lo, "-x-"), 5, "private suffix")
			}, pbt.Seed(1), pbt.Checks(2000), pbt.NoSave(), pbt.CheckCoverage())
			if s := minimalOf(t, g, always); s != "aa" {
				t.Errorf("minimum %q", s)
			}
			if s := minimalOf(t, g, func(s string) bool { return strings.HasPrefix(strings.ToLower(s), "x-") }); s != "x-a" {
				t.Errorf("private minimum %q", s)
			}
		})
	}
	pbt.Check(t, func(pt *pbt.T) {
		s := LanguageTag().Draw(pt, "")
		m := languageRE.FindStringSubmatch(s)
		if m == nil || isGrandfathered(s) {
			return
		}
		if m[2] != "" && m[2] != strings.ToUpper(m[2][:1])+strings.ToLower(m[2][1:]) {
			pt.Fatal("script case")
		}
		if m[3] != strings.ToUpper(m[3]) {
			pt.Fatal("region case")
		}
	}, pbt.Seed(1), pbt.Checks(2000), pbt.NoSave())
}
