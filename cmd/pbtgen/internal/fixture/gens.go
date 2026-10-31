// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package fixture

import "go.ymotongpoo.com/pbt"

// emailGen is the generator the Email field names in its tag.
var emailGen = pbt.Map(pbt.StringN(1, 8), func(s string) string { return s + "@example.com" })

// Temperature has a hand-written generator, so pbtgen must refuse to write
// GenTemperature next to it.
type Temperature float64

// GenTemperature generates temperatures above absolute zero.
func GenTemperature() pbt.Gen[Temperature] {
	return pbt.Map(pbt.Float64Range(-273.15, 1000), func(f float64) Temperature { return Temperature(f) })
}
