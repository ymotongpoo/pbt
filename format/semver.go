// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package format

import (
	"go.ymotongpoo.com/pbt"
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
		return semverNumber.Draw(t, "") + "." + semverNumber.Draw(t, "") + "." + semverNumber.Draw(t, "") + semverPreRelease.Draw(t, "") + semverBuild.Draw(t, "")
	})
}

var (
	identLen = pbt.IntRange(1, 16)
	// letterAt holds, per identifier length n, the position of the
	// character that must be a letter or a hyphen.
	letterAt = func() (gs [17]pbt.Gen[int]) {
		for n := 1; n <= 16; n++ {
			gs[n] = pbt.IntRange(0, n-1)
		}
		return gs
	}()
)

var semverNumber = pbt.Custom("numeric identifier", func(t *pbt.T) string {
	n := identLen.Draw(t, "")
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
var (
	semverPre        = pbt.OneOf(semverNumber, semverAlpha)
	semverPreRelease = optionalText(affix(joined(semverPre, 1, 4, "."), "-", ""))
	semverBuild      = optionalText(affix(joined(stringIn(alnum+"-", 1, 16), 1, 4, "."), "+", ""))
)
var semverAlpha = pbt.Custom("alphanumeric identifier", func(t *pbt.T) string {
	n := identLen.Draw(t, "")
	at := letterAt[n].Draw(t, "")
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
