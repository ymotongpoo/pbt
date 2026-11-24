// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package format

import (
	"go.ymotongpoo.com/pbt"
	"strings"
)

// RFC4180CSV generates file of RFC 4180 §2: one to six records and, in
// half the cases, a header with the same one to six fields per row. Header
// names have the same syntax as fields; their presence is not identifiable
// without the MIME header parameter. Half the files have a final CRLF.
// Fields have 0 to 24 characters; half are quoted, with doubled quotes
// and commas, CR and LF allowed inside. Only ASCII TEXTDATA is generated.
// Values shrink toward "", a single empty unquoted field.
//
// encoding/csv also accepts bare LF separators and drops blank records.
// It normalizes CRLF inside quoted fields to LF and drops a final bare CR.
func RFC4180CSV() pbt.Gen[string] {
	return pbt.Custom("RFC4180CSV()", func(t *pbt.T) string {
		columns := pbt.IntRange(1, 6).Draw(t, "")
		row := joined(csvField, columns, columns, ",")
		header := optionalText(affix(row, "", "\r\n")).Draw(t, "")
		records := joined(row, 1, 6, "\r\n").Draw(t, "")
		return header + records + pbt.OneOf(pbt.Just(""), pbt.Just("\r\n")).Draw(t, "")
	})
}

var csvField = pbt.OneOf(csvFieldOf(false), csvFieldOf(true))

func csvFieldOf(quoted bool) pbt.Gen[string] {
	return pbt.Custom("field", func(t *pbt.T) string {
		n := pbt.IntRange(0, 24).Draw(t, "")
		set := strings.ReplaceAll(strings.ReplaceAll(vchar+" ", "\"", ""), ",", "")
		if quoted {
			set += ",\r\n\""
		}
		var b strings.Builder
		if quoted {
			b.WriteByte('"')
		}
		for range n {
			c := charIn(set).Draw(t, "")
			b.WriteByte(c)
			if c == '"' {
				b.WriteByte(c)
			}
		}
		if quoted {
			b.WriteByte('"')
		}
		return b.String()
	})
}
