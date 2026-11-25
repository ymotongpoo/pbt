// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package snippets

// snippet: coverage-fixture
// This function lives in a .go file: go test does not instrument _test.go files.
func recognizesHeader(b []byte) bool {
	if len(b) == 4 {
		if b[0] == 'P' {
			if b[1] == 'B' {
				if b[2] == 'T' {
					if b[3] == '!' {
						return true
					}
				}
			}
		}
	}
	return false
}

// snippet: end
