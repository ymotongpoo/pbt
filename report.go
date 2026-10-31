// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package pbt

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// defaultReportDepth is how many levels of nested labeled draws a report
// lists under a top-level draw; the ReportDepth option changes it.
const defaultReportDepth = 3

// defaultValueLimit is how many characters of one drawn value a report
// shows before it cuts the text; the ReportLimit option changes it.
const defaultValueLimit = 2000

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
	omitted := 0
	for _, d := range t.draws {
		if d.depth > r.cfg.reportDepth {
			omitted++
			continue
		}
		indent := strings.Repeat("  ", d.depth)
		fmt.Fprintf(&b, "[pbt] %s%s\n", indent, strings.ReplaceAll(formatDrawLimit(d, r.cfg.valueLimit), "\n", "\n[pbt]   "+indent))
	}
	if omitted > 0 {
		fmt.Fprintf(&b, "[pbt] (%d labeled draws nested deeper than %d levels omitted; raise the depth with ReportDepth)\n", omitted, r.cfg.reportDepth)
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
	return formatDrawLimit(d, defaultValueLimit)
}

// formatDrawLimit formats a draw, cutting the value text at limit characters
// and saying how much was left out. A limit of 0 means no cut.
func formatDrawLimit(d drawRecord, limit int) string {
	text := fmt.Sprintf("%#v", d.value)
	if _, ok := d.value.(undrawn); ok {
		text = "(the draw did not finish)"
	}
	if limit > 0 && len(text) > limit {
		cut := text[:limit]
		for !utf8.ValidString(cut) {
			cut = cut[:len(cut)-1]
		}
		text = fmt.Sprintf("%s… (%d more characters; raise the limit with ReportLimit)", cut, utf8.RuneCountInString(text)-utf8.RuneCountInString(cut))
	}
	if d.label == "" {
		return fmt.Sprintf("%s = %s", d.gen, text)
	}
	return fmt.Sprintf("%s (%s) = %s", d.label, d.gen, text)
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
		file, _, _ := strings.Cut(loc[0], ":")
		if !verbose && (strings.HasPrefix(fn, "panic(") || strings.HasPrefix(fn, "testing.") ||
			internalFrame(fn, file)) {
			continue
		}
		out = append(out, loc[0]+" "+fn)
	}
	return out
}
