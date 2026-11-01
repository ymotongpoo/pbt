// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package ch17

import (
	"fmt"
	"strings"
	"testing"

	"go.ymotongpoo.com/pbt"
)

// quiet は pbt.Check の失敗を記録だけする testing.TB である。
type quiet struct {
	testing.TB
	failed, gaveUp bool
}

func (q *quiet) Errorf(format string, args ...any) {
	q.failed = true
	q.gaveUp = q.gaveUp || strings.Contains(format, "gave up") || strings.Contains(fmt.Sprintf(format, args...), "gave up")
}
func (q *quiet) Error(...any)          { q.failed = true }
func (q *quiet) Fatalf(string, ...any) { q.failed = true }
func (q *quiet) Fatal(...any)          { q.failed = true }
func (q *quiet) Fail()                 { q.failed = true }
func (q *quiet) FailNow()              { q.failed = true }
func (q *quiet) Logf(string, ...any)   {}
func (q *quiet) Log(...any)            {}
func (q *quiet) Helper()               {}
func (q *quiet) Failed() bool          { return q.failed }

// caught は既定の100ケースの実行を300回、シードを変えて繰り返し、
// 変異を捕まえた回数を返す。
func caught(t *testing.T, p pbt.Property) (n, gaveUp int) {
	for i := range 300 {
		q := &quiet{TB: t}
		pbt.Check(q, p, pbt.NoSave(), pbt.NoReplay(), pbt.Seed(uint64(i+1)))
		switch {
		case q.gaveUp:
			gaveUp++
		case q.failed:
			n++
		}
	}
	return n, gaveUp
}

func TestDetectionRates(t *testing.T) {
	for _, c := range []struct {
		name string
		p    pbt.Property
	}{
		{"assume bad", assumeProperty(indexSortedBad)},
		{"assume ok", assumeProperty(indexSortedOK)},
		{"generator bad", generatorProperty(indexSortedBad)},
		{"narrow boundary", narrowProperty(parseLimitBoundary)},
		{"full boundary", fullRangeProperty(parseLimitBoundary)},
		{"frequency boundary", boundaryProperty(parseLimitBoundary)},
		{"model only double", modelOnlyProperty(mkDouble, false)},
		{"model only forget", modelOnlyProperty(mkForget, false)},
		{"got double", modelOnlyProperty(mkDouble, true)},
		{"got forget", modelOnlyProperty(mkForget, true)},
		{"alias first", aliasProperty(firstElement)},
		{"clone first", cloneProperty(firstElement)},
		{"copy bytes", copyProperty(truncateBytes)},
		{"spec bytes", specProperty(truncateBytes)},
	} {
		n, g := caught(t, c.p)
		t.Logf("%-20s caught %d/300, gave up %d", c.name, n, g)
	}
}

// 変異と検出の対応表：largest の3つのプロパティと3つの変異
func TestMatrix(t *testing.T) {
	props := []struct {
		name string
		p    func(func([]int) int) pbt.Property
	}{{"element", elementProperty}, {"upper", upperProperty}, {"reverse", cloneProperty}}
	muts := []struct {
		name string
		f    func([]int) int
	}{{"zero init", largestZeroInit}, {"flipped", largestFlipped}, {"first", firstElement}}
	for _, m := range muts {
		for _, p := range props {
			n, _ := caught(t, p.p(m.f))
			t.Logf("%-10s %-8s %d/300", m.name, p.name, n)
		}
	}
}

// 等価な変異：hi の初期値を変えても振る舞いは変わらない
func indexSortedHi(xs []int, v int) int {
	if len(xs) < 16 {
		return indexLinear(xs, v)
	}
	lo, hi := 0, len(xs)-1
	for lo < hi {
		mid := (lo + hi) / 2
		if xs[mid] < v {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	if lo < len(xs) && xs[lo] == v {
		return lo
	}
	return -1
}

func TestEquivalentHiSurvives(t *testing.T) {
	if n, _ := caught(t, generatorProperty(indexSortedHi)); n != 0 {
		t.Fatalf("caught %d/300", n)
	}
}

// 出金の反映忘れをどのコマンドが検出するかはシードで変わる
func TestForgetDetectors(t *testing.T) {
	counts := map[string]int{}
	for i := range 100 {
		q := &detector{quiet: quiet{TB: t}}
		pbt.Check(q, ledgerProperty(mkForget), pbt.NoSave(), pbt.NoReplay(), pbt.Seed(uint64(i+1)))
		counts[q.which]++
	}
	t.Log(counts)
}

type detector struct {
	quiet
	which string
}

func (d *detector) Errorf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	for _, name := range []string{"postcondition of deposit", "postcondition of balance", "postcondition of overdraft", "postcondition of withdraw"} {
		if strings.Contains(msg, name) {
			d.which = name
		}
	}
}
