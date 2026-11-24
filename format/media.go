// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package format

import (
	"strings"

	"go.ymotongpoo.com/pbt"
)

const httpTchar = alnum + "!#$%&'*+-.^_`|~"

var obsBytes = func() string {
	b := make([]byte, 128)
	for i := range b {
		b[i] = byte(i + 128)
	}
	return string(b)
}()
var httpToken = stringIn(httpTchar, 1, 32)

// RFC9110MediaType generates media-type of RFC 9110 §8.3.1: token "/"
// token followed by parameters of §5.6.6, including empty parameter slots,
// OWS around semicolons and token or quoted-string values (§5.6.4).
// Tokens have 1 to 32 bytes, quoted values 0 to 16 characters and there
// are 0 to 4 parameter slots. Each slot is empty with probability 1/4;
// nonempty slots choose token and quoted values equally. Quoted characters
// may be obs-text, including invalid UTF-8; a quarter use quoted-pair.
// Names may repeat and may use RFC 2231-looking syntax with no special
// meaning here. Values shrink toward "a/a".
//
// mime.ParseMediaType rejects internal empty slots and differing duplicate
// parameters, and interprets RFC 2231 parameter suffixes rather than
// preserving them as HTTP token names. It accepts trailing empty slots,
// and preserves the backslash of a quoted-pair before obs-text.
func RFC9110MediaType() pbt.Gen[string] { return mediaType(false) }

// MediaType generates a common media type with type and subtype following
// restricted-name of RFC 6838 §4.2 (1 to 127 bytes, initially alphanumeric).
// It has 0 to 4 distinct lower-case alphanumeric parameter names of at
// most 32 bytes and token or ASCII quoted values, equally likely. Quoted
// values have at most 16 characters. No registry membership is implied.
// mime.ParseMediaType accepts these strings. Values shrink toward "a/a".
func MediaType() pbt.Gen[string] { return mediaType(true) }

func mediaType(common bool) pbt.Gen[string] {
	name := "RFC9110MediaType()"
	names := httpToken
	if common {
		name = "MediaType()"
		names = restrictedName
	}
	value := pbt.OneOf(httpToken, httpQuoted(!common))
	key := httpToken
	if common {
		key = stringIn(lower+digits, 1, 32)
	}
	param := pbt.Custom("parameter", func(t *pbt.T) string {
		return key.Draw(t, "") + "=" + value.Draw(t, "")
	})
	ows := stringIn(" \t", 0, 3)
	maybeParam := pbt.Frequency(pbt.W(1, pbt.Just("")), pbt.W(3, param))
	slots := joined(pbt.Custom("parameter slot", func(t *pbt.T) string {
		return ows.Draw(t, "") + ";" + ows.Draw(t, "") + maybeParam.Draw(t, "")
	}), 0, 4, "")
	params := pbt.SliceOfN(param, 0, 4)
	return pbt.Custom(name, func(t *pbt.T) string {
		s := names.Draw(t, "") + "/" + names.Draw(t, "")
		if !common {
			return s + slots.Draw(t, "")
		}
		params := params.Draw(t, "")
		used := map[string]bool{}
		for _, p := range params {
			key, value, _ := strings.Cut(p, "=")
			for used[key] {
				j := strings.IndexByte(lower+digits, key[len(key)-1])
				key = key[:len(key)-1] + string((lower + digits)[(j+1)%36])
			}
			used[key] = true
			s += "; " + key + "=" + value
		}
		return s
	})
}

var restrictedLen = pbt.IntRange(1, 127)

var restrictedName = pbt.Custom("restricted-name", func(t *pbt.T) string {
	n := restrictedLen.Draw(t, "")
	b := make([]byte, n)
	for i := range b {
		set := alnum + "!#$&-^_.+"
		if i == 0 {
			set = alnum
		}
		b[i] = charIn(set).Draw(t, "")
	}
	return string(b)
})

func httpQuoted(obs bool) pbt.Gen[string] {
	text, pair := qtextSMTP+"\t", vchar+" \t"
	if obs {
		text += obsBytes
		pair += obsBytes
	}
	literal := pbt.Map(charIn(text), func(b byte) string { return string(b) })
	escaped := pbt.Map(charIn(pair), func(b byte) string { return "\\" + string(b) })
	return affix(joined(pbt.Frequency(pbt.W(3, literal), pbt.W(1, escaped)), 0, 16, ""), "\"", "\"")
}
