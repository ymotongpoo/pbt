// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package format

import (
	"strings"

	"go.ymotongpoo.com/pbt"
)

var grandfatheredTags = []string{
	"i-ami", "en-GB-oed", "i-bnn", "i-default", "i-enochian", "i-hak", "i-klingon", "i-lux", "i-mingo", "i-navajo", "i-pwn", "i-tao", "i-tay", "i-tsu", "sgn-BE-FR", "sgn-BE-NL", "sgn-CH-DE",
	"art-lojban", "cel-gaulish", "no-bok", "no-nyn", "zh-guoyu", "zh-hakka", "zh-min", "zh-min-nan", "zh-xiang",
}

// RFC5646LanguageTag generates Language-Tag of RFC 5646 §2.1 (BCP 47).
// Half are langtags, a quarter private-use tags, and a quarter one of all
// 26 grandfathered tags (§2.2.8). Every letter independently takes either
// case (apart from the minimal "aa", selected in 1% of draws). Langtags cover all language lengths, up to three extlangs, optional
// script and region (each present in half), up to four distinct variants,
// up to four distinct extension singletons with one to four subtags each,
// and optional private use with one to four subtags. Unbounded repetitions
// are capped at four. These tags are well-formed (§2.2.9), not necessarily
// valid: subtags and prefixes are not checked against the IANA registry.
// Values shrink toward "aa"; private-use tags shrink toward "x-a", and
// grandfathered tags toward "i-ami" when those forms are retained.
func RFC5646LanguageTag() pbt.Gen[string] { return languageTag(false) }

// LanguageTag generates the same well-formed tags as RFC5646LanguageTag,
// with the case conventions of RFC 5646 §2.1.1: lower case except title-case
// scripts and upper-case alphabetic regions. This is case formatting, not
// registry-based canonicalization (§4.5) or validation. The same bounds
// and frequencies apply. Shrink targets are "aa", "x-a", and "i-ami".
func LanguageTag() pbt.Gen[string] { return languageTag(true) }

func languageTag(canonical bool) pbt.Gen[string] {
	name := "RFC5646LanguageTag()"
	if canonical {
		name = "LanguageTag()"
	}
	forms := pbt.Frequency(
		pbt.W(1, pbt.Just("aa")),
		pbt.W(49, casedTag(langtag, canonical)),
		pbt.W(25, casedTag(affix(joined(stringIn(lower+digits, 1, 8), 1, 4, "-"), "x-", ""), canonical)),
		pbt.W(25, casedTag(pbt.SampledFrom(grandfatheredTags), canonical)),
	)
	return pbt.Custom(name, func(t *pbt.T) string { return forms.Draw(t, "") })
}
func casedTag(g pbt.Gen[string], canonical bool) pbt.Gen[string] {
	return pbt.Custom("cased tag", func(t *pbt.T) string {
		parts := strings.Split(strings.ToLower(g.Draw(t, "")), "-")
		if canonical {
			afterSingleton := len(parts[0]) == 1
			for i := 1; i < len(parts); i++ {
				if len(parts[i]) == 1 {
					afterSingleton = true
				}
				if !afterSingleton {
					switch len(parts[i]) {
					case 2:
						parts[i] = strings.ToUpper(parts[i])
					case 4:
						parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
					}
				}
			}
			return strings.Join(parts, "-")
		}
		b := []byte(strings.Join(parts, "-"))
		for i, c := range b {
			up := pbt.Bool().Draw(t, "")
			if c >= 'a' && c <= 'z' && up {
				b[i] = c - 'a' + 'A'
			}
		}
		return string(b)
	})
}

var (
	languageGen = pbt.OneOf(
		pbt.Custom("short language", func(t *pbt.T) string {
			s := stringIn(lower, 2, 3).Draw(t, "")
			for _, e := range extlangs.Draw(t, "") {
				s += "-" + e
			}
			return s
		}), stringIn(lower, 4, 8))
	extlangs      = pbt.SliceOfN(stringIn(lower, 3, 3), 0, 3)
	scriptGen     = optionalText(affix(stringIn(lower, 4, 4), "-", ""))
	regionGen     = optionalText(affix(pbt.OneOf(stringIn(lower, 2, 2), stringIn(digits, 3, 3)), "-", ""))
	variantList   = pbt.SliceOfN(languageVariant, 0, 4)
	extensionList = pbt.SliceOfN(languageExtension, 0, 4)
	privateUse    = optionalText(affix(joined(stringIn(lower+digits, 1, 8), 1, 4, "-"), "-x-", ""))
	extSubtags    = joined(stringIn(lower+digits, 2, 8), 1, 4, "-")
	variantLen    = pbt.IntRange(4, 8)
)

var langtag = pbt.Custom("langtag", func(t *pbt.T) string {
	language := languageGen.Draw(t, "")
	script := scriptGen.Draw(t, "")
	region := regionGen.Draw(t, "")
	variants := variantList.Draw(t, "")
	used := map[string]bool{}
	for i, v := range variants {
		for used[v] {
			last := strings.IndexByte(lower+digits, v[len(v)-1])
			v = v[:len(v)-1] + string((lower + digits)[(last+1)%36])
		}
		used[v] = true
		variants[i] = v
	}
	extensions := extensionList.Draw(t, "")
	const singletons = "abcdefghijklmnopqrstuvwyz0123456789"
	used = map[string]bool{}
	for i, v := range extensions {
		for used[v[:1]] {
			j := strings.IndexByte(singletons, v[0])
			v = string(singletons[(j+1)%len(singletons)]) + v[1:]
		}
		used[v[:1]] = true
		extensions[i] = v
	}
	s := language + script + region
	for _, v := range variants {
		s += "-" + v
	}
	for _, v := range extensions {
		s += "-" + v
	}
	return s + privateUse.Draw(t, "")
})
var languageExtension = pbt.Custom("extension", func(t *pbt.T) string {
	return string(charIn("abcdefghijklmnopqrstuvwyz0123456789").Draw(t, "")) + "-" + extSubtags.Draw(t, "")
})

var languageVariant = pbt.Custom("variant", func(t *pbt.T) string {
	n := variantLen.Draw(t, "")
	b := make([]byte, n)
	for i := range b {
		set := lower + digits
		if i == 0 && n == 4 {
			set = digits
		}
		b[i] = charIn(set).Draw(t, "")
	}
	return string(b)
})
