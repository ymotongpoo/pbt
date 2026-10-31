package pbt

import (
	"fmt"
	"strings"
)

// report writes the counterexample to the test log and marks the test as
// failed.
func (r *runner) report(t *T, headline string, trailer []string) {
	r.tb.Helper()
	r.tb.Errorf("%s", r.format(t, headline, trailer))
}

// format renders a test case: the headline, every value the property drew,
// its logs, errors, panic and the trailer lines.
func (r *runner) format(t *T, headline string, trailer []string) string {
	t.runOnFail()
	var b strings.Builder
	fmt.Fprintf(&b, "[pbt] %s (size %d)\n", headline, t.size)
	if t.hasUtility {
		fmt.Fprintf(&b, "[pbt] utility %s = %s\n", t.utilityName, formatNum(t.utility))
	}
	for _, d := range t.draws {
		indent := strings.Repeat("  ", d.depth)
		fmt.Fprintf(&b, "[pbt] %s%s\n", indent, strings.ReplaceAll(formatDraw(d), "\n", "\n[pbt]   "+indent))
	}
	for _, l := range t.logs {
		fmt.Fprintf(&b, "[pbt] log: %s\n", l)
	}
	for _, m := range t.messages {
		fmt.Fprintf(&b, "[pbt] error: %s\n", m)
	}
	if t.panicVal != nil {
		fmt.Fprintf(&b, "[pbt] panic: %v\n", t.panicVal)
		for _, fr := range userFrames(t.stack, 8, r.cfg.verbose) {
			fmt.Fprintf(&b, "[pbt]   %s\n", fr)
		}
	}
	for _, line := range trailer {
		fmt.Fprintf(&b, "[pbt] %s\n", line)
	}
	return strings.TrimRight(b.String(), "\n")
}

func formatDraw(d drawRecord) string {
	if d.label == "" {
		return fmt.Sprintf("%s = %#v", d.gen, d.value)
	}
	return fmt.Sprintf("%s (%s) = %#v", d.label, d.gen, d.value)
}

// userFrames extracts "file:line function" entries from a stack trace,
// dropping the runtime and this package unless verbose is set.
func userFrames(stack []byte, limit int, verbose bool) []string {
	lines := strings.Split(string(stack), "\n")
	var out []string
	for i := 1; i+1 < len(lines) && len(out) < limit; i += 2 {
		fn, loc := lines[i], strings.Fields(strings.TrimSpace(lines[i+1]))
		if len(loc) == 0 {
			continue
		}
		if !verbose && (strings.HasPrefix(fn, "runtime.") || strings.HasPrefix(fn, "runtime/debug.") ||
			strings.HasPrefix(fn, "panic(") || strings.HasPrefix(fn, "testing.") ||
			strings.HasPrefix(fn, "go.ymotongpoo.com/pbt.")) {
			continue
		}
		out = append(out, loc[0]+" "+fn)
	}
	return out
}
