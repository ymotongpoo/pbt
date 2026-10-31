package pbt

import (
	"fmt"
	"math"
	"slices"
	"sort"
	"strings"
)

// stats accumulates the statistics of the passing test cases of one run.
type stats struct {
	cases    int
	labels   map[string]int
	samples  map[string]int
	nsamples int
	measures map[string]*summary
	covers   map[string]*coverage
}

type summary struct {
	count         int
	min, max, sum float64
}

type coverage struct {
	pct float64
	hit int
}

func newStats() *stats {
	return &stats{labels: map[string]int{}, samples: map[string]int{}, measures: map[string]*summary{}, covers: map[string]*coverage{}}
}

func (s *stats) add(t *T) {
	s.cases++
	for _, l := range t.labels {
		s.labels[l]++
	}
	for _, l := range t.samples {
		s.samples[l]++
		s.nsamples++
	}
	for _, m := range t.measures {
		sm := s.measures[m.name]
		if sm == nil {
			sm = &summary{min: math.Inf(1), max: math.Inf(-1)}
			s.measures[m.name] = sm
		}
		sm.count++
		sm.sum += m.value
		sm.min = min(sm.min, m.value)
		sm.max = max(sm.max, m.value)
	}
	for _, c := range t.covers {
		cv := s.covers[c.label]
		if cv == nil {
			cv = &coverage{}
			s.covers[c.label] = cv
		}
		cv.pct = c.pct
		if c.hit {
			cv.hit++
		}
	}
}

func (s *stats) empty() bool {
	return len(s.labels) == 0 && len(s.samples) == 0 && len(s.measures) == 0 && len(s.covers) == 0
}

// lines formats the statistics the way PropEr prints them: one line per
// label with its share of the cases, one line per aggregated sample with its
// share of all samples, then one line per measurement.
func (s *stats) lines() []string {
	out := shareLines(s.labels, s.cases)
	out = append(out, shareLines(s.samples, s.nsamples)...)
	names := make([]string, 0, len(s.measures))
	for n := range s.measures {
		names = append(names, n)
	}
	slices.Sort(names)
	for _, n := range names {
		m := s.measures[n]
		out = append(out, fmt.Sprintf("%s: min %s, avg %s, max %s (%d samples)",
			n, formatNum(m.min), formatNum(m.sum/float64(m.count)), formatNum(m.max), m.count))
	}
	return out
}

// shortfalls lists the Cover requirements that the run did not meet. A
// requirement is unmet only when the 99.7% Wilson upper bound of the observed
// share lies below the required share, so that a run with few cases does not
// fail by chance.
func (s *stats) shortfalls() []string {
	var out []string
	labels := make([]string, 0, len(s.covers))
	for l := range s.covers {
		labels = append(labels, l)
	}
	slices.Sort(labels)
	for _, l := range labels {
		c := s.covers[l]
		observed := 100 * float64(c.hit) / float64(s.cases)
		if wilsonUpper(c.hit, s.cases, 3) < c.pct/100 {
			out = append(out, fmt.Sprintf("%s: %.1f%% of %d cases, required %.1f%%", l, observed, s.cases, c.pct))
		}
	}
	return out
}

func shareLines(counts map[string]int, total int) []string {
	labels := make([]string, 0, len(counts))
	for l := range counts {
		labels = append(labels, l)
	}
	sort.Slice(labels, func(i, j int) bool {
		if counts[labels[i]] != counts[labels[j]] {
			return counts[labels[i]] > counts[labels[j]]
		}
		return labels[i] < labels[j]
	})
	out := make([]string, 0, len(labels))
	for _, l := range labels {
		out = append(out, fmt.Sprintf("%5.1f%% %s", 100*float64(counts[l])/float64(total), l))
	}
	return out
}

func wilsonUpper(hit, n int, z float64) float64 {
	if n == 0 {
		return 1
	}
	p := float64(hit) / float64(n)
	nf := float64(n)
	denom := 1 + z*z/nf
	center := p + z*z/(2*nf)
	spread := z * math.Sqrt(p*(1-p)/nf+z*z/(4*nf*nf))
	return (center + spread) / denom
}

func formatNum(f float64) string {
	if f == math.Trunc(f) && math.Abs(f) < 1e15 {
		return fmt.Sprintf("%d", int64(f))
	}
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.3f", f), "0"), ".")
}
