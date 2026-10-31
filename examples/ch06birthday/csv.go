// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

// Package ch06birthday ports the birthday greetings kata of Chapter 6 of the
// PropEr Testing book: a CSV codec, the employee records read from it, the
// birthday filter, the mail template, and the plumbing between them.
package ch06birthday

import (
	"errors"
	"slices"
	"strings"
)

// Encode writes rows as CSV: the sorted keys of the first row as the
// header, then one record per row, separated by CRLF without a trailing
// one. Fields that contain CRLF, a double quote or a comma are quoted, with
// inner quotes doubled, as RFC 4180 describes.
func Encode(rows []map[string]string) string {
	if len(rows) == 0 {
		return ""
	}
	keys := make([]string, 0, len(rows[0]))
	for k := range rows[0] {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	lines := make([]string, 0, len(rows)+1)
	lines = append(lines, encodeRecord(keys))
	for _, row := range rows {
		fields := make([]string, len(keys))
		for i, k := range keys {
			fields[i] = row[k]
		}
		lines = append(lines, encodeRecord(fields))
	}
	return strings.Join(lines, "\r\n")
}

func encodeRecord(fields []string) string {
	escaped := make([]string, len(fields))
	for i, f := range fields {
		escaped[i] = escape(f)
	}
	return strings.Join(escaped, ",")
}

func escape(field string) string {
	if strings.ContainsAny(field, "\r\n\",") {
		return `"` + strings.ReplaceAll(field, `"`, `""`) + `"`
	}
	return field
}

// ErrFieldCount reports a record whose field count differs from the header.
var ErrFieldCount = errors.New("record has a different number of fields than the header")

// Decode parses CSV with a header line into one map per record. A trailing
// CRLF is optional. Duplicate header names are not supported: the last
// column wins, because a map holds one value per key.
func Decode(s string) ([]map[string]string, error) {
	if s == "" {
		return nil, nil
	}
	s = strings.TrimSuffix(s, "\r\n")
	records := parseRecords(s)
	header := records[0]
	rows := make([]map[string]string, 0, len(records)-1)
	for _, rec := range records[1:] {
		if len(rec) != len(header) {
			return nil, ErrFieldCount
		}
		row := make(map[string]string, len(header))
		for i, name := range header {
			row[name] = rec[i]
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// parseRecords splits CSV text into records of fields, honoring quotes.
func parseRecords(s string) [][]string {
	var records [][]string
	var fields []string
	var field strings.Builder
	quoted := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case quoted && c == '"' && i+1 < len(s) && s[i+1] == '"':
			field.WriteByte('"')
			i++
		case quoted && c == '"':
			quoted = false
		case quoted:
			field.WriteByte(c)
		case c == '"':
			quoted = true
		case c == ',':
			fields = append(fields, field.String())
			field.Reset()
		case c == '\r' && i+1 < len(s) && s[i+1] == '\n':
			fields = append(fields, field.String())
			field.Reset()
			records = append(records, fields)
			fields = nil
			i++
		default:
			field.WriteByte(c)
		}
	}
	fields = append(fields, field.String())
	return append(records, fields)
}
