// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package format

import (
	"encoding/json"
	"math"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"go.ymotongpoo.com/pbt"
)

func TestJSON(t *testing.T) {
	checkValid(t, JSON(), func(s string) string {
		if !json.Valid([]byte(s)) {
			return "encoding/json.Valid rejects it"
		}
		return ""
	})
	bigInt := regexp.MustCompile(`(^|[^0-9.eE+\-])-?[1-9][0-9]{20,}`)
	pbt.Check(t, func(pt *pbt.T) {
		s := JSON().Draw(pt, "s")
		trimmed := strings.TrimSpace(s)
		pt.Cover(trimmed != "" && trimmed[0] != '[' && trimmed[0] != '{', 20, "scalar at the top")
		pt.Cover(strings.Contains(s, "[[") || strings.Contains(s, "[{") || strings.Contains(s, `":{`) || strings.Contains(s, `":[`), 3, "nested")
		pt.Cover(strings.Contains(s, `\u`), 10, `\u escape`)
		pt.Cover(regexp.MustCompile(`\\u[dD][89abAB]`).MatchString(s), 2, "surrogate escape")
		pt.Cover(strings.ContainsAny(s, "eE") && regexp.MustCompile(`[0-9][eE]`).MatchString(s), 5, "exponent")
		pt.Cover(bigInt.MatchString(s), 1, "integer beyond 64 bits")
		pt.Cover(strings.ContainsAny(s, "\n\t\r"), 5, "white space other than spaces")
	}, pbt.Seed(1), pbt.Checks(2000), pbt.NoSave(), pbt.CheckCoverage())
	if got := minimalOf(t, JSON(), always); got != "null" {
		t.Errorf("shrinks to %q", got)
	}
}

// loneSurrogate finds a \u escape of a surrogate that is not part of a
// high-low pair.
var surrogateRe = regexp.MustCompile(`\\u[dD][89a-fA-F][0-9a-fA-F]{2}`)

func lonelySurrogate(s string) bool {
	locs := surrogateRe.FindAllStringIndex(s, -1)
	for i := 0; i < len(locs); i++ {
		hi, _ := strconv.ParseUint(s[locs[i][0]+2:locs[i][1]], 16, 16)
		if hi >= 0xDC00 {
			return true
		}
		if i+1 == len(locs) || locs[i+1][0] != locs[i][1] {
			return true
		}
		lo, _ := strconv.ParseUint(s[locs[i+1][0]+2:locs[i+1][1]], 16, 16)
		if lo < 0xDC00 {
			return true
		}
		i++
	}
	return false
}

// checkIJSON walks the tokens of s and checks the rules of RFC 7493.
func checkIJSON(s string) string {
	if !json.Valid([]byte(s)) {
		return "not JSON"
	}
	if trimmed := strings.TrimSpace(s); trimmed[0] != '[' && trimmed[0] != '{' {
		return "neither an object nor an array at the top"
	}
	if lonelySurrogate(s) {
		return "unpaired surrogate escape"
	}
	dec := json.NewDecoder(strings.NewReader(s))
	dec.UseNumber()
	type frame struct {
		object bool
		names  map[string]bool
		key    bool // the next string token is a member name
	}
	var stack []*frame
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		var top *frame
		if len(stack) > 0 {
			top = stack[len(stack)-1]
		}
		switch v := tok.(type) {
		case json.Delim:
			switch v {
			case '{', '[':
				if top != nil && top.object {
					top.key = true
				}
				stack = append(stack, &frame{object: v == '{', names: map[string]bool{}, key: v == '{'})
			default:
				stack = stack[:len(stack)-1]
			}
			continue
		case string:
			for _, r := range v {
				if isNoncharacter(r) || r == 0xFFFD && !strings.Contains(s, "�") {
					return "noncharacter or replaced surrogate"
				}
			}
			if top != nil && top.object && top.key {
				if top.names[v] {
					return "duplicate member name " + strconv.Quote(v)
				}
				top.names[v] = true
				top.key = false
				continue
			}
		case json.Number:
			f, err := strconv.ParseFloat(v.String(), 64)
			if err != nil || math.IsInf(f, 0) {
				return "number outside binary64: " + v.String()
			}
			if !strings.ContainsAny(v.String(), ".eE") && math.Abs(f) > 1<<53-1 {
				return "integer beyond 2^53-1: " + v.String()
			}
		}
		if top != nil && top.object {
			top.key = true
		}
	}
	return ""
}

func TestIJSON(t *testing.T) {
	checkValid(t, IJSON(), checkIJSON)
	pbt.Check(t, func(pt *pbt.T) {
		s := IJSON().Draw(pt, "s")
		pt.Cover(strings.HasPrefix(strings.TrimSpace(s), "{"), 30, "object")
		pt.Cover(strings.Contains(s, `\u`), 10, `\u escape`)
		pt.Cover(surrogateRe.MatchString(s), 1, "escaped surrogate pair")
		pt.Cover(regexp.MustCompile(`[0-9][eE]`).MatchString(s), 5, "exponent")
	}, pbt.Seed(1), pbt.Checks(2000), pbt.NoSave(), pbt.CheckCoverage())
	if got := minimalOf(t, IJSON(), always); got != "[]" {
		t.Errorf("shrinks to %q", got)
	}
	for _, s := range []string{`{"a":1,"a":2}`, `["\ud800"]`, `1`, `[9007199254740993]`} {
		if checkIJSON(s) == "" {
			t.Errorf("checkIJSON accepts %s", s)
		}
	}
}
