// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"testing"

	"go.ymotongpoo.com/pbt"
)

// 第18章 18.4: Sizes オプション、Sized、Resize が v0.3.2 の実在APIであることの確認。
func TestSizesSizedResize(t *testing.T) {
	sized := pbt.Sized(func(size int) pbt.Gen[[]int] {
		return pbt.SliceOfN(pbt.Int(), 0, size)
	})
	pbt.Check(t, func(t *pbt.T) {
		if s := t.Size(); s < 1 || s > 60 {
			t.Fatalf("size %d out of Sizes(1, 60)", s)
		}
		xs := sized.Draw(t, "xs")
		if len(xs) > t.Size() {
			t.Fatalf("len %d exceeds size %d", len(xs), t.Size())
		}
		if ys := pbt.Resize(0, sized).Draw(t, "ys"); len(ys) != 0 {
			t.Fatalf("Resize(0) produced %v", ys)
		}
	}, pbt.Checks(200), pbt.Sizes(1, 60))
}
