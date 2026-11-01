// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"slices"
	"strconv"
	"strings"
	"testing"

	"go.ymotongpoo.com/pbt"
)

func EncodeIDs(ids []int) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = strconv.Itoa(id)
	}
	return strings.Join(parts, ",")
}

func DecodeIDs(s string) ([]int, error) {
	if s == "" {
		return nil, nil
	}
	parts := strings.Split(s, ",")
	out := make([]int, len(parts))
	for i, p := range parts {
		v, err := strconv.Atoi(p)
		if err != nil {
			return nil, err
		}
		out[i] = v
	}
	return out, nil
}

func TestRoundTripDir1(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		ids := pbt.SliceOf(pbt.IntRange(-1000, 1000)).Draw(t, "ids")
		got, err := DecodeIDs(EncodeIDs(ids))
		if err != nil || !slices.Equal(got, ids) {
			t.Fatalf("Decode(Encode(%v)) = %v, %v", ids, got, err)
		}
	})
}

func TestDraftClaimSpaces(t *testing.T) {
	for _, s := range []string{"1, 02, 2", "1,02,2", "+1,02,2", "01,+2,-0"} {
		got, err := DecodeIDs(s)
		t.Logf("DecodeIDs(%q) = %v, err=%v ; re-encoded=%q", s, got, err, EncodeIDs(got))
	}
}

func TestCanonIdempotent(t *testing.T) {
	canon := func(s string) string {
		ids, err := DecodeIDs(s)
		if err != nil {
			return ""
		}
		return EncodeIDs(ids)
	}
	digits := pbt.StringOfN(pbt.SampledFrom([]rune("0123456789+-,")), 0, 8)
	pbt.Check(t, func(t *pbt.T) {
		s := digits.Draw(t, "s")
		if _, err := DecodeIDs(s); err != nil {
			t.Skip("not decodable")
		}
		if canon(canon(s)) != canon(s) {
			t.Fatalf("canon not idempotent for %q", s)
		}
	})
}
