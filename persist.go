// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package pbt

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// persistHeader is the first line of a saved test case. The version suffix
// lets the format change later.
const persistHeader = "go.ymotongpoo.com/pbt v1"

func (r *runner) saveDir() string {
	return filepath.Join(r.cfg.testdata, sanitizeName(r.tb.Name()))
}

// sanitizeName turns a test name into a relative path. Subtests become
// subdirectories; characters outside a conservative set become underscores.
func sanitizeName(name string) string {
	parts := strings.Split(name, "/")
	for i, p := range parts {
		var b strings.Builder
		for _, c := range p {
			switch {
			case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '_', c == '-', c == '.':
				b.WriteRune(c)
			default:
				b.WriteByte('_')
			}
		}
		s := b.String()
		if s == "" || s == "." || s == ".." {
			s = "_" + s
		}
		parts[i] = s
	}
	return filepath.Join(parts...)
}

// encodeCase serializes a test case. The comment lines carry the drawn
// values for readers; only the size and the choices are read back.
func encodeCase(t *T) []byte {
	var b bytes.Buffer
	fmt.Fprintf(&b, "%s\n", persistHeader)
	fmt.Fprintf(&b, "size %d\n", t.size)
	for _, d := range t.draws {
		fmt.Fprintf(&b, "# %s\n", strings.ReplaceAll(formatDraw(d), "\n", " "))
	}
	b.WriteString("choices")
	for i, c := range t.choices {
		if i%16 == 0 {
			b.WriteByte('\n')
		} else {
			b.WriteByte(' ')
		}
		b.WriteString(strconv.FormatUint(c, 10))
	}
	b.WriteByte('\n')
	return b.Bytes()
}

func decodeCase(data []byte) (size int, choices []uint64, err error) {
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(nil, 1<<24)
	if !sc.Scan() || sc.Text() != persistHeader {
		return 0, nil, fmt.Errorf("missing header %q", persistHeader)
	}
	inChoices := false
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		switch {
		case line == "" || strings.HasPrefix(line, "#"):
		case strings.HasPrefix(line, "size "):
			size, err = strconv.Atoi(strings.TrimPrefix(line, "size "))
			if err != nil {
				return 0, nil, fmt.Errorf("bad size line %q", line)
			}
		case line == "choices":
			inChoices = true
		case inChoices:
			for _, f := range strings.Fields(line) {
				c, err := strconv.ParseUint(f, 10, 64)
				if err != nil {
					return 0, nil, fmt.Errorf("bad choice %q", f)
				}
				choices = append(choices, c)
			}
		default:
			return 0, nil, fmt.Errorf("unexpected line %q", line)
		}
	}
	if err := sc.Err(); err != nil {
		return 0, nil, err
	}
	if !inChoices {
		return 0, nil, fmt.Errorf("no choices section")
	}
	return size, choices, nil
}

func caseFileName(size int, choices []uint64) string {
	h := sha256.New()
	fmt.Fprintf(h, "%d\n", size)
	for _, c := range choices {
		fmt.Fprintf(h, "%d ", c)
	}
	return hex.EncodeToString(h.Sum(nil))[:32]
}

func (r *runner) save(t *T) (string, error) {
	dir := r.saveDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, caseFileName(t.size, t.choices))
	if err := os.WriteFile(path, encodeCase(t), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// replaySaved runs every saved test case of this test and returns the first
// one that still fails, with its path.
func (r *runner) replaySaved() (*T, string) {
	dir := r.saveDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, ""
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		path := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			r.tb.Logf("[pbt] cannot read %s: %v", path, err)
			continue
		}
		size, choices, err := decodeCase(data)
		if err != nil {
			r.tb.Logf("[pbt] skipping %s: %v", path, err)
			continue
		}
		if t := r.run(choices, size, nil); t.failed && !t.discarded {
			return t, path
		}
	}
	return nil, ""
}
