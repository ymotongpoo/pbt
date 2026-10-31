// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package pbt

import (
	"fmt"
	"maps"
	"runtime"
	"strings"
	"sync"
)

const (
	defaultBranches     = 2
	defaultMaxBranchLen = 6
	maxParallelCommands = 12
	maxParallelBranches = 3
)

// ParallelProgram is a command sequence for a parallel test: a sequential
// prefix followed by branches that run concurrently, as PropEr's
// parallel_commands/1 produces.
type ParallelProgram struct {
	model    model
	prefix   []Step
	branches [][]Step
}

// Prefix returns the commands that run before the branches.
func (p ParallelProgram) Prefix() []Step { return p.prefix }

// Branches returns the command sequences that run concurrently.
func (p ParallelProgram) Branches() [][]Step { return p.branches }

// String lists the prefix and the branches.
func (p ParallelProgram) String() string {
	var b strings.Builder
	b.WriteString("ParallelProgram{")
	writeSteps := func(title string, steps []Step) {
		fmt.Fprintf(&b, "\n  %s:", title)
		for _, s := range steps {
			fmt.Fprintf(&b, "\n    %v = %s", s.Result, s.Command)
		}
	}
	writeSteps("prefix", p.prefix)
	for i, br := range p.branches {
		writeSteps(fmt.Sprintf("branch %d", i+1), br)
	}
	b.WriteString("\n}")
	return b.String()
}

// GoString makes %#v print the command lists.
func (p ParallelProgram) GoString() string { return p.String() }

// ParallelCommands generates parallel programs: a prefix generated like
// Commands, followed by branches generated from the state the prefix leads
// to, two of at most six commands each unless Branches says otherwise.
// Commands are dropped from the end of the longest branch until every
// interleaving of the branches satisfies all preconditions, so that
// whatever order the branches execute in is a valid test. init and gen are
// those of Commands.
func ParallelCommands(init func(), gen func() Gen[Command], opts ...StatefulOption) Gen[ParallelProgram] {
	cfg := statefulConfig(opts)
	checkLengths(cfg.minLen, cfg.maxLen)
	if cfg.branches < 1 || cfg.branches > maxParallelBranches || cfg.maxBranchLen < 1 || cfg.branches*cfg.maxBranchLen > maxParallelCommands {
		panic(fmt.Sprintf("pbt: Branches(%d, %d): need 1 to %d branches and at most %d commands in total",
			cfg.branches, cfg.maxBranchLen, maxParallelBranches, maxParallelCommands))
	}
	return Gen[ParallelProgram]{name: "ParallelCommands", draw: func(t *T) ParallelProgram {
		m := model{init: init, show: cfg.show}
		m.reset()
		t.beginSpan("prefix")
		prefix := drawSteps(t, gen, 1, cfg.minLen, cfg.maxLen)
		t.endSpan()
		nextID := len(prefix) + 1
		brs := make([][]Step, cfg.branches)
		for i := range brs {
			m.replay(prefix)
			t.beginSpan("branch")
			brs[i] = drawSteps(t, gen, nextID, 0, cfg.maxBranchLen)
			t.endSpan()
			nextID += len(brs[i])
		}
		for !m.allInterleavingsValid(prefix, brs) {
			longest := 0
			for i := range brs {
				if len(brs[i]) > len(brs[longest]) {
					longest = i
				}
			}
			brs[longest] = brs[longest][:len(brs[longest])-1]
		}
		return ParallelProgram{model: m, prefix: prefix, branches: brs}
	}}
}

// allInterleavingsValid reports whether every interleaving of the branches
// satisfies the preconditions of its commands when it starts from the
// state the prefix leads to. The model is replayed up to each point.
func (m model) allInterleavingsValid(prefix []Step, branches [][]Step) bool {
	pos := make([]int, len(branches))
	var path []Step
	var valid func() bool
	valid = func() bool {
		for b := range branches {
			if pos[b] >= len(branches[b]) {
				continue
			}
			s := branches[b][pos[b]]
			m.replay(prefix, path)
			if !s.Command.pre() {
				return false
			}
			pos[b]++
			path = append(path, s)
			ok := valid()
			path = path[:len(path)-1]
			pos[b]--
			if !ok {
				return false
			}
		}
		return true
	}
	return valid()
}

// RunParallel puts the model in its initial state, executes the prefix
// sequentially and then the branches in one goroutine each, and checks
// that some interleaving of the branches explains the results, as PropEr's
// run_parallel_commands/2 does. The check keeps the order within each
// branch and ignores the real-time order between branches, so it is weaker
// than a linearizability check: a read that started after a write
// completed may still be explained by placing it before the write. A
// failure is a sign of a race in the system under test; running the
// property under the race detector helps to locate it. Because the
// schedule is not under the test's control, a race may need many runs to
// show, and shrinking keeps only the candidates on which it shows again.
//
// Each branch resolves the Vars of the prefix and of its own earlier
// commands; a command cannot refer to the result of another branch. The
// Run functions of the branches execute concurrently and must not touch
// the model; Pre, Post and Next run on the test's goroutine afterwards.
func (p ParallelProgram) RunParallel(t *T) {
	env := Env{vals: map[Var]any{}}
	p.model.reset()
	runSteps(t, p.model, p.prefix, env, nil)
	var prefixHistory []string
	for i, s := range p.prefix {
		prefixHistory = append(prefixHistory, historyLine(i+1, s, env.vals[s.Result]))
	}
	for _, br := range p.branches {
		for _, s := range br {
			t.Aggregate(s.Command.Name)
		}
	}

	results := make([][]any, len(p.branches))
	panics := make([][]any, len(p.branches))
	stacks := make([][]byte, len(p.branches))
	var wg sync.WaitGroup
	for b, br := range p.branches {
		results[b] = make([]any, len(br))
		panics[b] = make([]any, len(br))
		local := Env{vals: maps.Clone(env.vals)}
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i, s := range br {
				runtime.Gosched()
				var stack []byte
				results[b][i], panics[b][i], stack = executeCommand(s.Command, local)
				if panics[b][i] != nil {
					stacks[b] = stack
					return
				}
				local.vals[s.Result] = results[b][i]
			}
		}()
	}
	wg.Wait()

	for b, br := range p.branches {
		for i, s := range br {
			if panics[b][i] != nil {
				break
			}
			env.vals[s.Result] = results[b][i]
		}
	}
	for b := range p.branches {
		for _, rec := range panics[b] {
			if d, isDiscard := rec.(discardTest); isDiscard {
				panic(d)
			}
		}
	}
	logHistory := func() {
		t.Logf("history:")
		for _, h := range prefixHistory {
			t.Logf("  %s", h)
		}
		for b, br := range p.branches {
			t.Logf("  branch %d:", b+1)
			for i, s := range br {
				if panics[b][i] != nil {
					t.Logf("    %s panicked: %v", s.Command, panics[b][i])
					break
				}
				t.Logf("    %s", historyLine(i+1, s, results[b][i]))
			}
		}
	}
	for b := range p.branches {
		for i, s := range p.branches[b] {
			if panics[b][i] != nil {
				if t.stack == nil {
					t.stack = stacks[b]
				}
				logHistory()
				t.fatalAt(panicKey(panics[b][i], stacks[b]), fmt.Sprintf("branch %d: %s panicked: %v", b+1, s.Command, panics[b][i]))
			}
		}
	}
	if !p.explained(results, env) {
		logHistory()
		p.model.replay(p.prefix)
		if state, ok := p.model.state(); ok {
			t.Logf("model state after the prefix: %s", state)
		}
		t.fatalAt("no interleaving", fmt.Sprintf("no interleaving of the %d branches satisfies the postconditions", len(p.branches)))
	}
}

// explained reports whether some interleaving of the branches, applied to
// the model from the state after the prefix, accepts every recorded
// result. Only the order within each branch constrains the interleavings.
// The model is replayed up to each point.
func (p ParallelProgram) explained(results [][]any, env Env) bool {
	pos := make([]int, len(p.branches))
	var path []Step
	var search func() bool
	search = func() bool {
		done := true
		for b := range p.branches {
			if pos[b] >= len(p.branches[b]) {
				continue
			}
			done = false
			s := p.branches[b][pos[b]]
			p.model.replay(p.prefix, path)
			if !s.Command.pre() || s.Command.post(env, results[b][pos[b]]) != nil {
				continue
			}
			pos[b]++
			path = append(path, s)
			ok := search()
			path = path[:len(path)-1]
			pos[b]--
			if ok {
				return true
			}
		}
		return done
	}
	return search()
}
