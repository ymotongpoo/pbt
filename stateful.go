// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package pbt

import (
	"fmt"
	"reflect"
	"runtime/debug"
	"strings"
)

// Var names the result of a command before the command has run. While a
// command sequence is generated, no command has executed, so the model can
// refer to a result only through its Var; during execution, Env resolves
// the Var to the real value. This is PropEr's {var, N}.
type Var struct{ id int }

// String returns the symbolic name, such as var3.
func (v Var) String() string {
	if v.id == 0 {
		return "var?"
	}
	return fmt.Sprintf("var%d", v.id)
}

// GoString makes %#v print the symbolic name.
func (v Var) GoString() string { return v.String() }

// Env holds the results of the commands that have executed so far.
type Env struct{ vals map[Var]any }

// Get returns the value of v. It panics when the command that produces v has
// not run, which is a mistake in the command sequence rather than in the
// system under test.
func (e Env) Get(v Var) any {
	val, ok := e.vals[v]
	if !ok {
		panic(fmt.Sprintf("pbt: %v has no value yet", v))
	}
	return val
}

// Has reports whether v has a value.
func (e Env) Has(v Var) bool {
	_, ok := e.vals[v]
	return ok
}

// Resolve returns the value of v as a T. A nil result resolves to the zero
// value of T when T is an interface or pointer type.
func Resolve[T any](env Env, v Var) T {
	val := env.Get(v)
	if val == nil {
		var zero T
		return zero
	}
	t, ok := val.(T)
	if !ok {
		panic(fmt.Sprintf("pbt: %v holds a %T, not a %s", v, val, reflect.TypeFor[T]()))
	}
	return t
}

// Command is one operation of a stateful test, with its arguments already
// chosen. The functions correspond to PropEr's precondition, the call
// itself, next_state and postcondition. They reach the model and the
// system under test through the variables of the test function that
// declares them: a stateful test keeps both in variables of the property,
// and the commands close over them.
//
// A nil Pre allows the command in every state, a nil Run returns nil, a nil
// Next leaves the model unchanged and a nil Post accepts every result. Name
// and Args appear in reports and statistics; put the arguments of the call
// in Args, with the Var of a result where the call uses one, so that the
// report reads like a program.
type Command struct {
	Name string
	Args []any
	// Pre reports whether the command is allowed in the current state of
	// the model. It is checked when the sequence is generated and again
	// when the sequence runs, so a command whose precondition fails after
	// the shrinker removed an earlier one is not a candidate.
	Pre func() bool
	// Run performs the call against the system. Env maps every Var that
	// has a value to the result its command returned.
	Run func(env Env) any
	// Next updates the model for the command. It runs both while the
	// sequence is generated and while it executes, and receives the result
	// only as a Var in both cases, so the model never depends on real
	// values and the sequence can be regenerated.
	Next func(result Var)
	// Post checks the real result against the state of the model before
	// the command. It runs during execution only.
	Post func(env Env, result any) error
}

func (c Command) pre() bool { return c.Pre == nil || c.Pre() }

func (c Command) run(env Env) any {
	if c.Run == nil {
		return nil
	}
	return c.Run(env)
}

func (c Command) next(result Var) {
	if c.Next != nil {
		c.Next(result)
	}
}

func (c Command) post(env Env, result any) error {
	if c.Post == nil {
		return nil
	}
	return c.Post(env, result)
}

// String formats the command as a call, such as cache(3, 5).
func (c Command) String() string {
	args := make([]string, len(c.Args))
	for i, a := range c.Args {
		args[i] = fmt.Sprintf("%#v", a)
	}
	return c.Name + "(" + strings.Join(args, ", ") + ")"
}

// GoString makes %#v print the call form.
func (c Command) GoString() string { return c.String() }

// Step is one command of a Program together with the Var that names its
// result.
type Step struct {
	Command Command
	Result  Var
}

// StatefulOption configures Commands and ParallelCommands.
type StatefulOption func(*stateful)

type stateful struct {
	minLen, maxLen         int
	branches, maxBranchLen int
	show                   []func() any
}

func statefulConfig(opts []StatefulOption) stateful {
	cfg := stateful{minLen: 0, maxLen: -1, branches: defaultBranches, maxBranchLen: defaultMaxBranchLen}
	for _, o := range opts {
		o(&cfg)
	}
	return cfg
}

// Length bounds the number of commands of a sequence: at least min and at
// most max, where a negative max means no upper bound. In a parallel
// program it bounds the prefix.
func Length(min, max int) StatefulOption {
	return func(c *stateful) { c.minLen, c.maxLen = min, max }
}

// Branches sets the number of branches of a parallel program and the
// maximum length of each. At most three branches and twelve commands in
// total are allowed, which bounds the number of interleavings that
// generation and checking enumerate.
func Branches(n, maxLen int) StatefulOption {
	return func(c *stateful) { c.branches, c.maxBranchLen = n, maxLen }
}

// ModelState lets the report show the model: when a case fails, the
// function is called and its result is printed as the model state before
// the failing step. Several ModelState options are printed side by side.
func ModelState(f func() any) StatefulOption {
	return func(c *stateful) { c.show = append(c.show, f) }
}

// model is what a program knows about the model of its test: how to put it
// in its initial state and how to show it.
type model struct {
	init func()
	show []func() any
}

func (m model) reset() {
	if m.init != nil {
		m.init()
	}
}

// replay puts the model in the state the steps lead to: the initial state,
// then the Next of every step in order.
func (m model) replay(steps ...[]Step) {
	m.reset()
	for _, list := range steps {
		for _, s := range list {
			s.Command.next(s.Result)
		}
	}
}

// state formats the model for the report, or reports false when the test
// gave no ModelState.
func (m model) state() (string, bool) {
	if len(m.show) == 0 {
		return "", false
	}
	parts := make([]string, len(m.show))
	for i, f := range m.show {
		parts[i] = fmt.Sprintf("%+v", f())
	}
	return strings.Join(parts, ", "), true
}

// Program is a generated command sequence.
type Program struct {
	model model
	steps []Step
}

// Steps returns the commands in order.
func (p Program) Steps() []Step { return p.steps }

// Len returns the number of commands.
func (p Program) Len() int { return len(p.steps) }

// String lists the commands, one per line, with the Var each one produces.
func (p Program) String() string {
	if len(p.steps) == 0 {
		return "Program{}"
	}
	var b strings.Builder
	b.WriteString("Program{")
	for _, s := range p.steps {
		fmt.Fprintf(&b, "\n  %v = %s", s.Result, s.Command)
	}
	b.WriteString("\n}")
	return b.String()
}

// GoString makes %#v print the command list.
func (p Program) GoString() string { return p.String() }

// noCommandsName marks the generator NoCommands returns; a zero Gen with
// another name is a mistake and panics when drawn.
const noCommandsName = "NoCommands"

// CommandGen is the type of a generator of commands, an alias of
// Gen[Command]. The generators that Map, Custom and Frequency return are
// CommandGen values as they are; the name is for the signature of a
// function that returns one, where Go does not infer the type argument.
type CommandGen = Gen[Command]

// NoCommands is what a command generator returns in a model state in which
// no command applies, which ends the sequence.
func NoCommands() CommandGen { return CommandGen{name: noCommandsName} }

// Commands generates command sequences for a stateful test. init puts the
// model in its initial state, and gen returns the generator of the next
// command for the current state of the model, as PropEr's command/1 does;
// a command whose Pre does not hold is generated again, and the sequence
// ends when none is found. Sequences grow with the size parameter.
//
// The model lives in variables of the test function: init assigns their
// initial values, the Next functions of the commands update them, and gen
// and the Pre and Post functions read them. pbt calls init again whenever
// it replays the sequence, before the program runs and whenever the
// shrinker regenerates it, so nothing but init and Next may change the
// model. Deleting a command while shrinking regenerates the commands after
// it from the new state and re-checks their preconditions. Generators that
// pick from the model must iterate it in a fixed order; a map iterated
// directly makes the sequence nondeterministic.
func Commands(init func(), gen func() CommandGen, opts ...StatefulOption) Gen[Program] {
	cfg := statefulConfig(opts)
	checkLengths(cfg.minLen, cfg.maxLen)
	return Gen[Program]{name: "Commands", draw: func(t *T) Program {
		m := model{init: init, show: cfg.show}
		m.reset()
		steps := drawSteps(t, gen, 1, cfg.minLen, cfg.maxLen)
		return Program{model: m, steps: steps}
	}}
}

// drawSteps generates a command sequence from the current state of the
// model, numbering the result Vars from firstID, and leaves the model in
// the state the sequence leads to.
func drawSteps(t *T, gen func() CommandGen, firstID, minLen, maxLen int) []Step {
	var steps []Step
	p := continueProbability(t, maxLen)
	for {
		t.beginSpan("command")
		var more bool
		switch {
		case len(steps) < minLen:
			t.forced(1)
			more = true
		case maxLen >= 0 && len(steps) >= maxLen:
			t.forced(0)
		default:
			more = t.drawBool(p)
		}
		if !more {
			t.endSpan()
			return steps
		}
		cmd, ok := drawCommand(t, gen())
		t.endSpan()
		if !ok {
			if len(steps) >= minLen {
				return steps
			}
			t.discard(fmt.Sprintf("no command satisfies its precondition after %d commands", len(steps)))
		}
		v := Var{id: firstID + len(steps)}
		cmd.next(v)
		steps = append(steps, Step{Command: cmd, Result: v})
	}
}

// drawCommand draws commands until one satisfies its precondition. Each
// attempt sits in its own span, so the shrinker can delete rejected ones.
func drawCommand(t *T, g CommandGen) (Command, bool) {
	if g.draw == nil {
		if g.name != noCommandsName {
			panic("pbt: the command generator returned an uninitialized Gen; return NoCommands() to end the sequence")
		}
		return Command{}, false
	}
	for range maxFilterTries {
		if c := g.drawSpan(t); c.pre() {
			return c, true
		}
	}
	return Command{}, false
}

// RunSequential puts the model in its initial state, executes the commands
// one after another and checks every postcondition. It records each
// command's name for the statistics, as aggregate(command_names(Cmds))
// does in PropEr, and on failure logs the history of calls and results
// before failing the case.
func (p Program) RunSequential(t *T) {
	env := Env{vals: map[Var]any{}}
	p.model.reset()
	runSteps(t, p.model, p.steps, env, nil)
}

// runSteps executes steps in order from the current state of the model and
// leaves the model in the state they lead to. On any failure it logs the
// history, preceded by the extra lines (the history of an earlier phase),
// and fails the case.
func runSteps(t *T, m model, steps []Step, env Env, extra []string) {
	var history []string
	// Each failure carries an identity (which check, on which command) so
	// that shrinking keeps the failure it started from.
	fail := func(step int, key, format string, args ...any) {
		t.Logf("history:")
		for _, h := range append(extra, history...) {
			t.Logf("  %s", h)
		}
		if state, ok := m.state(); ok {
			t.Logf("model state before step %d: %s", step, state)
		}
		t.fatalAt(key, fmt.Sprintf("step %d: %s", step, fmt.Sprintf(format, args...)))
	}
	for i, s := range steps {
		n := i + 1
		t.Aggregate(s.Command.Name)
		if !s.Command.pre() {
			fail(n, "precondition of "+s.Command.Name, "precondition of %s does not hold at execution time", s.Command)
		}
		result, panicked, stack := executeCommand(s.Command, env)
		if panicked != nil {
			if d, isDiscard := panicked.(discardTest); isDiscard {
				panic(d)
			}
			if t.stack == nil {
				t.stack = stack
			}
			history = append(history, fmt.Sprintf("%d. %s panicked: %v", n, s.Command, panicked))
			fail(n, panicKey(panicked, stack), "%s panicked: %v", s.Command, panicked)
		}
		env.vals[s.Result] = result
		history = append(history, historyLine(n, s, result))
		if err := s.Command.post(env, result); err != nil {
			fail(n, "postcondition of "+s.Command.Name, "postcondition of %s failed: %v", s.Command, err)
		}
		s.Command.next(s.Result)
	}
}

func historyLine(n int, s Step, result any) string {
	return fmt.Sprintf("%d. %v = %s -> %#v", n, s.Result, s.Command, result)
}

// executeCommand runs one command and captures a panic with its stack. A
// discard is captured as well, so that a branch goroutine never panics; the
// caller raises it again on its own goroutine.
func executeCommand(c Command, env Env) (result any, panicked any, stack []byte) {
	defer func() {
		if rec := recover(); rec != nil {
			panicked = rec
			if _, isDiscard := rec.(discardTest); !isDiscard {
				stack = debug.Stack()
			}
		}
	}()
	return c.run(env), nil, nil
}
