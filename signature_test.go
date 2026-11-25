// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package pbt

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSignatureCaseLifetime(t *testing.T) {
	for _, tc := range []struct {
		name string
		prop Property
		want string
	}{
		{"before", func(pt *T) { pt.Signature("cause"); pt.Fatal("bad") }, "cause"},
		{"after", func(pt *T) { pt.Error("bad"); pt.Signature("cause") }, "cause"},
		{"last", func(pt *T) { pt.Signature("old"); pt.Error("bad"); pt.Signature("cause") }, "cause"},
		{"panic", func(pt *T) { pt.Signature("cause"); panic(42) }, "cause"},
		{"deferred", func(pt *T) { defer pt.Signature("cause"); panic(42) }, "cause"},
		{"cleanup", func(pt *T) {
			pt.Signature("old")
			pt.Cleanup(func() { pt.Signature("cause") })
			pt.Fatal("bad")
		}, "cause"},
		{"empty", func(pt *T) { pt.Signature(""); pt.Fail() }, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := newT("signature", nil, 1, 42, nil)
			c.execute(tc.prop)
			if !c.failed || c.failKey != signaturePrefix+tc.want {
				t.Fatalf("failed %v, key %q", c.failed, c.failKey)
			}
		})
	}
	c := newT("signature", nil, 1, 42, nil)
	c.execute(func(pt *T) { pt.Signature("unused") })
	if c.failed || c.failKey != "" {
		t.Fatalf("passing case acquired a failure: %q", c.failKey)
	}
}

func TestSignatureConjunction(t *testing.T) {
	for _, panics := range []bool{false, true} {
		c := newT("signature", nil, 1, 42, nil)
		c.execute(Conjunction(
			Named{"first", func(pt *T) {
				pt.Signature("first")
				if panics {
					panic("bad")
				}
				pt.Error("bad")
			}},
			Named{"passing", Conjunction(Named{"nested", func(pt *T) { pt.Signature("last") }})},
		))
		if !c.failed || c.failKey != signaturePrefix+"last" || !strings.Contains(strings.Join(c.messages, " "), "first:") {
			t.Fatalf("failed %v, key %q, messages %q", c.failed, c.failKey, c.messages)
		}
	}
}

func TestSignatureOverridesStatefulCommand(t *testing.T) {
	for _, name := range []string{"read", "write"} {
		c := newT("signature", nil, 1, 42, nil)
		c.execute(func(pt *T) {
			cmd := Command{Name: name, Post: func(Env, any) error {
				pt.Signature("wrong value")
				return fmt.Errorf("bad %s", name)
			}}
			Commands(func() {}, func() CommandGen { return Just(cmd) }, Length(1, 1)).Draw(pt, "program").RunSequential(pt)
		})
		if !c.failed || c.failKey != signaturePrefix+"wrong value" || !strings.Contains(strings.Join(c.messages, " "), name) {
			t.Fatalf("key %q, messages %q", c.failKey, c.messages)
		}
	}
}

func TestSignatureShrinkAndMultipleFailures(t *testing.T) {
	prop := func(pt *T) {
		x := IntRange(0, 100).Draw(pt, "x")
		pt.Signature("small")
		if x >= 10 {
			pt.Signature("large")
		}
		pt.Error("same assertion site")
	}
	cfg, _ := newConfig([]Option{Seed(1), NoSave()})
	r := &runner{tb: &fakeTB{}, cfg: cfg, prop: prop}
	minimal, _ := r.shrink(r.run([]uint64{50}, 1, nil))
	if minimal.draws[0].value != 10 || minimal.failKey != signaturePrefix+"large" {
		t.Fatal(minimal.draws, minimal.failKey)
	}
	tb := healthRun(prop, MaxFailures(2))
	if len(tb.errors) != 2 || !strings.Contains(healthOutput(tb), `failure was named "small"`) || !strings.Contains(healthOutput(tb), `failure was named "large"`) {
		t.Fatal(healthOutput(tb))
	}
}

func TestSignatureSavedSuffix(t *testing.T) {
	cfg, _ := newConfig([]Option{Seed(1), MaxFailures(2), TestdataDir(t.TempDir())})
	r := &runner{tb: &fakeTB{name: "signature"}, cfg: cfg}
	for _, name := range []string{"one", "two"} {
		r.prop = func(pt *T) { pt.Signature(name); pt.Fail() }
		c := r.run(nil, 1, nil)
		path, err := r.save(c)
		if err != nil {
			t.Fatal(err)
		}
		h := sha256.Sum256([]byte(c.failKey))
		if want := caseFileName(c.size, c.choices) + "-" + hex.EncodeToString(h[:8]); filepath.Base(path) != want {
			t.Fatalf("path %s, want %s", path, want)
		}
	}
	files, err := os.ReadDir(r.saveDir())
	if err != nil || len(files) != 2 {
		t.Fatal(files, err)
	}
}

func TestSignatureFlakyIdentity(t *testing.T) {
	for _, changed := range []bool{false, true} {
		calls := 0
		tb := healthRun(func(pt *T) {
			calls++
			pt.Signature("cause/path:123")
			if calls%2 == 0 {
				if changed {
					pt.Signature("other")
				}
				pt.Fatal("second assertion site")
			}
			pt.Fatal("first assertion site")
		})
		out := healthOutput(tb)
		if strings.Contains(out, "HealthFlaky") != changed || !strings.Contains(out, `was named "cause/path:123"`) {
			t.Fatal(out)
		}
		if changed && !strings.Contains(out, `failed elsewhere: named "other"`) {
			t.Fatal(out)
		}
	}
}
