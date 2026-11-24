// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package snippets

import (
	"testing"

	"go.ymotongpoo.com/pbt"
)

// snippet: maximize
// A random walk rarely ends far to the right of the origin. Maximize turns
// the run into a search for walks that do, and the property fails once one
// gets past 50; without the search, 100 random cases would almost never
// fail.
var walkProperty = func(t *pbt.T) {
	steps := pbt.SliceOf(pbt.SampledFrom([]int{-1, 1})).Draw(t, "steps")
	pos := 0
	for _, s := range steps {
		pos += s
	}
	t.Maximize("position", float64(pos))
	if pos > 50 {
		t.Fatalf("the walk reached %d", pos)
	}
}

// snippet: end

func TestMaximizeFindsAFarWalk(t *testing.T) {
	report := failureReport(t, "TestWalk", walkProperty, pbt.Seed(1), pbt.SearchSteps(3000))
	mustContain(t, report, "utility position = 51", "the walk reached 51")
}

// snippet: exists
// CheckExists succeeds once a case passes: here, a Pythagorean triple. The
// witness is shrunk and logged. The seed keeps the run reproducible: a few
// seeds need more than 5000 cases to meet a triple.
func TestATripleExists(t *testing.T) {
	pbt.CheckExists(t, func(t *pbt.T) {
		a := pbt.IntRange(1, 50).Draw(t, "a")
		b := pbt.IntRange(1, 50).Draw(t, "b")
		c := pbt.IntRange(1, 70).Draw(t, "c")
		if a*a+b*b != c*c {
			t.Fail()
		}
	}, pbt.Checks(5000), pbt.Seed(1))
}

// snippet: end

// snippet: objectives
// simulate runs jobs of the given durations on a pool of workers and
// returns the longest queue and the longest wait.
func simulate(jobs []int, workers int) (longestQueue, longestWait int) {
	free := make([]int, workers) // the time at which each worker is free
	for i, d := range jobs {
		w := 0
		for k := range free {
			if free[k] < free[w] {
				w = k
			}
		}
		start := max(free[w], i) // job i arrives at time i
		longestWait = max(longestWait, start-i)
		waiting := 0
		for _, f := range free {
			if f > i {
				waiting++
			}
		}
		longestQueue = max(longestQueue, waiting)
		free[w] = start + d
	}
	return longestQueue, longestWait
}

var poolProperty = func(t *pbt.T) {
	jobs := pbt.SliceOf(pbt.IntRange(1, 10)).Draw(t, "jobs")
	workers := pbt.IntRange(1, 4).Draw(t, "workers")
	queue, wait := simulate(jobs, workers)
	t.Maximize("queue", float64(queue))
	t.Maximize("wait", float64(wait))
	if wait > 100 {
		t.Fatalf("a job waited %d", wait)
	}
}

// snippet: end

func TestPoolObjectives(t *testing.T) {
	report := failureReport(t, "TestPool", poolProperty, pbt.Seed(1))
	mustContain(t, report, "utility queue = 1", "utility wait = 101", "a job waited 101")
}
