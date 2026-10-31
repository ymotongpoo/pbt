// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

//go:build ignore

// This file is excluded by its build constraint. It redeclares a type of
// the package, which would make type-checking fail if pbtgen read it.

package fixture

// Order collides with the Order in types.go.
type Order int
