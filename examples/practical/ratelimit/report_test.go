// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package ratelimit

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"go.ymotongpoo.com/pbt"
)

func TestLimiterRunArtifacts(t *testing.T) {
	reportPath := filepath.Join(t.TempDir(), "runs.jsonl")
	observations := t.TempDir()
	const checks = 10000
	pbt.Check(t, func(t *pbt.T) {
		newHarness(Options{}).program().Draw(t, "cmds").RunSequential(t)
	}, pbt.Seed(1), pbt.Checks(checks), pbt.Budget(100*time.Millisecond),
		pbt.ReportJSON(reportPath), pbt.Observations(observations))

	// An agent or CI job needs only these fields. Ignore unknown fields so
	// additions to the report schema do not break the consumer.
	f, err := os.Open(reportPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var report struct {
		Schema          string `json:"schema"`
		Test            string `json:"test"`
		Status          string `json:"status"`
		Passed          int    `json:"checks_passed"`
		BudgetExhausted bool   `json:"budget_exhausted"`
		Statistics      struct {
			Cases   int            `json:"cases"`
			Classes map[string]int `json:"classes"`
			Samples int            `json:"samples"`
		} `json:"statistics"`
	}
	decoder := json.NewDecoder(f)
	if err := decoder.Decode(&report); err != nil {
		t.Fatal(err)
	}
	if err := decoder.Decode(new(json.RawMessage)); err != io.EOF {
		t.Fatalf("expected one run report, got another record or error: %v", err)
	}
	if report.Schema != "go.ymotongpoo.com/pbt/report/v1" || report.Test != t.Name() || report.Status != "passed" {
		t.Fatalf("unexpected run identity or outcome: %+v", report)
	}
	// The budget can end a successful run before the case limit. The count
	// depends on the host's speed, so do not require an exact number.
	if report.Passed <= 0 || report.Passed > checks || report.Passed < checks && !report.BudgetExhausted {
		t.Fatalf("unexpected search counts: %+v", report)
	}
	if report.Statistics.Cases != report.Passed {
		t.Fatalf("statistics cover %d cases, want %d", report.Statistics.Cases, report.Passed)
	}
	commands := 0
	for _, name := range []string{"allow", "advance", "sweep"} {
		commands += report.Statistics.Classes[name]
	}
	if commands == 0 || commands != report.Statistics.Samples {
		t.Fatalf("unexpected command statistics: %+v", report.Statistics)
	}
	t.Logf("JSON: status=%s checks_passed=%d budget_exhausted=%t statistics=%+v",
		report.Status, report.Passed, report.BudgetExhausted, report.Statistics)

	trace, err := os.Open(filepath.Join(observations, t.Name()+".jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer trace.Close()
	decoder = json.NewDecoder(trace)
	cases, infos := 0, 0
	for {
		var record struct {
			Type      string            `json:"type"`
			Property  string            `json:"property"`
			Status    string            `json:"status"`
			Arguments map[string]string `json:"arguments"`
		}
		if err := decoder.Decode(&record); err == io.EOF {
			break
		} else if err != nil {
			t.Fatal(err)
		}
		if record.Property != t.Name() || infos != 0 {
			t.Fatalf("unexpected property or record after the final info: %+v", record)
		}
		switch record.Type {
		case "test_case":
			if record.Status != "passed" || record.Arguments["cmds"] == "" {
				t.Fatalf("expected a passing case with its command program: %+v", record)
			}
			cases++
		case "info":
			infos++
		default:
			t.Fatalf("unexpected observation type %q", record.Type)
		}
	}
	if cases != report.Passed || infos != 1 {
		t.Fatalf("got %d test_case and %d info records, want %d and 1", cases, infos, report.Passed)
	}
	t.Logf("OpenPBTStats: %d test_case records followed by one info record", cases)
}
