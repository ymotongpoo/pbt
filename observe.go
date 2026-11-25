// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package pbt

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var flagObserve = flag.String("pbt.observe", "", "directory for OpenPBTStats observation JSONL files")

// Observations appends OpenPBTStats records for each Check or CheckExists to
// dir/<sanitized test name>.jsonl, as does -pbt.observe=dir. Subtest names make
// subdirectories; directories are created as needed. An empty dir disables
// observation, with no additional per-case allocations or timing calls.
//
// Records follow the test_case and info schemas at
// https://github.com/tyche-pbt/tyche-extension/blob/192ded8a5e10eae7d4406c9bb59971dcfc786695/observability-tools/src/datatypes.ts
// and https://hypothesis.readthedocs.io/en/latest/reference/integrations.html#observability.
// run_start is Unix time in seconds. how_generated is generate, target, replay,
// stability_replay or shrink. Each shrink operation also writes a minimal record
// with metadata.final=true: a snapshot of the chosen case, not another execution.
// Case status describes the property itself, so a rejected CheckExists candidate
// is failed and a witness is passed. Discards are gave_up.
//
// representation and arguments use the same rendered draws and limits as reports.
// Repeated argument labels get #2, #3, ... suffixes, avoiding existing labels.
// Features use label:<text> for Classify/Collect counts, aggregate:<text> for
// Aggregate counts, measure:<name> for the last Measure sample, cover:<label> for
// 0/1 hits, and target:<name> for utilities (Minimize uses the negated value).
// Non-finite feature values are strings, as with ReportJSON.
// timing contains non-overlapping generate:input and execute:test durations in
// seconds; execution includes setup and cleanup, excluding top-level Draw time.
// coverage is null. Metadata contains seed (hex), size, choices (count), final,
// and evaluation (a run-local index shared by a minimal snapshot and its case).
// An info record with a human-readable summary ends each run.
//
// Output is buffered in bounded batches and flushed at the end of the run.
// Parallel calls are synchronized within the process. Sanitized names can
// collide; property and run_start identify records in a shared file. Write errors
// call TB.Errorf once and stop recording, without stopping property evaluation.
// Abnormal process termination can lose buffered lines. No network sink is used.
func Observations(dir string) Option { return func(c *config) { c.observe = dir } }

type caseTiming struct {
	generation time.Duration
	execution  time.Duration
	evaluation int
}
type observationMetadata struct {
	Seed       string `json:"seed"`
	Size       int    `json:"size"`
	Choices    int    `json:"choices"`
	Final      bool   `json:"final"`
	Evaluation int    `json:"evaluation"`
}
type observationCase struct {
	Type           string              `json:"type"`
	RunStart       float64             `json:"run_start"`
	Property       string              `json:"property"`
	Status         string              `json:"status"`
	StatusReason   string              `json:"status_reason"`
	Representation string              `json:"representation"`
	Arguments      map[string]string   `json:"arguments"`
	HowGenerated   string              `json:"how_generated"`
	Features       map[string]any      `json:"features"`
	Timing         map[string]float64  `json:"timing"`
	Coverage       any                 `json:"coverage"`
	Metadata       observationMetadata `json:"metadata"`
}
type observationInfo struct {
	Type     string  `json:"type"`
	RunStart float64 `json:"run_start"`
	Property string  `json:"property"`
	Title    string  `json:"title"`
	Content  string  `json:"content"`
}
type observer struct {
	file        *os.File
	buffer      *bufio.Writer
	encoded     bytes.Buffer
	encoder     *json.Encoder
	runStart    float64
	phase       string
	evaluations int
	err         error
}

// lockedAppend keeps every buffered write together, even when sanitized names
// collide. ReportJSON uses the same lock. File handles are local to each run.
type lockedAppend struct{ file *os.File }

func (w lockedAppend) Write(p []byte) (int, error) {
	jsonFileMu.Lock()
	defer jsonFileMu.Unlock()
	return w.file.Write(p)
}
func (r *runner) startObservations() {
	o := &observer{phase: "replay", runStart: float64(r.output.start.UnixNano()) / 1e9}
	r.observer = o
	path := filepath.Join(r.cfg.observe, sanitizeName(r.tb.Name())+".jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		o.err = err
		return
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		o.err = err
		return
	}
	o.file = f
	o.buffer = bufio.NewWriterSize(lockedAppend{f}, 64<<10)
	o.encoder = json.NewEncoder(&o.encoded)
}
func (o *observer) write(value any) {
	if o.err != nil {
		return
	}
	o.encoded.Reset()
	if err := o.encoder.Encode(value); err != nil {
		o.err = err
		return
	}
	// Flush before adding a line that does not fit: a line is never split
	// between two independently locked writes, including lines over 64 KiB.
	line := o.encoded.Bytes()
	if len(line) > o.buffer.Available() {
		if err := o.buffer.Flush(); err != nil {
			o.err = err
			return
		}
	}
	_, o.err = o.buffer.Write(line)
}
func (r *runner) observeCase(t *T, phase string, final bool) {
	o := r.observer
	if o.err != nil {
		return
	}
	draws := r.caseDraws(t)
	args := make(map[string]string, len(draws))
	reserved := make(map[string]bool, len(draws))
	for _, d := range draws {
		reserved[d.Label] = true
	}
	parts := make([]string, 0, len(draws))
	for _, d := range draws {
		key := d.Label
		if _, exists := args[key]; exists {
			for n := 2; ; n++ {
				candidate := fmt.Sprintf("%s#%d", d.Label, n)
				if _, used := args[candidate]; !used && !reserved[candidate] {
					key = candidate
					break
				}
			}
		}
		args[key] = d.Value
		parts = append(parts, strings.Repeat("  ", d.Depth)+d.display+" = "+d.Value)
	}
	status, reason := "passed", ""
	if t.discarded {
		status, reason = "gave_up", t.discardReason
	} else if t.failed {
		status = "failed"
		reason = t.failKey
		if len(t.messages) > 0 {
			reason += ": " + strings.Join(t.messages, "; ")
		}
		if t.panicVal != nil {
			reason += fmt.Sprintf(": %v", t.panicVal)
		}
	}
	features := map[string]any{}
	for _, label := range t.labels {
		addFeatureCount(features, "label:"+label)
	}
	for _, label := range t.samples {
		addFeatureCount(features, "aggregate:"+label)
	}
	for _, m := range t.measures {
		features["measure:"+m.name] = jsonNumber(m.value)
	}
	for _, c := range t.covers {
		key := "cover:" + c.label
		if c.hit {
			features[key] = 1
		} else if _, exists := features[key]; !exists {
			features[key] = 0
		}
	}
	for i := 0; i < t.utilityCount(); i++ {
		u := t.utilityAt(i)
		features["target:"+u.name] = jsonNumber(u.value)
	}
	o.write(observationCase{
		Type: "test_case", RunStart: o.runStart, Property: r.tb.Name(), Status: status, StatusReason: reason,
		Representation: strings.Join(parts, "\n"), Arguments: args, HowGenerated: phase, Features: features,
		Timing:   map[string]float64{"generate:input": t.timing.generation.Seconds(), "execute:test": t.timing.execution.Seconds()},
		Metadata: observationMetadata{fmt.Sprintf("0x%x", r.cfg.seed), t.size, len(t.choices), final, t.timing.evaluation},
	})
}
func addFeatureCount(features map[string]any, key string) {
	n, _ := features[key].(int)
	features[key] = n + 1
}
func (r *runner) finishObservations() {
	o := r.observer
	report := &r.output.report
	content := fmt.Sprintf("%s: %d passed, %d rejected, %d discarded (seed %s); search %.6gs, shrink %.6gs", report.Status, report.Passed, report.Rejected, report.Discards, report.Seed, report.Durations.Search, report.Durations.Shrink)
	if r.budgetEnded {
		content += fmt.Sprintf("; %s budget exhausted", r.cfg.budget)
	}
	if len(report.Errors) > 0 {
		content += "\n\n" + strings.Join(report.Errors, "\n\n")
	}
	o.write(observationInfo{"info", o.runStart, r.tb.Name(), "pbt run", content})
	if o.file != nil {
		flushErr := o.buffer.Flush()
		closeErr := o.file.Close()
		if o.err == nil {
			o.err = flushErr
		}
		if o.err == nil {
			o.err = closeErr
		}
	}
	if o.err != nil {
		r.tb.Errorf("[pbt] cannot write observations under %s: %v", r.cfg.observe, o.err)
	}
}
