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
type ParallelProgram[S, Sys any] struct {
	init     S
	prefix   []Step[S, Sys]
	branches [][]Step[S, Sys]
}

// Prefix returns the commands that run before the branches.
func (p ParallelProgram[S, Sys]) Prefix() []Step[S, Sys] { return p.prefix }

// Branches returns the command sequences that run concurrently.
func (p ParallelProgram[S, Sys]) Branches() [][]Step[S, Sys] { return p.branches }

// String lists the prefix and the branches.
func (p ParallelProgram[S, Sys]) String() string {
	var b strings.Builder
	b.WriteString("ParallelProgram{")
	writeSteps := func(title string, steps []Step[S, Sys]) {
		fmt.Fprintf(&b, "\n  %s:", title)
		for _, s := range steps {
			fmt.Fprintf(&b, "\n    %v = %s", s.Result, describe(s.Command))
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
func (p ParallelProgram[S, Sys]) GoString() string { return p.String() }

// ParallelCommands generates parallel programs with two branches of at most
// six commands each.
func ParallelCommands[S, Sys any](init S, gen func(state S) Gen[Command[S, Sys]]) Gen[ParallelProgram[S, Sys]] {
	return named(ParallelCommandsN(init, gen, defaultBranches, defaultMaxBranchLen), "ParallelCommands")
}

// ParallelCommandsN generates parallel programs with the given number of
// branches, each at most maxBranchLen commands long. The prefix is generated
// like Commands; every branch is generated from the state the prefix leads
// to. Commands are dropped from the end of the longest branch until every
// interleaving of the branches satisfies all preconditions, so that whatever
// order the branches execute in is a valid test. At most three branches and
// twelve commands in total are allowed, which bounds the number of
// interleavings that generation and checking enumerate.
func ParallelCommandsN[S, Sys any](init S, gen func(state S) Gen[Command[S, Sys]], branches, maxBranchLen int) Gen[ParallelProgram[S, Sys]] {
	if branches < 1 || branches > maxParallelBranches || maxBranchLen < 1 || branches*maxBranchLen > maxParallelCommands {
		panic(fmt.Sprintf("pbt: ParallelCommandsN(%d branches, %d commands each): need 1 to %d branches and at most %d commands in total",
			branches, maxBranchLen, maxParallelBranches, maxParallelCommands))
	}
	return Gen[ParallelProgram[S, Sys]]{name: fmt.Sprintf("ParallelCommandsN(%d, %d)", branches, maxBranchLen), draw: func(t *T) ParallelProgram[S, Sys] {
		t.beginSpan("prefix")
		prefix, state := drawSteps(t, gen, init, 1, 0, -1)
		t.endSpan()
		nextID := len(prefix) + 1
		brs := make([][]Step[S, Sys], branches)
		for i := range brs {
			t.beginSpan("branch")
			brs[i], _ = drawSteps(t, gen, state, nextID, 0, maxBranchLen)
			t.endSpan()
			nextID += len(brs[i])
		}
		for !allInterleavingsValid(state, brs, make([]int, len(brs))) {
			longest := 0
			for i := range brs {
				if len(brs[i]) > len(brs[longest]) {
					longest = i
				}
			}
			brs[longest] = brs[longest][:len(brs[longest])-1]
		}
		return ParallelProgram[S, Sys]{init: init, prefix: prefix, branches: brs}
	}}
}

// allInterleavingsValid reports whether every interleaving of the branches
// satisfies the preconditions of its commands when it starts from state.
func allInterleavingsValid[S, Sys any](state S, branches [][]Step[S, Sys], pos []int) bool {
	for b := range branches {
		if pos[b] >= len(branches[b]) {
			continue
		}
		s := branches[b][pos[b]]
		if !s.Command.Precondition(state) {
			return false
		}
		pos[b]++
		ok := allInterleavingsValid(s.Command.NextState(state, s.Result), branches, pos)
		pos[b]--
		if !ok {
			return false
		}
	}
	return true
}

// RunParallel executes the prefix sequentially and then the branches in one
// goroutine each, and checks that some interleaving of the branches explains
// the results, as PropEr's run_parallel_commands/2 does. The check keeps the
// order within each branch and ignores the real-time order between
// branches, so it is weaker than a linearizability check: a read that
// started after a write completed may still be explained by placing it
// before the write. A failure is a sign of a race in the system under test;
// running the property under the race detector helps to locate it. Because
// the schedule is not under the test's control, a race may need many runs
// to show, and shrinking keeps only the candidates on which it shows again.
//
// Each branch resolves the Vars of the prefix and of its own earlier
// commands; a command cannot refer to the result of another branch.
func (p ParallelProgram[S, Sys]) RunParallel(t *T, sys Sys) {
	env := Env{vals: map[Var]any{}}
	state := runSteps(t, p.prefix, p.init, sys, env, nil)
	var prefixHistory []string
	for i, s := range p.prefix {
		prefixHistory = append(prefixHistory, historyLine(i+1, s, env.vals[s.Result]))
	}
	for _, br := range p.branches {
		for _, s := range br {
			t.Aggregate(CommandName(s.Command))
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
				results[b][i], panics[b][i], stack = executeCommand(s.Command, sys, local)
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
					t.Logf("    %s panicked: %v", describe(s.Command), panics[b][i])
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
				t.fatalAt(panicKey(panics[b][i], stacks[b]), fmt.Sprintf("branch %d: %s panicked: %v", b+1, describe(s.Command), panics[b][i]))
			}
		}
	}
	if !linearizable(state, p.branches, results, make([]int, len(p.branches)), env) {
		logHistory()
		t.Logf("model state after the prefix: %+v", state)
		t.fatalAt("no interleaving", fmt.Sprintf("no interleaving of the %d branches satisfies the postconditions", len(p.branches)))
	}
}

// explained reports whether some interleaving of the branches, applied to
// the model from state, accepts every recorded result. Only the order within
// each branch constrains the interleavings.
func linearizable[S, Sys any](state S, branches [][]Step[S, Sys], results [][]any, pos []int, env Env) bool {
	done := true
	for b := range branches {
		if pos[b] >= len(branches[b]) {
			continue
		}
		done = false
		s := branches[b][pos[b]]
		if !s.Command.Precondition(state) || s.Command.Postcondition(state, env, results[b][pos[b]]) != nil {
			continue
		}
		pos[b]++
		ok := linearizable(s.Command.NextState(state, s.Result), branches, results, pos, env)
		pos[b]--
		if ok {
			return true
		}
	}
	return done
}
