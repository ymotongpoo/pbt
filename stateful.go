package pbt

import (
	"fmt"
	"reflect"
	"runtime/debug"
	"strings"
)

// Var names the result of a command before the command has run. While a
// command sequence is generated, no command has executed, so the model state
// can refer to a result only through its Var; during execution, Env resolves
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

// Resolve returns the value of v as a T.
func Resolve[T any](env Env, v Var) T {
	val := env.Get(v)
	t, ok := val.(T)
	if !ok {
		var want T
		panic(fmt.Sprintf("pbt: %v holds a %T, not a %T", v, val, want))
	}
	return t
}

// Command is one operation of a stateful test, with its arguments already
// chosen. S is the model state and Sys the system under test. The methods
// correspond to PropEr's precondition, the call itself, next_state and
// postcondition.
//
// NextState runs both while the sequence is generated and while it executes,
// and receives the result only as a Var in both cases, so the model evolves
// identically. Postcondition receives the real result and an Env to resolve
// any Var the command refers to.
type Command[S, Sys any] interface {
	Precondition(state S) bool
	Execute(sys Sys, env Env) any
	NextState(state S, result Var) S
	Postcondition(state S, env Env, result any) error
}

// Cmd implements Command with function fields, which lets a command
// generator build commands inline. A nil Pre allows the command in every
// state, a nil Next leaves the state unchanged, and a nil Post accepts every
// result. Name and Args appear in reports and statistics.
type Cmd[S, Sys any] struct {
	Name string
	Args []any
	Pre  func(state S) bool
	Run  func(sys Sys, env Env) any
	Next func(state S, result Var) S
	Post func(state S, env Env, result any) error
}

// Precondition implements Command.
func (c Cmd[S, Sys]) Precondition(state S) bool {
	return c.Pre == nil || c.Pre(state)
}

// Execute implements Command.
func (c Cmd[S, Sys]) Execute(sys Sys, env Env) any {
	if c.Run == nil {
		return nil
	}
	return c.Run(sys, env)
}

// NextState implements Command.
func (c Cmd[S, Sys]) NextState(state S, result Var) S {
	if c.Next == nil {
		return state
	}
	return c.Next(state, result)
}

// Postcondition implements Command.
func (c Cmd[S, Sys]) Postcondition(state S, env Env, result any) error {
	if c.Post == nil {
		return nil
	}
	return c.Post(state, env, result)
}

// CommandName returns Name, the label used in statistics.
func (c Cmd[S, Sys]) CommandName() string { return c.Name }

// String formats the command as a call, such as cache(3, 5).
func (c Cmd[S, Sys]) String() string {
	args := make([]string, len(c.Args))
	for i, a := range c.Args {
		args[i] = fmt.Sprintf("%#v", a)
	}
	return c.Name + "(" + strings.Join(args, ", ") + ")"
}

// GoString makes %#v print the call form.
func (c Cmd[S, Sys]) GoString() string { return c.String() }

// CommandName is the label of a command in statistics: its CommandName
// method when it has one, otherwise its type name.
func CommandName(c any) string {
	if n, ok := c.(interface{ CommandName() string }); ok {
		return n.CommandName()
	}
	typ := reflect.TypeOf(c)
	for typ != nil && typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if typ == nil {
		return "nil"
	}
	return typ.Name()
}

// describe formats a command for the history: its String when it has one.
func describe(c any) string {
	if s, ok := c.(fmt.Stringer); ok {
		return s.String()
	}
	return fmt.Sprintf("%#v", c)
}

// Step is one command of a Program together with the Var that names its
// result.
type Step[S, Sys any] struct {
	Command Command[S, Sys]
	Result  Var
}

// Program is a generated command sequence with the model state it starts
// from.
type Program[S, Sys any] struct {
	init  S
	steps []Step[S, Sys]
}

// Steps returns the commands in order.
func (p Program[S, Sys]) Steps() []Step[S, Sys] { return p.steps }

// Len returns the number of commands.
func (p Program[S, Sys]) Len() int { return len(p.steps) }

// String lists the commands, one per line, with the Var each one produces.
func (p Program[S, Sys]) String() string {
	if len(p.steps) == 0 {
		return "Program{}"
	}
	var b strings.Builder
	b.WriteString("Program{")
	for _, s := range p.steps {
		fmt.Fprintf(&b, "\n  %v = %s", s.Result, describe(s.Command))
	}
	b.WriteString("\n}")
	return b.String()
}

// GoString makes %#v print the command list.
func (p Program[S, Sys]) GoString() string { return p.String() }

// NoCommands is what a command generator returns for a model state in which
// no command applies, which ends the sequence.
func NoCommands[S, Sys any]() Gen[Command[S, Sys]] { return Gen[Command[S, Sys]]{name: "NoCommands"} }

// Commands generates command sequences for a stateful test. gen returns the
// generator of the next command for a model state, as PropEr's command/1
// does; a command whose Precondition does not hold is generated again, and
// the sequence ends when none is found. Sequences grow with the size
// parameter. Deleting a command while shrinking regenerates the commands
// after it from the new state and re-checks their preconditions.
//
// The model state must be treated as immutable: NextState returns a new
// state and must not modify the one it receives, because the initial state
// is shared by every test case and generation replays the same sequence.
// Generators that pick from the model must iterate it in a fixed order; a
// map iterated directly makes the sequence nondeterministic.
func Commands[S, Sys any](init S, gen func(state S) Gen[Command[S, Sys]]) Gen[Program[S, Sys]] {
	return named(CommandsN(init, gen, 0, -1), "Commands")
}

// CommandsN generates command sequences with between minLen and maxLen
// commands. A negative maxLen means no upper bound.
func CommandsN[S, Sys any](init S, gen func(state S) Gen[Command[S, Sys]], minLen, maxLen int) Gen[Program[S, Sys]] {
	checkLengths(minLen, maxLen)
	return Gen[Program[S, Sys]]{name: fmt.Sprintf("CommandsN(%d, %d)", minLen, maxLen), draw: func(t *T) Program[S, Sys] {
		state := init
		var steps []Step[S, Sys]
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
				break
			}
			cmd, ok := drawCommand(t, gen(state), state)
			t.endSpan()
			if !ok {
				if len(steps) >= minLen {
					break
				}
				t.discard(fmt.Sprintf("no command satisfies its precondition after %d commands", len(steps)))
			}
			v := Var{id: len(steps) + 1}
			state = cmd.NextState(state, v)
			steps = append(steps, Step[S, Sys]{Command: cmd, Result: v})
		}
		return Program[S, Sys]{init: init, steps: steps}
	}}
}

// drawCommand draws commands until one satisfies its precondition. Each
// attempt sits in its own span, so the shrinker can delete rejected ones.
func drawCommand[S, Sys any](t *T, g Gen[Command[S, Sys]], state S) (Command[S, Sys], bool) {
	if g.draw == nil {
		return nil, false
	}
	for range maxFilterTries {
		if c := g.drawSpan(t); c != nil && c.Precondition(state) {
			return c, true
		}
	}
	return nil, false
}

// RunSequential executes the commands against sys one after another and
// checks every postcondition. It records each command's name for the
// statistics, as aggregate(command_names(Cmds)) does in PropEr, and on
// failure logs the history of calls and results before failing the case.
func (p Program[S, Sys]) RunSequential(t *T, sys Sys) {
	state := p.init
	env := Env{vals: map[Var]any{}}
	var history []string
	fail := func(step int, format string, args ...any) {
		t.Logf("history:")
		for _, h := range history {
			t.Logf("  %s", h)
		}
		t.Logf("model state before step %d: %+v", step, state)
		t.Fatalf("step %d: %s", step, fmt.Sprintf(format, args...))
	}
	for i, s := range p.steps {
		n := i + 1
		t.Aggregate(CommandName(s.Command))
		if !s.Command.Precondition(state) {
			fail(n, "precondition of %s does not hold at execution time", describe(s.Command))
		}
		result, panicked := executeCommand(t, s.Command, sys, env)
		if panicked != nil {
			history = append(history, fmt.Sprintf("%d. %s panicked: %v", n, describe(s.Command), panicked))
			fail(n, "%s panicked: %v", describe(s.Command), panicked)
		}
		env.vals[s.Result] = result
		history = append(history, fmt.Sprintf("%d. %v = %s -> %#v", n, s.Result, describe(s.Command), result))
		if err := s.Command.Postcondition(state, env, result); err != nil {
			fail(n, "postcondition of %s failed: %v", describe(s.Command), err)
		}
		state = s.Command.NextState(state, s.Result)
	}
}

func executeCommand[S, Sys any](t *T, c Command[S, Sys], sys Sys, env Env) (result any, panicked any) {
	defer func() {
		if rec := recover(); rec != nil {
			if _, isDiscard := rec.(discardTest); isDiscard {
				panic(rec)
			}
			panicked = rec
			if t.stack == nil {
				t.stack = debug.Stack()
			}
		}
	}()
	return c.Execute(sys, env), nil
}
