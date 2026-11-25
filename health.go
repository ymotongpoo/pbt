// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package pbt

import (
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// HealthCheck identifies a diagnostic about a property's generation or replay.
// Most checks warn by default; HealthStale requires WarnHealth or CheckHealth.
// CheckHealth upgrades selected warnings to errors and IgnoreHealth silences
// them. None changes the distribution of generated values.
type HealthCheck uint8

const (
	// HealthFilter diagnoses excessive case discards or rejected Filter values.
	HealthFilter HealthCheck = iota
	// HealthLarge diagnoses cases that consume very many random choices.
	HealthLarge
	// HealthSlow diagnoses time spent generating values, excluding property work.
	// It is never upgraded to an error unless explicitly named in CheckHealth.
	HealthSlow
	// HealthFlaky diagnoses a failure that does not reproduce with the same choices
	// and failure identity. Shrinking and saving still proceed.
	HealthFlaky
	// HealthShrink reports shrinking stopped by its time or step budget.
	HealthShrink
	// HealthStale diagnoses saved draw comments that differ from replay. Enable
	// it with WarnHealth(HealthStale) or CheckHealth(HealthStale); it is excluded
	// from CheckHealth's default set because generators can change intentionally.
	// It compares saved draw text, whose fixed 2000-byte value
	// prefix is independent of ReportLimit. Changes hidden by truncation may go
	// undetected. Draws containing pointers, functions, channels, custom
	// formatters, NaN map keys or containers nested 100 levels deep are skipped.
	HealthStale
)

// String returns the Go constant name, for use in diagnostics and options.
func (h HealthCheck) String() string {
	switch h {
	case HealthFilter:
		return "HealthFilter"
	case HealthLarge:
		return "HealthLarge"
	case HealthSlow:
		return "HealthSlow"
	case HealthFlaky:
		return "HealthFlaky"
	case HealthShrink:
		return "HealthShrink"
	case HealthStale:
		return "HealthStale"
	default:
		return fmt.Sprintf("HealthCheck(%d)", uint8(h))
	}
}

// CheckHealth makes named health warnings fail a passing Check run. With no
// arguments it selects HealthFilter and HealthLarge. Timing is noisy, so
// HealthSlow must be explicitly named. HealthFlaky and HealthShrink only apply
// to runs that already fail. IgnoreHealth takes precedence. CheckExists,
// Gen.Example and SampleShrink do not perform health checks. The flag
// -pbt.checkhealth names checks for every Check of a go test run, such as
// -pbt.checkhealth=filter,slow or -pbt.checkhealth=default; the flag and
// the options add up. HealthStale must be explicitly named.
func CheckHealth(checks ...HealthCheck) Option {
	checks = append([]HealthCheck(nil), checks...)
	if len(checks) == 0 {
		checks = []HealthCheck{HealthFilter, HealthLarge}
	}
	return func(c *config) {
		if c.checkHealth == nil {
			c.checkHealth = map[HealthCheck]bool{}
		}
		for _, h := range checks {
			c.checkHealth[h] = true
		}
	}
}

// WarnHealth enables the named optional health diagnostics as warnings.
// HealthStale is off until named here or in CheckHealth, preserving the output
// of existing tests. Other health checks already warn by default. With no
// arguments this changes nothing. CheckHealth upgrades a warning to an error;
// IgnoreHealth silences it regardless of option order. The flag -pbt.warnhealth
// accepts the same names, such as -pbt.warnhealth=stale; flag and option add up.
func WarnHealth(checks ...HealthCheck) Option {
	checks = append([]HealthCheck(nil), checks...)
	return func(c *config) {
		if c.warnHealth == nil {
			c.warnHealth = map[HealthCheck]bool{}
		}
		for _, h := range checks {
			c.warnHealth[h] = true
		}
	}
}

// healthEnabled selects optional diagnostics without changing existing ones.
func (c config) healthEnabled(h HealthCheck) bool {
	return !c.ignoreHealth[h] && (c.warnHealth[h] || c.checkHealth[h])
}

// parseHealthChecks reads -pbt.warnhealth, -pbt.checkhealth or -pbt.ignorehealth:
// names separated by commas, such as "filter,slow", in any letter case and
// with or without the Health prefix. With withDefault, "default" stands for
// the checks of CheckHealth without arguments.
func parseHealthChecks(s string, withDefault bool) ([]HealthCheck, error) {
	var checks []HealthCheck
	for _, name := range strings.Split(s, ",") {
		name = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(name)), "health")
		switch name {
		case "filter":
			checks = append(checks, HealthFilter)
		case "large":
			checks = append(checks, HealthLarge)
		case "slow":
			checks = append(checks, HealthSlow)
		case "flaky":
			checks = append(checks, HealthFlaky)
		case "shrink":
			checks = append(checks, HealthShrink)
		case "stale":
			checks = append(checks, HealthStale)
		case "default":
			if !withDefault {
				return nil, fmt.Errorf("unknown health check %q", name)
			}
			checks = append(checks, HealthFilter, HealthLarge)
		default:
			return nil, fmt.Errorf("unknown health check %q; use filter, large, slow, flaky, shrink or stale", name)
		}
	}
	return checks, nil
}

// IgnoreHealth silences the named health diagnostics, including report notes.
// It takes precedence over CheckHealth regardless of option order. With no
// arguments it changes nothing. The stability replay still runs. The flag
// -pbt.ignorehealth names checks to silence for every Check of a go test
// run, such as -pbt.ignorehealth=slow; the flag and the options add up.
func IgnoreHealth(checks ...HealthCheck) Option {
	checks = append([]HealthCheck(nil), checks...)
	return func(c *config) {
		if c.ignoreHealth == nil {
			c.ignoreHealth = map[HealthCheck]bool{}
		}
		for _, h := range checks {
			c.ignoreHealth[h] = true
		}
	}
}

const (
	// Twenty discards avoid diagnosing small runs from a handful of samples.
	healthMinDiscards = 20
	// At least half the attempts discarded means generation wastes most work.
	healthDiscardFactor = 2
	// A hundred trials and 90% rejection distinguish severe filters from useful
	// constraints (such as even numbers) without flagging small samples.
	healthMinFilterDraws = 100
	healthFilterPercent  = 90
	// Ten thousand choices is well beyond ordinary small structures. Require
	// ten cases and at least 20% large cases so an isolated edge case stays quiet.
	healthLargeChoices = 10_000
	healthMinCases     = 10
	healthLargePercent = 20
	// Both a 10ms average and a one-second total are needed: scheduling noise
	// or one expensive startup should not diagnose an otherwise cheap run.
	healthSlowAverage = 10 * time.Millisecond
	healthSlowTotal   = time.Second
	// Three sources make the diagnostic actionable without flooding the report.
	healthTopSources = 3
)

type filterHealth struct{ drawn, rejected int }
type healthDraw struct {
	label, gen string
	pc         uintptr
}
type healthStats struct {
	cases, discards, large int
	generation             time.Duration
	sources                map[string]int
	filters                map[string]filterHealth
	draws                  map[healthDraw]int
}
type healthNote struct {
	check HealthCheck
	text  string
}

func newHealthStats() *healthStats {
	return &healthStats{sources: map[string]int{}, filters: map[string]filterHealth{}, draws: map[healthDraw]int{}}
}

func (h *healthStats) filter(name string, rejected bool) {
	if h == nil {
		return
	}
	f := h.filters[name]
	f.drawn++
	if rejected {
		f.rejected++
	}
	h.filters[name] = f
}

func (h *healthStats) add(t *T) {
	h.cases++
	if t.discarded {
		h.discards++
		source := t.discardSource
		if source == "" {
			source = t.discardReason
		}
		h.sources[source]++
	}
	if len(t.choices) > healthLargeChoices {
		h.large++
		for _, d := range t.draws {
			if d.depth == 0 {
				h.draws[healthDraw{label: d.label, gen: d.gen, pc: d.pc}] += d.choices
			}
		}
	}
}

func topHealthSources(m map[string]int, unit string) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if m[keys[i]] != m[keys[j]] {
			return m[keys[i]] > m[keys[j]]
		}
		return keys[i] < keys[j]
	})
	parts := make([]string, 0, healthTopSources)
	for _, k := range keys[:min(len(keys), healthTopSources)] {
		parts = append(parts, fmt.Sprintf("%s (%d%s)", k, m[k], unit))
	}
	return strings.Join(parts, "; ")
}

func (r *runner) healthNotes() []healthNote {
	notes := append([]healthNote(nil), r.healthExtra...)
	h := r.health
	if h != nil {
		var filter []string
		if h.discards >= healthMinDiscards && h.discards*healthDiscardFactor >= h.cases {
			filter = append(filter, fmt.Sprintf("%d of %d cases were discarded; most by %s", h.discards, h.cases, topHealthSources(h.sources, "")))
		}
		var rejected []string
		for name, f := range h.filters {
			if f.drawn >= healthMinFilterDraws && f.rejected*100 >= f.drawn*healthFilterPercent {
				rejected = append(rejected, fmt.Sprintf("%s rejected %d of %d values", name, f.rejected, f.drawn))
			}
		}
		if len(rejected) > 0 {
			sort.Strings(rejected)
			filter = append(filter, rejected...)
		}
		if len(filter) > 0 {
			notes = append(notes, healthNote{HealthFilter, strings.Join(filter, "; ")})
		}
		if h.cases >= healthMinCases && h.large*100 >= h.cases*healthLargePercent {
			draws := map[string]int{}
			for d, n := range h.draws {
				name := drawName(drawRecord{label: d.label, gen: d.gen, pc: d.pc})
				if name == "" {
					name = d.gen
					if site := shortSite(d.pc); site != "" {
						name += " at " + site
					}
				}
				draws[name] += n
			}
			notes = append(notes, healthNote{HealthLarge, fmt.Sprintf("%d of %d cases exceeded %d choices; largest draws: %s", h.large, h.cases, healthLargeChoices, topHealthSources(draws, " choices"))})
		}
		if h.cases > 0 && h.generation >= healthSlowTotal && h.generation/time.Duration(h.cases) >= healthSlowAverage {
			notes = append(notes, healthNote{HealthSlow, fmt.Sprintf("generation took %s total, %s per case (property work excluded)", h.generation.Round(time.Millisecond), (h.generation / time.Duration(h.cases)).Round(time.Millisecond))})
		}
	}
	out := notes[:0]
	for _, n := range notes {
		if !r.cfg.ignoreHealth[n.check] {
			out = append(out, n)
		}
	}
	return out
}

// healthLine formats a note. The hint names CheckHealth only where it
// would change the outcome: on a warning of a passing run.
func healthLine(n healthNote, warning bool) string {
	if warning {
		return fmt.Sprintf("health: %s: %s (CheckHealth(%s) makes this fail; IgnoreHealth(%s) hides it)", n.check, n.text, n.check, n.check)
	}
	return fmt.Sprintf("health: %s: %s (IgnoreHealth(%s) hides it)", n.check, n.text, n.check)
}

func (r *runner) reportHealth() {
	for _, n := range r.healthNotes() {
		if r.cfg.checkHealth[n.check] {
			r.tb.Errorf("[pbt] %s", healthLine(n, false))
		} else {
			r.tb.Logf("[pbt] %s", healthLine(n, true))
		}
	}
}

func (r *runner) stableFailure(t *T) *T {
	replay := r.run(t.choices, t.size, nil)
	if replay.failed && !replay.discarded && replay.failKey == t.failKey {
		return replay
	}
	outcome := "passed"
	if replay.discarded {
		outcome = "discarded: " + replay.discardReason
	} else if replay.failed {
		outcome = "failed elsewhere: " + strings.TrimPrefix(failureIdentity(replay.failKey), "was ")
	}
	r.healthExtra = append(r.healthExtra, healthNote{HealthFlaky, "the failure did not recur when its input was replayed (the replay " + outcome + "); the original failure " + failureIdentity(t.failKey)})
	return t
}

// Failure keys also include descriptions, such as "no interleaving".
func failureIdentity(key string) string {
	if name, ok := strings.CutPrefix(key, signaturePrefix); ok {
		return fmt.Sprintf("was named %q", name)
	}
	if i := strings.LastIndexByte(key, ':'); i >= 0 {
		if _, err := strconv.Atoi(key[i+1:]); err == nil {
			return "was at " + filepath.Base(key)
		}
	}
	return fmt.Sprintf("was %q", key)
}
