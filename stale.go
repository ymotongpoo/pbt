// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package pbt

import (
	"bufio"
	"bytes"
	"fmt"
	"reflect"
	"strings"
)

// checkStale compares replay results without wrapping execution: disabled
// diagnostics must not add frames to the property's verbose panic report.
func (r *runner) checkStale(path string, data []byte, t *T) {
	if !r.cfg.healthEnabled(HealthStale) {
		return
	}
	if note := staleDrawNote(path, data, t); note != nil {
		// ReplayFile may also be in the automatically replayed directory.
		for _, old := range r.healthExtra {
			if old == *note {
				return
			}
		}
		r.healthExtra = append(r.healthExtra, *note)
	}
}

func staleDrawNote(path string, data []byte, t *T) *healthNote {
	// decodeCase has already checked the header, choices and scanner limit.
	// Preserve spaces in comments, including trailing spaces in rendered values.
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(nil, 1<<24)
	var saved []string
	for sc.Scan() {
		if strings.TrimSpace(sc.Text()) == "choices" {
			break
		}
		if s, ok := strings.CutPrefix(sc.Text(), "# "); ok {
			saved = append(saved, s)
		}
	}
	for i := 0; i < max(len(saved), len(t.draws)); i++ {
		before, now := "(no draw)", "(no draw)"
		if i < len(saved) {
			before = saved[i]
		}
		if i < len(t.draws) {
			d := t.draws[i]
			if !stableDrawValue(reflect.ValueOf(d.value), 0) {
				continue
			}
			now = strings.ReplaceAll(formatDraw(d), "\n", " ")
		}
		if before != now {
			return &healthNote{HealthStale, fmt.Sprintf("saved case %q is stale at draw %d: saved %q; replayed now %q", path, i+1, before, now)}
		}
	}
	return nil
}

// stableDrawValue is conservative: addresses and user-defined formatters may
// change without an input change. Inspect nested fields too, including private
// fields, since %#v renders those. The depth bound also stops cyclic containers.
func stableDrawValue(v reflect.Value, depth int) bool {
	if !v.IsValid() {
		return true
	}
	if depth >= 100 || v.Type().Implements(reflect.TypeFor[fmt.Formatter]()) || v.Type().Implements(reflect.TypeFor[fmt.GoStringer]()) {
		return false
	}
	switch v.Kind() {
	case reflect.Pointer, reflect.UnsafePointer, reflect.Func, reflect.Chan:
		return false
	case reflect.Interface:
		return stableDrawValue(v.Elem(), depth+1)
	case reflect.Array, reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			if !stableDrawValue(v.Index(i), depth+1) {
				return false
			}
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if !stableDrawValue(v.Field(i), depth+1) {
				return false
			}
		}
	case reflect.Map:
		it := v.MapRange()
		for it.Next() {
			// NaN keys (including NaNs inside comparable aggregates) do not
			// have a stable ordering when fmt sorts otherwise equal keys.
			if !it.Key().Equal(it.Key()) {
				return false
			}
			if !stableDrawValue(it.Key(), depth+1) || !stableDrawValue(it.Value(), depth+1) {
				return false
			}
		}
	}
	return true
}

// A generation error bypasses the usual health report. Keep replay diagnostics
// visible on that path too, without adding unrelated generation diagnostics.
func (r *runner) reportReplayHealth() {
	for _, n := range r.healthExtra {
		if n.check != HealthStale || r.cfg.ignoreHealth[n.check] {
			continue
		}
		if r.cfg.checkHealth[n.check] {
			r.tb.Errorf("[pbt] %s", healthLine(n, false))
		} else {
			r.tb.Logf("[pbt] %s", healthLine(n, true))
		}
	}
}
