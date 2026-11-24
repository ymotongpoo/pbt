// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package format

import (
	"go.ymotongpoo.com/pbt"
	"regexp"
	"strings"
	"testing"
)

// The numbered-capture expression published in the SemVer 2.0.0 FAQ.
var semverRE = regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-((?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:\+([0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$`)

func TestSemVer(t *testing.T) {
	g := SemVer()
	checkMinimumSeeds(t, g, "0.0.0")
	checkValid(t, g, func(s string) string {
		if !semverRE.MatchString(s) {
			return "not SemVer"
		}
		return ""
	})
	pbt.Check(t, func(pt *pbt.T) {
		s := g.Draw(pt, "")
		m := semverRE.FindStringSubmatch(s)
		pt.Cover(m[4] != "", 30, "pre-release")
		pt.Cover(m[5] != "", 30, "build")
		pt.Cover(m[4] == "" && m[5] == "", 10, "release")
		pt.Cover(strings.Contains(m[4], "."), 10, "multiple pre-release identifiers")
		pt.Cover(regexp.MustCompile(`(^|\.)[0-9]+($|\.)`).MatchString(m[4]), 5, "numeric pre-release")
		pt.Cover(strings.ContainsAny(m[4], "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ-"), 10, "alphanumeric pre-release")
	}, pbt.Seed(1), pbt.Checks(2000), pbt.NoSave(), pbt.CheckCoverage())
	if s := minimalOf(t, g, always); s != "0.0.0" {
		t.Errorf("minimum %q", s)
	}
}
