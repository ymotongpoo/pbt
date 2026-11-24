// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package format

import (
	"encoding/base32"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"testing"

	"go.ymotongpoo.com/pbt"
)

func TestEncodings(t *testing.T) {
	for _, tc := range []struct {
		g       pbt.Gen[string]
		decode  func(string) ([]byte, error)
		encode  func([]byte) string
		quantum int
	}{
		{Base64(), base64.StdEncoding.Strict().DecodeString, base64.StdEncoding.EncodeToString, 3},
		{Base64URL(), base64.URLEncoding.Strict().DecodeString, base64.URLEncoding.EncodeToString, 3},
		{RawBase64(), base64.RawStdEncoding.Strict().DecodeString, base64.RawStdEncoding.EncodeToString, 3},
		{RawBase64URL(), base64.RawURLEncoding.Strict().DecodeString, base64.RawURLEncoding.EncodeToString, 3},
		{Base32(), base32.StdEncoding.DecodeString, base32.StdEncoding.EncodeToString, 5},
		{Base32Hex(), base32.HexEncoding.DecodeString, base32.HexEncoding.EncodeToString, 5},
		{RawBase32(), base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString, base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString, 5},
		{RawBase32Hex(), base32.HexEncoding.WithPadding(base32.NoPadding).DecodeString, base32.HexEncoding.WithPadding(base32.NoPadding).EncodeToString, 5},
		{Base16(), hex.DecodeString, func(b []byte) string { return strings.ToUpper(hex.EncodeToString(b)) }, 1},
		{Hex(), hex.DecodeString, hex.EncodeToString, 1},
	} {
		t.Run(tc.g.String(), func(t *testing.T) {
			checkMinimumSeeds(t, tc.g, "")
			checkValid(t, tc.g, func(s string) string {
				b, e := tc.decode(s)
				if e != nil {
					return e.Error()
				}
				if tc.encode(b) != s {
					return "not canonical"
				}
				return ""
			})
			pbt.Check(t, func(pt *pbt.T) {
				s := tc.g.Draw(pt, "")
				b, _ := tc.decode(s)
				for r := 0; r < tc.quantum; r++ {
					pt.Cover(len(b)%tc.quantum == r, 2, string(rune('0'+r)))
				}
				pt.Cover(s == "", 1, "empty")
				pt.Cover(len(b) > 0, 50, "nonempty")
			}, pbt.Seed(1), pbt.Checks(2000), pbt.NoSave(), pbt.CheckCoverage())
			if s := minimalOf(t, tc.g, always); s != "" {
				t.Errorf("minimum %q", s)
			}
		})
	}
}
