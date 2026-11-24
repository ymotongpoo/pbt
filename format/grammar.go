// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package format

import (
	"strings"

	"go.ymotongpoo.com/pbt"
)

// optionalText keeps the alternative index and the entire part in one
// span, so shrinking can omit the part and reconnect the choices after it.
func optionalText(g pbt.Gen[string]) pbt.Gen[string] {
	return pbt.OneOf(pbt.Just(""), g)
}

func affix(g pbt.Gen[string], before, after string) pbt.Gen[string] {
	return pbt.Map(g, func(s string) string { return before + s + after })
}

func joined(g pbt.Gen[string], lo, hi int, sep string) pbt.Gen[string] {
	return pbt.Map(pbt.SliceOfN(g, lo, hi), func(s []string) string { return strings.Join(s, sep) })
}
