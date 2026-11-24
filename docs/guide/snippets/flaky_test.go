// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package snippets

import (
	"sync/atomic"
	"testing"

	"go.ymotongpoo.com/pbt"
)

// snippet: flaky
// The cache keeps a hit counter across cases, so the outcome of a case
// depends on the cases that ran before it.
var hits atomic.Int64

var flakyProperty = func(t *pbt.T) {
	key := pbt.IntRange(0, 9).Draw(t, "key")
	if hits.Add(1)%7 == 0 && key > 2 {
		t.Fatalf("cache miss for key %d", key)
	}
}

// snippet: end

func TestFlakyIsReported(t *testing.T) {
	hits.Store(0)
	report := failureReport(t, "TestCache", flakyProperty, pbt.Seed(1))
	mustContain(t, report, "[pbt] health: HealthFlaky: the failure did not recur when its input was replayed")
}
