// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package pbt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSPDXHeaders checks that every Go file in the repository, including
// the examples module, starts with the SPDX copyright and license lines.
func TestSPDXHeaders(t *testing.T) {
	want := []string{
		"// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>\n",
		"// SPDX-License-Identifier: Apache-2.0\n",
	}
	err := filepath.WalkDir(".", func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "testdata" || strings.HasPrefix(d.Name(), ".") && path != "." {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !strings.HasPrefix(string(data), want[0]+want[1]+"\n") {
			t.Errorf("%s lacks the SPDX header", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
