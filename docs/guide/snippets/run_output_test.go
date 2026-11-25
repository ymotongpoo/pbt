// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package snippets

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"go.ymotongpoo.com/pbt"
)

func TestSearchBudget(t *testing.T) {
	// synctest advances time without waiting three real seconds.
	synctest.Test(t, func(t *testing.T) {
		logs := passingLogs(t, "TestBudget", func(t *pbt.T) {
			pbt.Int().Draw(t, "n")
			time.Sleep(time.Second)
		}, pbt.Seed(1), pbt.Checks(1000), pbt.Budget(3*time.Second))
		mustContain(t, logs,
			"[pbt] OK, passed 3 checks in the 3s budget (seed 0x1)")
	})
}

// snippet: machine-output
func checkRecordedRun(tb pbt.TB, reportPath, observationDir string) {
	pbt.Check(tb, func(t *pbt.T) {
		n := pbt.IntRange(0, 9).Draw(t, "n")
		t.Classify(n%2 == 0, "even")
		if n < 0 || n > 9 {
			t.Fatalf("out of range: %d", n)
		}
	}, pbt.Seed(1), pbt.Checks(3),
		pbt.ReportJSON(reportPath), pbt.Observations(observationDir))
}

// snippet: end

func TestMachineReadableOutput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runs.jsonl")
	dir := t.TempDir()
	rec := &recorder{name: "TestRecordedRun"}
	checkRecordedRun(rec, path, dir)
	if len(rec.errs) != 0 {
		t.Fatal(rec.errs)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(data), "\n") != 1 {
		t.Fatalf("want one run report: %s", data)
	}
	var report struct {
		Schema string `json:"schema"`
		Status string `json:"status"`
		Passed int    `json:"checks_passed"`
	}
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatal(err)
	}
	if report.Schema != "go.ymotongpoo.com/pbt/report/v1" || report.Status != "passed" || report.Passed != 3 {
		t.Fatalf("unexpected report: %s", data)
	}
	t.Logf("JSON report:\n%s", data)
	data, err = os.ReadFile(filepath.Join(dir, "TestRecordedRun.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 4 {
		t.Fatalf("want three cases and one info record: %s", data)
	}
	for i, line := range lines {
		var record struct {
			Type     string `json:"type"`
			Property string `json:"property"`
		}
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatal(err)
		}
		want := "test_case"
		if i == 3 {
			want = "info"
		}
		if record.Type != want || record.Property != "TestRecordedRun" {
			t.Fatalf("unexpected observation: %s", line)
		}
	}
}
