// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package pbt

import (
	"cmp"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"
)

const (
	maxShrinkSteps     = 200_000
	redistributeWindow = 16
	maxDeleteRun       = 8
)

// shrinker minimizes a failing choice sequence. Every pass proposes
// candidate sequences; a candidate is adopted when the property still fails
// in the same way (the same assertion site or the same panic site) and the
// recorded sequence is shorter, or equally long and lexicographically
// smaller. Keeping the failure identity stops shrinking from trading the
// bug it found for an unrelated one that happens to be simpler. Passes run
// until none of them makes progress or the time budget is spent.
type shrinker struct {
	r        *runner
	cur      *T
	deadline time.Time
	steps    int

	// onAccept, when set, runs after every accepted candidate; returning
	// false stops shrinking. SampleShrink uses it.
	onAccept    func() bool
	stop        bool
	exhaustedBy string

	maskFor *T
	frozen  []bool // choices inside NoShrink spans
	inside  []bool // spans nested inside a NoShrink span
}

func (r *runner) shrink(t *T) (*T, int) {
	s := &shrinker{r: r, cur: t, deadline: time.Now().Add(r.cfg.shrinkTime)}
	s.run()
	if r.health != nil && s.exhaustedBy != "" {
		hint := "raise the time budget with ShrinkTime or -pbt.shrinktime"
		if s.steps >= maxShrinkSteps {
			hint = fmt.Sprintf("the step limit is fixed at %d; ShrinkTime and -pbt.shrinktime only change the time budget", maxShrinkSteps)
		}
		r.healthExtra = append(r.healthExtra, healthNote{HealthShrink, fmt.Sprintf("shrinking stopped after %d steps: %s; %s", s.steps, s.exhaustedBy, hint)})
	}
	return s.cur, s.steps
}

func (s *shrinker) exhausted() bool {
	if s.stop {
		return true
	}
	if s.steps >= maxShrinkSteps {
		s.exhaustedBy = "step limit reached"
		return true
	}
	if !time.Now().Before(s.deadline) {
		s.exhaustedBy = "ShrinkTime budget expired"
		return true
	}
	return false
}

func (s *shrinker) try(cand []uint64) bool {
	return s.tryReplay(cand, nil)
}

func (s *shrinker) tryReplay(cand []uint64, realign *spanReplay) bool {
	if s.exhausted() {
		return false
	}
	s.steps++
	t := s.r.runReplay(cand, s.cur.size, nil, realign)
	s.r.collectFailure(t)
	if s.r.want(t) && !t.discarded && t.failKey == s.cur.failKey && simpler(t.choices, s.cur.choices) && preservesNoShrink(s.cur, t) {
		s.cur = t
		if s.r.cfg.verbose {
			s.r.tb.Logf("[pbt] shrink step %d: %d choices\n[pbt]   %s", s.steps, len(t.choices), strings.ReplaceAll(drawsLine(t), "\n", "\n[pbt]   "))
		}
		if s.onAccept != nil && !s.onAccept() {
			s.stop = true
		}
		return true
	}
	return false
}

// preservesNoShrink requires the protected draws in the candidate to be an
// unchanged subsequence of those in the previous case. Deleting a containing
// collection element is allowed; deleting its choices and regenerating the
// same draw from shifted choices or trailing zeros is not. Check after replay
// because even an edit outside a protected span can change what it consumes.
func preservesNoShrink(before, after *T) bool {
	next := 0
	for _, sp := range after.spans {
		if !sp.noshrink {
			continue
		}
		found := false
		for next < len(before.spans) {
			old := before.spans[next]
			next++
			if old.noshrink && old.label == sp.label &&
				slices.Equal(before.choices[old.start:old.end], after.choices[sp.start:sp.end]) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// drawsLine formats the draws of a case on as few lines as possible.
func drawsLine(t *T) string {
	parts := make([]string, len(t.draws))
	for i, d := range t.draws {
		parts[i] = formatDrawSite(d, defaultValueLimit, true)
	}
	return strings.Join(parts, "; ")
}

// masks computes which choices and spans NoShrink protects in the current
// counterexample.
func (s *shrinker) masks() {
	if s.maskFor == s.cur {
		return
	}
	spans := s.cur.spans
	s.frozen = make([]bool, len(s.cur.choices))
	s.inside = make([]bool, len(spans))
	for i, sp := range spans {
		if sp.noshrink {
			for j := int(sp.start); j < int(sp.end) && j < len(s.frozen); j++ {
				s.frozen[j] = true
			}
		}
		for p := sp.parent; p >= 0; p = spans[p].parent {
			if spans[p].noshrink {
				s.inside[i] = true
				break
			}
		}
	}
	s.maskFor = s.cur
}

// frozenRange reports whether any choice in [start, end) is protected.
func (s *shrinker) frozenRange(start, end int) bool {
	s.masks()
	for i := start; i < end && i < len(s.frozen); i++ {
		if s.frozen[i] {
			return true
		}
	}
	return false
}

func (s *shrinker) frozenAt(i int) bool {
	s.masks()
	return i < len(s.frozen) && s.frozen[i]
}

// insideNoShrink reports whether span i is nested inside a NoShrink span.
func (s *shrinker) insideNoShrink(i int) bool {
	s.masks()
	return s.inside[i]
}

func simpler(a, b []uint64) bool {
	if len(a) != len(b) {
		return len(a) < len(b)
	}
	return slices.Compare(a, b) < 0
}

func (s *shrinker) run() {
	passes := []func() bool{
		s.deleteSpans,
		s.deleteSpanRuns,
		s.zeroSpans,
		s.replaceWithDescendant,
		s.sortSpans,
		s.minimizeDuplicates,
		s.minimizeChoices,
		s.redistribute,
		s.switchBranches,
	}
	for !s.exhausted() {
		changed := false
		for _, pass := range passes {
			if pass() {
				changed = true
			}
			if s.exhausted() {
				return
			}
		}
		if !changed {
			return
		}
	}
}

// spansByStartDesc returns the indices of the spans of the current
// counterexample ordered so that later and larger spans come first.
// Rewriting a span then never moves a span that is still to be visited.
func (s *shrinker) spansByStartDesc() []int {
	spans := s.cur.spans
	idx := make([]int, len(spans))
	for i := range idx {
		idx[i] = i
	}
	slices.SortFunc(idx, func(a, b int) int {
		if spans[a].start != spans[b].start {
			return int(spans[b].start - spans[a].start)
		}
		return int(spans[b].end - spans[a].end)
	})
	return idx
}

func (s *shrinker) firstStartBelow(idx []int, pos int) int {
	for k, i := range idx {
		if int(s.cur.spans[i].start) < pos {
			return k
		}
	}
	return len(idx)
}

// deleteSpans removes whole spans: list elements, rejected Filter attempts,
// subtrees. Spans nested inside a NoShrink span stay.
func (s *shrinker) deleteSpans() bool {
	progress := false
	idx := s.spansByStartDesc()
	for k := 0; k < len(idx) && !s.exhausted(); k++ {
		sp := s.cur.spans[idx[k]]
		cur := s.cur.choices
		if sp.end <= sp.start || int(sp.end) > len(cur) || s.insideNoShrink(idx[k]) {
			continue
		}
		if s.try(slices.Concat(cur[:int(sp.start)], cur[int(sp.end):])) {
			progress = true
			idx = s.spansByStartDesc()
			k = s.firstStartBelow(idx, int(sp.start)) - 1
		}
	}
	return progress
}

// deleteSpanRuns removes runs of two to maxDeleteRun adjacent sibling spans
// with the same label, which is what a constraint on the length modulo
// some number needs; single deletions cannot cross it.
func (s *shrinker) deleteSpanRuns() bool {
	progress := false
restart:
	for !s.exhausted() {
		spans := s.cur.spans
		children := make(map[int][]int)
		for i, sp := range spans {
			if sp.end > sp.start && !s.insideNoShrink(i) {
				children[int(sp.parent)] = append(children[int(sp.parent)], i)
			}
		}
		for parent := -1; parent < len(spans); parent++ {
			sib := children[parent]
			for run := 2; run <= maxDeleteRun && run <= len(sib); run++ {
				for start := 0; start+run <= len(sib); start++ {
					first, last := spans[sib[start]], spans[sib[start+run-1]]
					contiguous := true
					for k := start; k < start+run-1; k++ {
						a, b := spans[sib[k]], spans[sib[k+1]]
						if a.label != b.label || a.end != b.start {
							contiguous = false
							break
						}
					}
					if !contiguous || int(last.end) > len(s.cur.choices) {
						continue
					}
					cur := s.cur.choices
					if s.try(slices.Concat(cur[:int(first.start)], cur[int(last.end):])) {
						progress = true
						continue restart
					}
					if s.exhausted() {
						return progress
					}
				}
			}
		}
		return progress
	}
	return progress
}

// zeroSpans replaces every choice of a span with 0, the simplest value.
func (s *shrinker) zeroSpans() bool {
	progress := false
	idx := s.spansByStartDesc()
	for k := 0; k < len(idx) && !s.exhausted(); k++ {
		sp := s.cur.spans[idx[k]]
		cur := s.cur.choices
		if sp.end <= sp.start || int(sp.end) > len(cur) || s.frozenRange(int(sp.start), int(sp.end)) {
			continue
		}
		if !slices.ContainsFunc(cur[sp.start:sp.end], func(v uint64) bool { return v != 0 }) {
			continue
		}
		cand := slices.Clone(cur)
		clear(cand[sp.start:sp.end])
		if s.try(cand) {
			progress = true
			idx = s.spansByStartDesc()
			k = s.firstStartBelow(idx, int(sp.start)) - 1
		}
	}
	return progress
}

// replaceWithDescendant replaces a span with a smaller span of the same
// label nested inside it, which turns a tree into one of its subtrees. It is
// the automatic form of PropEr's ?LETSHRINK.
func (s *shrinker) replaceWithDescendant() bool {
	progress := false
restart:
	for !s.exhausted() {
		spans := s.cur.spans
		for i, sp := range spans {
			size := int(sp.end - sp.start)
			if size < 2 || s.frozenRange(int(sp.start), int(sp.end)) {
				continue
			}
			var descs []span
			for j := i + 1; j < len(spans) && spans[j].start < sp.end; j++ {
				d := spans[j]
				if d.label == sp.label && d.end <= sp.end && int(d.end-d.start) < size {
					descs = append(descs, d)
				}
			}
			slices.SortFunc(descs, func(a, b span) int { return int((a.end - a.start) - (b.end - b.start)) })
			for _, d := range descs {
				cur := s.cur.choices
				if s.try(slices.Concat(cur[:int(sp.start)], cur[int(d.start):int(d.end)], cur[int(sp.end):])) {
					progress = true
					continue restart
				}
				if s.exhausted() {
					return progress
				}
			}
		}
		return progress
	}
	return progress
}

type spanGroup struct {
	label string
	size  int
}

type choiceGroup struct {
	meta  choiceMeta
	value uint64
}

// sortSpans puts sibling spans of the same label and length into
// lexicographic order, which canonicalizes lists whose elements can be
// permuted.
func (s *shrinker) sortSpans() bool {
	progress := false
restart:
	for !s.exhausted() {
		spans := s.cur.spans
		children := make(map[int]map[spanGroup][]span)
		// order keeps the groups of each parent in the order of their first
		// span, so that the shrinker tries them in the same order on every
		// run; ranging over the map would follow Go's random map order.
		order := make(map[int][]spanGroup)
		for _, sp := range spans {
			if sp.end <= sp.start {
				continue
			}
			g := spanGroup{sp.label, int(sp.end - sp.start)}
			if children[int(sp.parent)] == nil {
				children[int(sp.parent)] = make(map[spanGroup][]span)
			}
			if _, seen := children[int(sp.parent)][g]; !seen {
				order[int(sp.parent)] = append(order[int(sp.parent)], g)
			}
			children[int(sp.parent)][g] = append(children[int(sp.parent)][g], sp)
		}
		for parent := -1; parent < len(spans); parent++ {
			for _, g := range order[parent] {
				group := children[parent][g]
				if len(group) < 2 || s.frozenRange(int(group[0].start), int(group[len(group)-1].end)) {
					continue
				}
				cur := s.cur.choices
				segs := make([][]uint64, len(group))
				for i, sp := range group {
					segs[i] = cur[sp.start:sp.end]
				}
				sorted := slices.Clone(segs)
				slices.SortFunc(sorted, slices.Compare)
				if slices.EqualFunc(segs, sorted, slices.Equal) {
					continue
				}
				cand := slices.Clone(cur)
				for i, sp := range group {
					copy(cand[sp.start:sp.end], sorted[i])
				}
				if s.try(cand) {
					progress = true
					continue restart
				}
				if s.exhausted() {
					return progress
				}
			}
		}
		return progress
	}
	return progress
}

// minimizeDuplicates lowers choices that share a value together, which is
// what a counterexample needs when two values must stay equal.
func (s *shrinker) minimizeDuplicates() bool {
	progress := false
	groups := make(map[choiceGroup][]int)
	for i, v := range s.cur.choices {
		if v != 0 && s.cur.metas[i].kind != kindFlag && !s.frozenAt(i) {
			g := choiceGroup{s.cur.metas[i], v}
			groups[g] = append(groups[g], i)
		}
	}
	keys := make([]choiceGroup, 0, len(groups))
	for g, idx := range groups {
		if len(idx) >= 2 {
			keys = append(keys, g)
		}
	}
	// The order must be total: keys come from a map, so a tie would leave
	// the order of the tied groups to Go's random map order.
	slices.SortFunc(keys, func(a, b choiceGroup) int {
		if c := cmp.Compare(a.value, b.value); c != 0 {
			return c
		}
		if c := cmp.Compare(a.meta.kind, b.meta.kind); c != 0 {
			return c
		}
		if c := cmp.Compare(a.meta.zz, b.meta.zz); c != 0 {
			return c
		}
		switch {
		case a.meta.tailPos == b.meta.tailPos:
			return 0
		case !a.meta.tailPos:
			return -1
		default:
			return 1
		}
	})
	for _, g := range keys {
		if s.exhausted() {
			break
		}
		v, idx := g.value, groups[g]
		set := func(x uint64) bool {
			cand := slices.Clone(s.cur.choices)
			for _, i := range idx {
				if i >= len(cand) {
					return false
				}
				cand[i] = x
			}
			return s.try(cand)
		}
		if set(0) {
			progress = true
			continue
		}
		if s.searchDown(0, v, set) {
			progress = true
		}
	}
	return progress
}

// searchDown finds, by binary search, the smallest x in (lo, hi) for which ok
// holds, assuming ok fails at lo, holds at hi and is monotone in between.
// It reports whether any call of ok succeeded.
func (s *shrinker) searchDown(lo, hi uint64, ok func(uint64) bool) bool {
	progress := false
	for lo+1 < hi && !s.exhausted() {
		mid := lo + (hi-lo)/2
		if ok(mid) {
			hi = mid
			progress = true
		} else {
			lo = mid
		}
	}
	return progress
}

// minimizeChoices lowers each choice on its own: to 0 first, then by binary
// search. Zigzag choices shrink in magnitude first and then flip to the
// positive side.
func (s *shrinker) minimizeChoices() bool {
	progress := false
	for i := 0; i < len(s.cur.choices) && !s.exhausted(); i++ {
		if s.cur.choices[i] == 0 || s.frozenAt(i) {
			continue
		}
		set := func(x uint64) bool {
			if i >= len(s.cur.choices) {
				return false
			}
			cand := slices.Clone(s.cur.choices)
			cand[i] = x
			return s.try(cand)
		}
		if set(0) {
			progress = true
			continue
		}
		if s.cur.metas[i].kind == kindZigzag {
			if s.minimizeZigzag(i, set) {
				progress = true
			}
		} else if s.searchDown(0, s.cur.choices[i], set) {
			progress = true
		}
	}
	return progress
}

// minimizeZigzag shrinks a signed value toward its center: first the
// magnitude, keeping the sign, then a negative value flips to the positive
// value of the same magnitude, which is the smaller choice.
func (s *shrinker) minimizeZigzag(i int, set func(uint64) bool) bool {
	m := s.cur.metas[i]
	v := offsetOf(s.cur.choices[i], m)
	negative := v < 0
	signed := func(k uint64) int64 {
		if negative {
			return -int64(k)
		}
		return int64(k)
	}
	progress := s.searchDown(0, absInt64(v), func(k uint64) bool { return set(choiceOf(signed(k), m)) })
	if negative && i < len(s.cur.choices) {
		k := absInt64(offsetOf(s.cur.choices[i], m))
		if set(choiceOf(int64(k), m)) {
			progress = true
			s.searchDown(0, k, func(k uint64) bool { return set(choiceOf(int64(k), m)) })
		}
	}
	return progress
}

// redistribute moves quantity from an earlier value choice to a later one
// while keeping their sum, so that a constraint on the sum ends up on as few
// values as possible.
func (s *shrinker) redistribute() bool {
	progress := false
	for i := 0; i < len(s.cur.choices) && !s.exhausted(); i++ {
		if s.cur.metas[i].kind == kindFlag || s.cur.choices[i] == 0 || s.frozenAt(i) {
			continue
		}
		for j := i + 1; j < len(s.cur.choices) && j <= i+redistributeWindow && !s.exhausted(); j++ {
			mi, mj := s.cur.metas[i], s.cur.metas[j]
			if mi.kind != mj.kind || mj.kind == kindFlag || s.frozenAt(j) {
				continue
			}
			vi, vj := s.cur.choices[i], s.cur.choices[j]
			var amount uint64
			var move func(k uint64) bool
			if mi.kind == kindZigzag {
				ai, aj := offsetOf(vi, mi), offsetOf(vj, mj)
				if ai <= 0 {
					continue
				}
				amount = uint64(ai)
				move = func(k uint64) bool {
					return s.set2(i, choiceOf(ai-int64(k), mi), j, choiceOf(aj+int64(k), mj))
				}
			} else {
				amount = vi
				move = func(k uint64) bool {
					if vj > math.MaxUint64-k {
						return false
					}
					return s.set2(i, vi-k, j, vj+k)
				}
			}
			if amount == 0 {
				continue
			}
			if move(amount) {
				progress = true
				break
			}
			lo, hi := uint64(0), amount // lo is a no-op, hi failed
			for lo+1 < hi && !s.exhausted() {
				mid := lo + (hi-lo)/2
				if move(mid) {
					lo = mid
					progress = true
				} else {
					hi = mid
				}
			}
			if s.cur.choices[i] == 0 {
				break
			}
		}
	}
	return progress
}

func (s *shrinker) set2(i int, vi uint64, j int, vj uint64) bool {
	if j >= len(s.cur.choices) {
		return false
	}
	cand := slices.Clone(s.cur.choices)
	cand[i], cand[j] = vi, vj
	return s.try(cand)
}

// switchBranches lowers the first choice of a span, such as the index of
// a OneOf or the length a generator draws first, and reconnects the
// choices following that span when it closes. Replay supplies
// either zeros or the old leading choices within the replacement, followed
// by zeros if it grows. The usual acceptance checks still apply to the actual
// recorded sequence, including failure identity and NoShrink protection.
func (s *shrinker) switchBranches() bool {
	progress := false
	for k := len(s.cur.spans) - 1; k >= 0 && !s.exhausted(); k-- {
		sp := s.cur.spans[k]
		i, end := int(sp.start), int(sp.end)
		if end <= i+1 || end >= len(s.cur.choices) || s.cur.choices[i] == 0 || s.frozenAt(i) || s.insideNoShrink(k) {
			continue
		}
		accepted := false
		for _, leading := range []bool{false, true} {
			set := func(v uint64) bool {
				cand := slices.Clone(s.cur.choices[:i+1])
				cand[i] = v
				if leading {
					cand = append(cand, s.cur.choices[i+1:end]...)
				}
				return s.tryReplay(cand, &spanReplay{seq: sp.seq, tail: s.cur.choices[end:]})
			}
			old := s.cur.choices[i]
			accepted = set(0)
			for v := old / 2; v > 0 && !accepted && !s.exhausted(); v /= 2 {
				accepted = set(v)
			}
			if accepted {
				break
			}
		}
		if accepted {
			progress = true
			// Span indices and lengths may have changed; restart on the new trace.
			k = len(s.cur.spans)
		}
	}
	return progress
}
