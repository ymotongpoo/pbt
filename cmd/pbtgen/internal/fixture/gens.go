// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package fixture

import "go.ymotongpoo.com/pbt"

// emailGen is the generator the Email field names in its tag.
var emailGen = pbt.Map(pbt.StringN(1, 8), func(s string) string { return s + "@example.com" })
