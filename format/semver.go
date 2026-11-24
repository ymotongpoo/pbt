// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package format

import (
	"go.ymotongpoo.com/pbt"
	"strings"
)

// SemVer generates <valid semver> of Semantic Versioning 2.0.0
// (https://semver.org/spec/v2.0.0.html), without a leading v. Core numbers
// and numeric pre-release identifiers have no leading zeros. Pre-release
// and build lists are independently present in half the cases, with one
// to four identifiers. Each number or identifier has at most 16 bytes;
// the specification has no length limit. Pre-release identifiers are
// equally likely to be numeric or contain a letter or hyphen. Build
// identifiers may contain leading zeros. Values shrink toward "0.0.0".
func SemVer() pbt.Gen[string] {
	return pbt.Custom("SemVer()", func(t *pbt.T) string {
		pre, build := pbt.Bool().Draw(t, ""), pbt.Bool().Draw(t, "")
		s := semverNumber.Draw(t, "") + "." + semverNumber.Draw(t, "") + "." + semverNumber.Draw(t, "")
		if pre {
			s += "-" + strings.Join(pbt.SliceOfN(semverPre, 1, 4).Draw(t, ""), ".")
		}
		if build {
			s += "+" + strings.Join(pbt.SliceOfN(stringIn(alnum+"-", 1, 16), 1, 4).Draw(t, ""), ".")
		}
		return s
	})
}

var semverNumber = pbt.Custom("numeric identifier", func(t *pbt.T) string {
	n := pbt.IntRange(1, 16).Draw(t, "")
	b := make([]byte, n)
	for i := range b {
		set := digits
		if i == 0 && n > 1 {
			set = "123456789"
		}
		b[i] = charIn(set).Draw(t, "")
	}
	return string(b)
})
var semverPre = pbt.Custom("pre-release identifier", func(t *pbt.T) string {
	if pbt.Bool().Draw(t, "") {
		return semverNumber.Draw(t, "")
	}
	return semverAlpha.Draw(t, "")
})
var semverAlpha = pbt.Custom("alphanumeric identifier", func(t *pbt.T) string {
	n := pbt.IntRange(1, 16).Draw(t, "")
	at := pbt.IntRange(0, n-1).Draw(t, "")
	b := make([]byte, n)
	for i := range b {
		set := alnum + "-"
		if i == at {
			set = alpha + "-"
		}
		b[i] = charIn(set).Draw(t, "")
	}
	return string(b)
})
