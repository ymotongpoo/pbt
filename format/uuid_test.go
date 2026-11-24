// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package format

import (
	"regexp"
	"strings"
	"testing"

	"go.ymotongpoo.com/pbt"
)

var uuidRe = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-([0-9a-f])[0-9a-f]{3}-([0-9a-f])[0-9a-f]{3}-[0-9a-f]{12}$`)

func checkUUID(versions string) func(string) string {
	return func(s string) string {
		m := uuidRe.FindStringSubmatch(s)
		switch {
		case m == nil:
			return "not the text form of a UUID"
		case s == "00000000-0000-0000-0000-000000000000" || s == "ffffffff-ffff-ffff-ffff-ffffffffffff":
			if versions != "12345678" {
				return "Nil or Max with versions given"
			}
		case !strings.Contains(versions, m[1]):
			return "version " + m[1]
		case !strings.Contains("89ab", m[2]):
			return "variant " + m[2]
		}
		return ""
	}
}

func TestUUID(t *testing.T) {
	checkValid(t, UUID(), checkUUID("12345678"))
	checkValid(t, UUID(4, 7), checkUUID("47"))
	pbt.Check(t, func(pt *pbt.T) {
		s := UUID().Draw(pt, "s")
		pt.Cover(s[14] == '4', 20, "version 4")
		pt.Cover(s[14] == '7', 15, "version 7")
		pt.Cover(s == "00000000-0000-0000-0000-000000000000", 2, "Nil")
		pt.Cover(s == "ffffffff-ffff-ffff-ffff-ffffffffffff", 2, "Max")
	}, pbt.Seed(1), pbt.Checks(2000), pbt.NoSave(), pbt.CheckCoverage())
	if got := minimalOf(t, UUID(), always); got != "00000000-0000-0000-0000-000000000000" {
		t.Errorf("shrinks to %q", got)
	}
	if got := minimalOf(t, UUID(7, 4), always); got != "00000000-0000-7000-8000-000000000000" {
		t.Errorf("UUID(7, 4) shrinks to %q", got)
	}
	if UUID(7, 4).String() != "UUID(7, 4)" {
		t.Errorf("name %q", UUID(7, 4))
	}
	defer func() {
		if msg, _ := recover().(string); !strings.Contains(msg, "versions are 1 to 8") {
			t.Errorf("panic %q", msg)
		}
	}()
	UUID(9)
}
