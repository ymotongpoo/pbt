// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package format

import (
	"encoding/csv"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"go.ymotongpoo.com/pbt"
)

// readCSVGrammar is a byte parser for file/record/field/TEXTDATA, retaining
// blank records which encoding/csv deliberately skips.
func readCSVGrammar(s string) ([][]string, error) {
	var rows [][]string
	var row []string
	for {
		var b strings.Builder
		if len(s) > 0 && s[0] == '"' {
			s = s[1:]
			closed := false
			for len(s) > 0 {
				c := s[0]
				s = s[1:]
				if c == '"' {
					if len(s) > 0 && s[0] == '"' {
						b.WriteByte('"')
						s = s[1:]
						continue
					}
					closed = true
					break
				}
				if c != 10 && c != 13 && (c < 32 || c > 126) {
					return nil, fmt.Errorf("quoted byte")
				}
				b.WriteByte(c)
			}
			if !closed {
				return nil, fmt.Errorf("unclosed quote")
			}
		} else {
			for len(s) > 0 && s[0] != ',' && s[0] != '\r' {
				c := s[0]
				if c < 32 || c > 126 || c == '"' {
					return nil, fmt.Errorf("TEXTDATA")
				}
				b.WriteByte(c)
				s = s[1:]
			}
		}
		row = append(row, b.String())
		if len(s) > 0 && s[0] == ',' {
			s = s[1:]
			continue
		}
		if len(rows) > 0 && len(row) != len(rows[0]) {
			return nil, fmt.Errorf("width")
		}
		rows = append(rows, row)
		row = nil
		if s == "" {
			return rows, nil
		}
		if !strings.HasPrefix(s, "\r\n") {
			return nil, fmt.Errorf("separator")
		}
		s = s[2:]
		if s == "" {
			return rows, nil
		}
	}
}
func TestRFC4180CSV(t *testing.T) {
	g := RFC4180CSV()
	checkMinimumSeeds(t, g, "")
	checkValid(t, g, func(s string) string {
		if _, e := readCSVGrammar(s); e != nil {
			return e.Error()
		}
		if _, e := csv.NewReader(strings.NewReader(s)).ReadAll(); e != nil {
			return e.Error()
		}
		return ""
	})
	pbt.Check(t, func(pt *pbt.T) {
		s := g.Draw(pt, "")
		pt.Cover(strings.HasSuffix(s, "\r\n"), 30, "final CRLF")
		pt.Cover(!strings.HasSuffix(s, "\r\n"), 30, "no final CRLF")
		pt.Cover(strings.Contains(s, "\""), 50, "quoted")
		pt.Cover(strings.Contains(s, "\"\""), 5, "doubled quote or empty quoted")
		pt.Cover(strings.Contains(s, ","), 50, "comma")
		rows, _ := readCSVGrammar(s)
		var lf, cr, quote, comma bool
		for _, row := range rows {
			for _, v := range row {
				lf = lf || strings.Contains(v, "\n")
				cr = cr || strings.Contains(v, "\r")
				quote = quote || strings.Contains(v, "\"")
				comma = comma || strings.Contains(v, ",")
			}
		}
		pt.Cover(lf, 10, "embedded LF")
		pt.Cover(cr, 10, "embedded CR")
		pt.Cover(quote, 10, "doubled quote")
		pt.Cover(comma, 10, "quoted comma")
	}, pbt.Seed(1), pbt.Checks(2000), pbt.NoSave(), pbt.CheckCoverage())
	if s := minimalOf(t, g, always); s != "" {
		t.Errorf("minimum %q", s)
	}
}
func TestCSVParserDifferences(t *testing.T) {
	for _, tc := range []struct {
		s    string
		want [][]string
	}{
		{"a\nb\n", [][]string{{"a"}, {"b"}}},
		{"\r\n", nil},
		{"\"a\r\nb\"", [][]string{{"a\nb"}}},
		{"a\r", [][]string{{"a"}}},
	} {
		got, e := csv.NewReader(strings.NewReader(tc.s)).ReadAll()
		if e != nil || !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%q: %q, %v", tc.s, got, e)
		}
	}
}
