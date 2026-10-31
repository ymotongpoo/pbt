package ch06birthday

import (
	"reflect"
	"testing"

	"go.ymotongpoo.com/pbt"
)

// The generators of Section 6.4. Alphanumerics come first in textdata so
// that counterexamples shrink to readable text.
var (
	textdata     = pbt.SampledFrom([]rune("0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ:;<=>?@ !#$%&'()*+-./[\\]^_`{|}~"))
	unquotedText = pbt.StringOf(textdata)
	quotableText = pbt.StringOf(pbt.OneOf(textdata, pbt.SampledFrom([]rune("\r\n\","))))
	field        = pbt.OneOf(unquotedText, quotableText)
)

// csvSource generates the header and the records with one shared size, as
// the nested ?LET of the chapter does. The header has at least two
// columns, which avoids the one-column ambiguity found by the property, and
// its names are distinct because the map representation cannot hold
// duplicates.
var csvSource = pbt.Sized(func(size int) pbt.Gen[[]map[string]string] {
	n := size + 1
	header := pbt.SliceOfNDistinct(field, n, n, func(s string) string { return s })
	return pbt.Bind(header, func(keys []string) pbt.Gen[[]map[string]string] {
		entry := pbt.Map(pbt.SliceOfN(field, n, n), func(values []string) map[string]string {
			row := make(map[string]string, n)
			for i, k := range keys {
				row[k] = values[i]
			}
			return row
		})
		return pbt.SliceOf(entry)
	})
})

func TestRoundTrip(t *testing.T) {
	pbt.Check(t, pbt.ForAll(csvSource, func(t *pbt.T, rows []map[string]string) {
		t.Collect(len(rows) > 0)
		got, err := Decode(Encode(rows))
		if err != nil {
			t.Fatalf("Decode: %v", err)
		}
		if len(got) == 0 && len(rows) == 0 {
			return
		}
		if !reflect.DeepEqual(got, rows) {
			t.Fatalf("round trip changed the rows:\n%#v\nbecame\n%#v", rows, got)
		}
	}))
}

// The counterexample the property found with one column: two empty rows
// encode to "\r\n\r\n", which decodes to a single row. The chapter keeps it
// as a unit test and generates at least two columns.
func TestOneColumnBug(t *testing.T) {
	rows := []map[string]string{{"": ""}, {"": ""}}
	encoded := Encode(rows)
	if encoded != "\r\n\r\n" {
		t.Fatalf("Encode = %q", encoded)
	}
	got, err := Decode(encoded)
	if err != nil || !reflect.DeepEqual(got, []map[string]string{{"": ""}}) {
		t.Errorf("Decode = %#v, %v", got, err)
	}
}

// The RFC 4180 examples the chapter keeps as unit tests next to the property.
func TestRFCExamples(t *testing.T) {
	three := []map[string]string{{"aaa": "zzz", "bbb": "yyy", "ccc": "xxx"}}
	cases := []struct {
		name  string
		input string
		want  []map[string]string
	}{
		{"record per line", "aaa,bbb,ccc\r\nzzz,yyy,xxx\r\n", three},
		{"optional trailing CRLF", "aaa,bbb,ccc\r\nzzz,yyy,xxx", three},
		{"double quotes", "\"aaa\",\"bbb\",\"ccc\"\r\nzzz,yyy,xxx", three},
		{"CRLF escape", "\"aaa\",\"b\r\nbb\",\"ccc\"\r\nzzz,yyy,xxx", []map[string]string{{"aaa": "zzz", "b\r\nbb": "yyy", "ccc": "xxx"}}},
		{"double quote escape", "\"aaa\",\"b\"\"bb\",\"ccc\"\r\n,,", []map[string]string{{"aaa": "", "b\"bb": "", "ccc": ""}}},
		{"duplicate keys unsupported", "field_name,field_name,field_name\r\naaa,bbb,ccc\r\nzzz,yyy,xxx", []map[string]string{{"field_name": "ccc"}, {"field_name": "xxx"}}},
	}
	for _, c := range cases {
		got, err := Decode(c.input)
		if err != nil || !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: Decode(%q) = %#v, %v; want %#v", c.name, c.input, got, err, c.want)
		}
	}
}
