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
	shrinksTo(t, JSON(), always, "null")
}

// lonelySurrogate lexes the strings of a JSON text, escapes included, and
// reports whether a \u escape of a surrogate is not part of a high-low
// pair. An escaped backslash followed by "u" is not an escape.
func lonelySurrogate(s string) bool {
	var units []uint16 // the code units of \u escapes; 0 marks anything else
	inString := false
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case !inString:
			inString = c == '"'
		case c == '"':
			inString = false
			units = append(units, 0)
		case c == '\\' && s[i+1] == 'u':
			u, _ := strconv.ParseUint(s[i+2:i+6], 16, 16)
			units = append(units, uint16(u))
			i += 5
		case c == '\\':
			units = append(units, 0)
			i++
		default:
			units = append(units, 0)
		}
	}
	for i := 0; i < len(units); i++ {
		switch u := units[i]; {
		case u >= 0xD800 && u < 0xDC00:
			if i+1 == len(units) || units[i+1] < 0xDC00 || units[i+1] > 0xDFFF {
				return true
			}
			i++
		case u >= 0xDC00 && u <= 0xDFFF:
			return true
		}
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
				if isNoncharacter(r) {
					return "noncharacter"
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
		pt.Cover(regexp.MustCompile(`\\u[dD][89abAB]`).MatchString(s), 1, "escaped surrogate pair")
		pt.Cover(regexp.MustCompile(`[0-9][eE]`).MatchString(s), 5, "exponent")
	}, pbt.Seed(1), pbt.Checks(2000), pbt.NoSave(), pbt.CheckCoverage())
	shrinksTo(t, IJSON(), always, "[]", "{}")
	for _, s := range []string{`{"a":1,"a":2}`, `["\ud800"]`, `["\udc00\ud800"]`, `1`, `[9007199254740993]`, `["\ufdd0"]`} {
		if checkIJSON(s) == "" {
			t.Errorf("checkIJSON accepts %s", s)
		}
	}
	for _, s := range []string{`["\ufffd"]`, `["\\ud800"]`, `["\ud83d\ude00"]`, "[\"\uFFFD\"]"} {
		if msg := checkIJSON(s); msg != "" {
			t.Errorf("checkIJSON rejects %s: %s", s, msg)
		}
	}
}
