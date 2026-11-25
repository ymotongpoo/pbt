// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

// Package coverage is an instrumented fixture for the coverage search tests.
package coverage

// Magic has distinct blocks for each matched byte. A loop would expose
// the same blocks at every depth and could not provide this feedback.
func Magic(b [6]byte) bool {
	if b[0] == 5 {
		if b[1] == 11 {
			if b[2] == 3 {
				if b[3] == 9 {
					if b[4] == 7 {
						if b[5] == 13 {
							return true
						}
					}
				}
			}
		}
	}
	return false
}
