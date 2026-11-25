// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package pbt

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

var flagJSON = flag.String("pbt.json", "", "append a JSON run report to this file")

// ReportJSON appends one JSON object and newline after each Check or CheckExists
// to path, as does -pbt.json=path. An empty path disables reports. The parent
// directory must exist. Writes use O_APPEND, a process-wide mutex and one Write
// per line, including parallel subtests. This does not coordinate separate
// processes on filesystems that lack atomic append. Write or close errors call
// TB.Errorf: requesting a report makes its delivery part of the test's success.
//
// The schema is "go.ymotongpoo.com/pbt/report/v1". Consumers should ignore unknown
// fields. Its fields are:
//   - schema, test, seed (a hexadecimal string), kind (check or exists), status
//     (passed, failed, gave_up, no_witness or error), and errors (strings).
//   - checks_passed, rejected, discards: search-only counts; rejected counts
//     failing candidates, including CheckExists candidates. Replay and shrink
//     evaluations are excluded. A found witness counts as one passed check.
//   - budget_exhausted and durations_seconds with total, search and shrink.
//     Total includes replay and reporting, up to serialization; search includes
//     case execution; shrink includes all failures' or the witness's shrinking.
//   - failures: objects with identity, headline, draws, saved_path and reproduce.
//     Each draw has label, generator, value (rendered text), and depth. Draws obey
//     ReportLimit and ReportDepth, and preserve duplicates in their original order.
//   - witness: a case object like a failure, present only when CheckExists succeeds.
//   - health: objects with kind (a HealthCheck constant name) and message.
//   - statistics: cases, labels (Classify/Collect counts), classes (Aggregate
//     counts), samples (total Aggregate samples), covers (label to required_percent,
//     hits and verdict: met, unmet or undecided), and measures (name to count,
//     min, max, sum and mean). These follow the search's accumulated statistics;
//     CheckExists accumulates rejected candidates rather than its witness.
//   - objectives: objects with name and best over non-discarded search cases,
//     using Maximize's sign convention
//     (Minimize negates values).
//
// Non-finite statistics or objective numbers are strings: "NaN", "+Inf", "-Inf".
// Empty collections are emitted as [] or {}. Errors before seed selection leave
// seed at its configured value. A report records this invocation's outcome,
// independently of other errors already reported by the enclosing test.
func ReportJSON(path string) Option { return func(c *config) { c.reportJSON = path } }

const reportSchema = "go.ymotongpoo.com/pbt/report/v1"

type jsonDraw struct {
	Label     string `json:"label"`
	Generator string `json:"generator"`
	Value     string `json:"value"`
	Depth     int    `json:"depth"`
	display   string
}
type jsonCase struct {
	Identity  string     `json:"identity"`
	Headline  string     `json:"headline"`
	Draws     []jsonDraw `json:"draws"`
	SavedPath string     `json:"saved_path"`
	Reproduce string     `json:"reproduce"`
}
type jsonHealth struct {
	Kind    string `json:"kind"`
	Message string `json:"message"`
}
type jsonObjective struct {
	Name string `json:"name"`
	Best any    `json:"best"`
}
type jsonCover struct {
	Required float64 `json:"required_percent"`
	Hits     int     `json:"hits"`
	Verdict  string  `json:"verdict"`
}
type jsonMeasure struct {
	Count int `json:"count"`
	Min   any `json:"min"`
	Max   any `json:"max"`
	Sum   any `json:"sum"`
	Mean  any `json:"mean"`
}
type jsonStats struct {
	Cases    int                    `json:"cases"`
	Labels   map[string]int         `json:"labels"`
	Classes  map[string]int         `json:"classes"`
	Samples  int                    `json:"samples"`
	Covers   map[string]jsonCover   `json:"covers"`
	Measures map[string]jsonMeasure `json:"measures"`
}
type jsonDurations struct {
	Total  float64 `json:"total"`
	Search float64 `json:"search"`
	Shrink float64 `json:"shrink"`
}
type jsonReport struct {
	Schema      string          `json:"schema"`
	Test        string          `json:"test"`
	Seed        string          `json:"seed"`
	Kind        string          `json:"kind"`
	Status      string          `json:"status"`
	Errors      []string        `json:"errors"`
	Passed      int             `json:"checks_passed"`
	Rejected    int             `json:"rejected"`
	Discards    int             `json:"discards"`
	BudgetEnded bool            `json:"budget_exhausted"`
	Durations   jsonDurations   `json:"durations_seconds"`
	Failures    []jsonCase      `json:"failures"`
	Witness     *jsonCase       `json:"witness,omitempty"`
	Health      []jsonHealth    `json:"health"`
	Statistics  jsonStats       `json:"statistics"`
	Objectives  []jsonObjective `json:"objectives"`
}
type runOutput struct {
	report jsonReport
	start  time.Time
	paths  map[string]string
	best   []utility
}

// outputTB records only errors issued by this runner, not other tests sharing TB.
type outputTB struct {
	TB
	output *runOutput
}

func (tb *outputTB) Errorf(format string, args ...any) {
	tb.TB.Helper()
	if tb.output.report.Status == "passed" {
		tb.output.report.Status = "failed"
	}
	tb.output.report.Errors = append(tb.output.report.Errors, fmt.Sprintf(format, args...))
	tb.TB.Errorf(format, args...)
}
func (r *runner) startOutput() {
	kind := "check"
	if r.exists {
		kind = "exists"
	}
	r.output = &runOutput{start: time.Now(), paths: map[string]string{}, report: jsonReport{
		Schema: reportSchema, Test: r.tb.Name(), Seed: fmt.Sprintf("0x%x", r.cfg.seed), Kind: kind, Status: "passed",
		Errors: []string{}, Failures: []jsonCase{}, Health: []jsonHealth{}, Objectives: []jsonObjective{},
	}}
	r.tb = &outputTB{TB: r.tb, output: r.output}
	if r.cfg.observe != "" {
		r.startObservations()
	}
}
func (r *runner) outputStatus(status string) {
	if r.output != nil {
		r.output.report.Status = status
	}
}
func (r *runner) outputSearchCase(t *T) {
	if !t.discarded {
		for i := 0; i < t.utilityCount(); i++ {
			u := t.utilityAt(i)
			index := -1
			for j, old := range r.output.best {
				if old.name == u.name {
					index = j
					break
				}
			}
			if index < 0 {
				r.output.best = append(r.output.best, u)
			} else if u.value > r.output.best[index].value {
				r.output.best[index] = u
			}
		}
	}
	switch {
	case t.discarded:
		r.output.report.Discards++
	case t.failed:
		r.output.report.Rejected++
	default:
		r.output.report.Passed++
	}
}
func (r *runner) finishOutput() {
	out := &r.output.report
	out.BudgetEnded = r.budgetEnded
	out.Statistics = reportStatistics(r.stats)
	if len(out.Failures) == 0 {
		r.outputHealth()
	}
	for _, o := range r.output.best {
		out.Objectives = append(out.Objectives, jsonObjective{o.name, jsonNumber(o.value)})
	}
	out.Durations.Total = time.Since(r.output.start).Seconds()
	if r.observer != nil {
		r.finishObservations()
	}
	if r.cfg.reportJSON != "" {
		if err := appendJSONLine(r.cfg.reportJSON, out); err != nil {
			r.tb.Errorf("[pbt] cannot write JSON report %s: %v", r.cfg.reportJSON, err)
		}
	}
}
func (r *runner) outputHealth() {
	for _, n := range r.healthNotes() {
		note := jsonHealth{n.check.String(), n.text}
		duplicate := false
		for _, old := range r.output.report.Health {
			if old == note {
				duplicate = true
				break
			}
		}
		if !duplicate {
			r.output.report.Health = append(r.output.report.Health, note)
		}
	}
}
func (r *runner) caseReport(t *T, headline string) jsonCase {
	path := r.output.paths[t.failKey]
	parts := strings.Split(r.tb.Name(), "/")
	for i, p := range parts {
		parts[i] = "^" + regexp.QuoteMeta(p) + "$"
	}
	reproduce := "go test -run " + shellQuote(strings.Join(parts, "/"))
	if path != "" {
		reproduce += " -pbt.replay=" + shellQuote(path)
	} else {
		reproduce += fmt.Sprintf(" -pbt.seed=0x%x", r.cfg.seed)
	}
	if r.cfg.maxFailures > 1 {
		reproduce += fmt.Sprintf(" -pbt.failures=%d", r.cfg.maxFailures)
	}
	return jsonCase{t.failKey, headline, r.caseDraws(t), path, reproduce}
}
func shellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }

// caseDraws is shared with observation traces; rendering never invokes OnFail.
func (r *runner) caseDraws(t *T) []jsonDraw {
	draws := make([]jsonDraw, 0, len(t.draws))
	for _, d := range t.draws {
		if d.depth > r.cfg.reportDepth {
			continue
		}
		label := drawName(d)
		if label == "" {
			label = d.gen
			if site := shortSite(d.pc); site != "" {
				label += " at " + site
			}
		}
		draws = append(draws, jsonDraw{label, d.gen, renderDrawValue(d, r.cfg.valueLimit), d.depth, formatDrawHead(d, r.cfg.verbose)})
	}
	return draws
}
func reportStatistics(st *stats) jsonStats {
	if st == nil {
		st = newStats()
	}
	out := jsonStats{Cases: st.cases, Labels: st.labels, Classes: st.samples, Samples: st.nsamples,
		Covers: map[string]jsonCover{}, Measures: map[string]jsonMeasure{}}
	for label, c := range st.covers {
		verdict := "undecided"
		switch st.status(c) {
		case coverMet:
			verdict = "met"
		case coverUnmet:
			verdict = "unmet"
		}
		out.Covers[label] = jsonCover{c.pct, c.hit, verdict}
	}
	for name, m := range st.measures {
		out.Measures[name] = jsonMeasure{m.count, jsonNumber(m.min), jsonNumber(m.max), jsonNumber(m.sum), jsonNumber(m.sum / float64(m.count))}
	}
	return out
}
func jsonNumber(v float64) any {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return fmt.Sprint(v)
	}
	return v
}

var jsonFileMu sync.Mutex

func appendJSONLine(path string, value any) error {
	line, err := json.Marshal(value)
	if err != nil {
		return err
	}
	line = append(line, '\n')
	jsonFileMu.Lock()
	defer jsonFileMu.Unlock()
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	n, err := f.Write(line)
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if n != len(line) {
		return io.ErrShortWrite
	}
	return closeErr
}
