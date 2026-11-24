// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package format

import (
	"fmt"
	"net/url"
	"strings"
	"testing"
	"unicode/utf8"

	"go.ymotongpoo.com/pbt"
)

// decodePuny is a test-only decoder, using insertion rather than the
// encoder's scan of sorted code points. It shares no production helpers.
func decodePuny(s string) (string, error) {
	var result []rune
	if i := strings.LastIndexByte(s, '-'); i >= 0 {
		result = []rune(s[:i])
		s = s[i+1:]
	}
	position, code, bias := int64(0), int64(128), int64(72)
	for s != "" {
		previous, weight := position, int64(1)
		for k := int64(36); ; k += 36 {
			if s == "" {
				return "", fmt.Errorf("truncated integer")
			}
			c := s[0]
			s = s[1:]
			digit := int64(-1)
			switch {
			case c >= 'a' && c <= 'z':
				digit = int64(c - 'a')
			case c >= 'A' && c <= 'Z':
				digit = int64(c - 'A')
			case c >= '0' && c <= '9':
				digit = int64(c-'0') + 26
			}
			if digit < 0 || weight > 1<<40 {
				return "", fmt.Errorf("invalid integer")
			}
			position += digit * weight
			threshold := k - bias
			if threshold < 1 {
				threshold = 1
			}
			if threshold > 26 {
				threshold = 26
			}
			if digit < threshold {
				break
			}
			weight *= 36 - threshold
		}
		delta := position - previous
		divisor := int64(2)
		if previous == 0 {
			divisor = 700
		}
		delta /= divisor
		delta += delta / int64(len(result)+1)
		bias = 0
		for delta > 455 {
			bias += 36
			delta /= 35
		}
		bias += 36 * delta / (delta + 38)
		code += position / int64(len(result)+1)
		position %= int64(len(result) + 1)
		if !utf8.ValidRune(rune(code)) {
			return "", fmt.Errorf("invalid scalar")
		}
		result = append(result, 0)
		copy(result[position+1:], result[position:])
		result[position] = rune(code)
		position++
	}
	return string(result), nil
}

func TestPunycodeRFC3492(t *testing.T) {
	// RFC 3492 §7.1 A-S, without the optional case annotations in I.
	samples := []struct{ u, a string }{
		{"ليهمابتكلموشعربي؟", "egbpdaj6bu4bxfgehfvwxn"},
		{"他们为什么不说中文", "ihqwcrb4cv8a8dqg056pqjye"},
		{"他們爲什麽不說中文", "ihqwctvzc91f659drss3x8bo0yb"},
		{"Pročprostěnemluvíčesky", "Proprostnemluvesky-uyb24dma41a"},
		{"למההםפשוטלאמדבריםעברית", "4dbcagdahymbxekheh6e0a7fei0b"},
		{"यहलोगहिन्दीक्योंनहींबोलसकतेहैं", "i1baa7eci9glrd9b2ae1bj0hfcgg6iyaf8o0a1dig0cd"},
		{"なぜみんな日本語を話してくれないのか", "n8jok5ay5dzabd5bym9f0cm5685rrjetr6pdxa"},
		{"세계의모든사람들이한국어를이해한다면얼마나좋을까", "989aomsvi5e83db1d2a355cv1e0vak1dwrv93d5xbh15a0dt30a5jpsd879ccm6fea98c"},
		{"почемужеонинеговорятпорусски", "b1abfaaepdrnnbgefbadotcwatmq2g4l"},
		{"PorquénopuedensimplementehablarenEspañol", "PorqunopuedensimplementehablarenEspaol-fmd56a"},
		{"TạisaohọkhôngthểchỉnóitiếngViệt", "TisaohkhngthchnitingVit-kjcr8268qyxafd2f1b9g"},
		{"3年B組金八先生", "3B-ww4c5e180e575a65lsy2b"},
		{"安室奈美恵-with-SUPER-MONKEYS", "-with-SUPER-MONKEYS-pc58ag80a8qai00g7n9n"},
		{"Hello-Another-Way-それぞれの場所", "Hello-Another-Way--fc4qua05auwb3674vfr0b"},
		{"ひとつ屋根の下2", "2-u9tlzr9756bt3uc0v"},
		{"MajiでKoiする5秒前", "MajiKoi5-783gue6qz075azm5e"},
		{"パフィーdeルンバ", "de-jg4avhby1noc0d"},
		{"そのスピードで", "d9juau41awczczp"},
		{"-> $1.00 <-", "-> $1.00 <--"}}
	for _, tc := range samples {
		if got := punycode(tc.u); got != tc.a {
			t.Errorf("encode %q: %s, want %s", tc.u, got, tc.a)
		}
		if got, err := decodePuny(tc.a); err != nil || got != tc.u {
			t.Errorf("decode %s: %q %v", tc.a, got, err)
		}
	}
}

// The checker spells out the subset independently of idnaLetters.
func checkULabel(s string) string {
	if s == "" || !utf8.ValidString(s) {
		return "empty or invalid UTF-8"
	}
	nonASCII := false
	for _, r := range s {
		if r > 127 {
			nonASCII = true
			switch r {
			case '\u00e9', '\u00f6', '\u00fc', '\u03b1', '\u03b2', '\u03b3', '\u0430', '\u0431', '\u0432', '\u3042', '\u3044', '\u3046', '\u30a2', '\u30a4', '\u30a6', '\u4e00', '\u4e01', '\u4e03':
			default:
				return "outside PVALID subset"
			}
		} else if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9') {
			return "invalid ASCII"
		}
	}
	if !nonASCII {
		return "no non-ASCII letter"
	}
	return ""
}

func checkIDNAName(s string, ascii bool) string {
	if len(s) > 253 {
		return "wire length"
	}
	encodedLength := 0
	for i, label := range strings.Split(s, ".") {
		u := label
		if ascii {
			if len(label) > 63 || !strings.HasPrefix(label, "xn--") {
				return "A-label"
			}
			var err error
			u, err = decodePuny(label[4:])
			if err != nil {
				return err.Error()
			}
		}
		if msg := checkULabel(u); msg != "" {
			return msg
		}
		a := "xn--" + punycode(u)
		// Decoding independently verifies the generated encoding as well as
		// both representations' DNS octet limits.
		round, err := decodePuny(a[4:])
		if err != nil || round != u {
			return "round trip"
		}
		if len(a) > 63 {
			return "label DNS length"
		}
		if ascii && label != a {
			return "noncanonical A-label"
		}
		if i > 0 {
			encodedLength++
		}
		encodedLength += len(a)
	}
	if encodedLength > 253 {
		return "DNS length"
	}
	return ""
}

func TestIDNAHostnames(t *testing.T) {
	for _, tc := range []struct {
		g      pbt.Gen[string]
		ascii  bool
		target string
	}{
		{IDNAHostname(), true, "xn--9ca"}, {UnicodeHostname(), false, "é"},
	} {
		t.Run(tc.target, func(t *testing.T) {
			checkValid(t, tc.g, func(s string) string {
				if msg := checkIDNAName(s, tc.ascii); msg != "" {
					return msg
				}
				u, err := url.Parse("https://" + s + "/")
				if err != nil || u.Hostname() != s {
					return "net/url changed host"
				}
				return ""
			})
			shrinksTo(t, tc.g, always[string], tc.target)
		})
	}
}

func TestIDNACoverage(t *testing.T) {
	g := UnicodeHostname()
	pbt.Check(t, func(pt *pbt.T) {
		s := g.Draw(pt, "")
		pt.Cover(strings.Contains(s, "."), 30, "multiple labels")
		pt.Cover(!strings.Contains(s, "."), 5, "single label")
		for _, set := range []string{"éöü", "αβγ", "абв", "あいう", "アイウ", "一丁七"} {
			pt.Cover(strings.ContainsAny(s, set), 5, set)
		}
		pt.Cover(strings.ContainsAny(s, "abcdefghijklmnopqrstuvwxyz0123456789"), 30, "mixed ASCII and Unicode")
	}, pbt.Seed(1), pbt.Checks(3000), pbt.CheckCoverage(), pbt.NoSave())
}

func TestIDNALengthBoundaries(t *testing.T) {
	u := strings.Repeat("é", 57)
	if len("xn--"+punycode(u)) != 63 {
		t.Fatal("63-octet fixture")
	}
	// DNS length, rather than rune count or U-label octets, is decisive.
	for _, ascii := range []bool{true, false} {
		s := fitIDNALabels([]string{u, u, u, u, u}, ascii, 253)
		if msg := checkIDNAName(s, ascii); msg != "" {
			t.Fatal(msg)
		}
	}
	labels := []string{u, u, u, strings.Repeat("é", 55)}
	s := fitIDNALabels(labels, true, 253)
	if len(s) != 253 {
		t.Fatalf("max name: %d", len(s))
	}
}

func TestPunycodeScalarRoundTrip(t *testing.T) {
	// Exercise the algorithm beyond the deliberately small IDNA subset,
	// including ASCII, supplementary planes and repeated code points.
	g := pbt.StringOfN(pbt.Rune(), 0, 57)
	checkValid(t, g, func(s string) string {
		got, err := decodePuny(punycode(s))
		if err != nil || got != s {
			return "Punycode scalar round trip"
		}
		return ""
	})
}
