// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package format

import (
	"strings"

	"go.ymotongpoo.com/pbt"
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
		columns := csvColumns.Draw(t, "")
		return csvHeaders[columns].Draw(t, "") + csvRecords[columns].Draw(t, "") + csvFinal.Draw(t, "")
	})
}

var (
	csvField   = pbt.OneOf(csvFieldOf(false), csvFieldOf(true))
	csvColumns = pbt.IntRange(1, 6)
	csvFinal   = pbt.OneOf(pbt.Just(""), pbt.Just("\r\n"))
	// csvHeaders and csvRecords hold, per number of columns, the optional
	// header and the records.
	csvHeaders, csvRecords [7]pbt.Gen[string]
	csvFieldLen            = pbt.IntRange(0, 24)
	// textdata of RFC 4180 §2: printable ASCII and space without the
	// double quote and the comma.
	textdata = strings.ReplaceAll(strings.ReplaceAll(vchar+" ", "\"", ""), ",", "")
)

func init() {
	for n := 1; n <= 6; n++ {
		row := joined(csvField, n, n, ",")
		csvHeaders[n] = optionalText(affix(row, "", "\r\n"))
		csvRecords[n] = joined(row, 1, 6, "\r\n")
	}
}

func csvFieldOf(quoted bool) pbt.Gen[string] {
	set := textdata
	if quoted {
		set += ",\r\n\""
	}
	return pbt.Custom("field", func(t *pbt.T) string {
		n := csvFieldLen.Draw(t, "")
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
