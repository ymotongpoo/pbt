// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package pbt

import (
	"fmt"
	"path/filepath"
)

// collectFailure preserves discovery order and never replaces a failure being
// shrunk. Each identity gets its own shrink budget.
func (r *runner) collectFailure(t *T) {
	if r.exists || r.cfg.maxFailures <= 1 || !t.failed || t.discarded {
		return
	}
	for _, f := range r.failures {
		if f.failKey == t.failKey {
			return
		}
	}
	if len(r.failures) < r.cfg.maxFailures {
		r.failures = append(r.failures, t)
	}
}

func (r *runner) collectSavedFailure(t *T, path string) {
	before := len(r.failures)
	r.collectFailure(t)
	if len(r.failures) > before {
		if r.savedPaths == nil {
			r.savedPaths = make(map[string]string)
		}
		r.savedPaths[t.failKey] = path
	}
}

func (r *runner) checkMultiple() {
	if path := r.cfg.replayFile; path != "" {
		t, err := r.replayFile(path)
		if err != nil {
			r.outputStatus("error")
			r.tb.Errorf("[pbt] cannot replay %s: %v", path, err)
			return
		}
		r.collectSavedFailure(t, path)
		if len(r.failures) == 0 {
			r.tb.Logf("[pbt] replayed %s: the property passes on it", path)
		}
	}
	if !r.cfg.noReplay && len(r.failures) < r.cfg.maxFailures {
		r.replaySaved()
	}
	replayed := len(r.failures) > 0
	passed := 0
	if !replayed {
		_, n, err := r.search()
		passed = n
		if err != nil {
			r.tb.Errorf("[pbt] %v", err)
		}
		if len(r.failures) == 0 {
			if err != nil {
				r.reportReplayHealth()
			}
			if err == nil {
				r.reportHealth()
				r.reportStats(passed)
			}
			return
		}
	}
	type result struct {
		t        *T
		headline string
		trailer  []string
		notes    []healthNote
	}
	var results []result
	replayNotes := append([]healthNote(nil), r.healthExtra...)
	// Shrinking may discover another identity, extending this loop.
	for i := 0; i < len(r.failures); i++ {
		r.healthExtra = nil
		t := r.stableFailure(r.failures[i])
		steps := 0
		source := r.savedPaths[t.failKey]
		headline := "saved counterexample " + filepath.Base(source) + " still fails"
		if !replayed {
			t, steps = r.shrink(t)
			headline = fmt.Sprintf("property failed after %d passed checks and %d shrink steps", passed, steps)
		}
		var trailer []string
		if !replayed && !r.cfg.noSave {
			if path, err := r.save(t); err != nil {
				trailer = append(trailer, fmt.Sprintf("could not save the counterexample: %v", err))
			} else {
				trailer = append(trailer, fmt.Sprintf("saved to %s; go test replays it until the file is deleted", path))

			}
		}
		if replayed {
			trailer = append(trailer, fmt.Sprintf("reproduce: go test -run '%s' -pbt.replay='%s' -pbt.failures=%d", r.tb.Name(), source, r.cfg.maxFailures))
		} else {
			trailer = append(trailer, fmt.Sprintf("reproduce: go test -run '%s' -pbt.seed=0x%x -pbt.failures=%d", r.tb.Name(), r.cfg.seed, r.cfg.maxFailures))
		}
		results = append(results, result{t, headline, trailer, r.healthExtra})
	}
	for i, result := range results {
		r.healthExtra = result.notes
		if i == 0 {
			r.healthExtra = append(replayNotes, r.healthExtra...)
		}
		r.report(result.t, fmt.Sprintf("failure %d of %d: %s", i+1, len(results), result.headline), result.trailer)
		// Run-level diagnostics belong to the whole search, so print them once.
		r.health = nil
	}
}
