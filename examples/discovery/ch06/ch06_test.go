// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package ch06

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"go.ymotongpoo.com/pbt"
	"go.ymotongpoo.com/pbt/examples/discovery/internal/mutant"
)

var opt = []pbt.Option{pbt.NoSave(), pbt.NoReplay()}

// ---- 6.3 固定の観測

func EncodeIDs(ids []int) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = strconv.Itoa(id)
	}
	return strings.Join(parts, ",")
}

func DecodeIDs(s string) ([]int, error) {
	if s == "" {
		return nil, nil
	}
	parts := strings.Split(s, ",")
	out := make([]int, len(parts))
	for i, p := range parts {
		v, err := strconv.Atoi(p)
		if err != nil {
			return nil, err
		}
		out[i] = v
	}
	return out, nil
}

type codec struct {
	enc func([]int) string
	dec func(string) ([]int, error)
}

func encSemi(ids []int) string { return strings.ReplaceAll(EncodeIDs(ids), ",", ";") }
func decSemi(s string) ([]int, error) {
	return DecodeIDs(strings.ReplaceAll(s, ";", ","))
}
func encRev(ids []int) string {
	c := slices.Clone(ids)
	slices.Reverse(c)
	return EncodeIDs(c)
}
func decRev(s string) ([]int, error) {
	x, err := DecodeIDs(s)
	slices.Reverse(x)
	return x, err
}

var (
	semicolonPair = codec{encSemi, decSemi}
	reversedPair  = codec{encRev, decRev}
)

func roundTrip(c codec) pbt.Property {
	return func(t *pbt.T) {
		ids := pbt.SliceOf(pbt.IntRange(-1000, 1000)).Draw(t, "ids")
		got, err := c.dec(c.enc(ids))
		if err != nil || !slices.Equal(got, ids) {
			t.Fatalf("Decode(Encode(%v)) = %v, %v", ids, got, err)
		}
	}
}

func commaCount(c codec) pbt.Property {
	return func(t *pbt.T) {
		ids := pbt.SliceOfN(pbt.IntRange(-1000, 1000), 1, -1).Draw(t, "ids")
		s := c.enc(ids)
		if got := strings.Count(s, ","); got != len(ids)-1 {
			t.Fatalf("%q has %d commas, want %d", s, got, len(ids)-1)
		}
	}
}

func TestRoundTripSurvivesPairs(t *testing.T) {
	pbt.Check(t, roundTrip(semicolonPair), opt...)
	pbt.Check(t, roundTrip(reversedPair), opt...)
}
func TestCommaCount(t *testing.T) { pbt.Check(t, commaCount(codec{EncodeIDs, DecodeIDs}), opt...) }
func TestCommaCountSemicolonMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, commaCount(semicolonPair), opt...)
}
func TestCommaCountReversedSurvives(t *testing.T) { pbt.Check(t, commaCount(reversedPair), opt...) }
func TestEncodeExample(t *testing.T) {
	if got := EncodeIDs([]int{1, 2, 3}); got != "1,2,3" {
		t.Fatalf("EncodeIDs = %q", got)
	}
}
func TestEncodeExampleSemicolonMutant(t *testing.T) {
	mutant.Only(t)
	if got := semicolonPair.enc([]int{1, 2, 3}); got != "1,2,3" {
		t.Fatalf("EncodeIDs = %q", got)
	}
}
func TestEncodeExampleReversedMutant(t *testing.T) {
	mutant.Only(t)
	if got := reversedPair.enc([]int{1, 2, 3}); got != "1,2,3" {
		t.Fatalf("EncodeIDs = %q", got)
	}
}

// ---- 6.4 JSON

type Profile struct {
	Name string   `json:"name"`
	Tags []string `json:"tags,omitempty"`
}

var profileGen = pbt.Custom("profile", func(t *pbt.T) Profile {
	p := Profile{Name: pbt.String().Draw(t, "name")}
	if pbt.Bool().Draw(t, "hasTags") {
		p.Tags = pbt.SliceOf(pbt.String()).Draw(t, "tags")
	}
	return p
})

func TestProfileDeepEqualMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, func(t *pbt.T) {
		p := profileGen.Draw(t, "p")
		b, err := json.Marshal(p)
		if err != nil {
			t.Fatal(err)
		}
		var q Profile
		if err := json.Unmarshal(b, &q); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(p, q) {
			t.Fatalf("round trip: %#v became %#v via %s", p, q, b)
		}
	}, opt...)
}

// Canon は保存形式が区別しない違いを消した正規形を返す。
func (p Profile) Canon() Profile {
	if len(p.Tags) == 0 {
		p.Tags = nil
	}
	return p
}

func TestProfileRoundTrip(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		p := profileGen.Draw(t, "p")
		b, err := json.Marshal(p)
		if err != nil {
			t.Fatal(err)
		}
		var q Profile
		if err := json.Unmarshal(b, &q); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(p.Canon(), q) {
			t.Fatalf("round trip: %#v became %#v via %s", p, q, b)
		}
	})
}

func TestProfileInvalidUTF8Mutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, func(t *pbt.T) {
		p := Profile{Name: string(pbt.Bytes().Draw(t, "name"))}
		b, err := json.Marshal(p)
		if err != nil {
			t.Fatal(err)
		}
		var q Profile
		if err := json.Unmarshal(b, &q); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(p.Canon(), q) {
			t.Fatalf("round trip: %#v became %#v via %s", p, q, b)
		}
	}, opt...)
}

// omitempty を外した形：空と nil を形式で区別する
type StrictProfile struct {
	Name string   `json:"name"`
	Tags []string `json:"tags"`
}

func TestStrictProfileRoundTrip(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		o := profileGen.Draw(t, "p")
		p := StrictProfile{o.Name, o.Tags}
		b, err := json.Marshal(p)
		if err != nil {
			t.Fatal(err)
		}
		var q StrictProfile
		if err := json.Unmarshal(b, &q); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(p, q) {
			t.Fatalf("round trip: %#v became %#v via %s", p, q, b)
		}
	})
}

// ---- 6.5 RLE

// RLEEncode は連続する同じバイトを「長さ、バイト」の2バイトの組にする。
// 1組の長さは1バイトに収まる255までで、それより長い連続は組を分ける。
func RLEEncode(b []byte) []byte {
	var out []byte
	for i := 0; i < len(b); {
		j := i
		for j < len(b) && b[j] == b[i] && j-i < 255 {
			j++
		}
		out = append(out, byte(j-i), b[i])
		i = j
	}
	return out
}

// RLEDecode は RLEEncode の逆で、奇数長の入力にはエラーを返す。
func RLEDecode(b []byte) ([]byte, error) {
	if len(b)%2 != 0 {
		return nil, errors.New("rle: odd length")
	}
	var out []byte
	for i := 0; i < len(b); i += 2 {
		out = append(out, bytes.Repeat([]byte{b[i+1]}, int(b[i]))...)
	}
	return out, nil
}

// 変異：組を分ける処理を忘れ、長さが byte に切り詰められる
func rleEncodeNoLimit(b []byte) []byte {
	var out []byte
	for i := 0; i < len(b); {
		j := i
		for j < len(b) && b[j] == b[i] {
			j++
		}
		out = append(out, byte(j-i), b[i])
		i = j
	}
	return out
}

// maxRun は b の中で同じバイトが続く最長の長さを返す。
func maxRun(b []byte) int {
	best, cur := 0, 0
	for i := range b {
		if i > 0 && b[i] == b[i-1] {
			cur++
		} else {
			cur = 1
		}
		best = max(best, cur)
	}
	return best
}

// 連続を狙う生成器：「バイトと長さ」の組をつないで入力を作る
var runsGen = pbt.Custom("runs", func(t *pbt.T) []byte {
	var out []byte
	for range pbt.IntRange(0, 4).Draw(t, "count") {
		c := pbt.Byte().Draw(t, "c")
		n := pbt.IntRange(1, 600).Draw(t, "n")
		out = append(out, bytes.Repeat([]byte{c}, n)...)
	}
	return out
})

func rleRound(enc func([]byte) []byte, gen pbt.Gen[[]byte]) pbt.Property {
	return func(t *pbt.T) {
		b := gen.Draw(t, "b")
		t.Classify(maxRun(b) > 255, "run > 255")
		t.Measure("maxRun", float64(maxRun(b)))
		got, err := RLEDecode(enc(b))
		if err != nil || !bytes.Equal(got, b) {
			t.Fatalf("round trip changed %d bytes into %d bytes (err %v)", len(b), len(got), err)
		}
	}
}

func TestRLEBytes(t *testing.T) { pbt.Check(t, rleRound(RLEEncode, pbt.Bytes())) }
func TestRLERuns(t *testing.T)  { pbt.Check(t, rleRound(RLEEncode, runsGen)) }
func TestRLEBytesSurvives(t *testing.T) {
	pbt.Check(t, rleRound(rleEncodeNoLimit, pbt.Bytes()), opt...)
}
func TestRLEBytesLargerChecksSurvives(t *testing.T) {
	pbt.Check(t, rleRound(rleEncodeNoLimit, pbt.Bytes()), append(opt, pbt.Checks(5000))...)
}
func TestRLERunsMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, rleRound(rleEncodeNoLimit, runsGen), opt...)
}
func TestRLERunsStats(t *testing.T) {
	pbt.Check(t, rleRound(RLEEncode, runsGen), opt...)
}

// 上限：符号化は最悪でも2倍に収まる
func TestRLEBound(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		b := pbt.Bytes().Draw(t, "b")
		if got := len(RLEEncode(b)); got > 2*len(b) {
			t.Fatalf("len(encoded) = %d for %d bytes", got, len(b))
		}
	})
}

// 方向2：正規化の冪等性
func TestRLECanon(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		n := pbt.IntRange(0, 6).Draw(t, "pairs")
		s := pbt.BytesN(2*n, 2*n).Draw(t, "s")
		dec, err := RLEDecode(s)
		if err != nil {
			t.Fatal(err)
		}
		once := RLEEncode(dec)
		dec2, err := RLEDecode(once)
		if err != nil {
			t.Fatal(err)
		}
		if twice := RLEEncode(dec2); !bytes.Equal(once, twice) {
			t.Fatalf("not idempotent: %v -> %v -> %v", s, once, twice)
		}
	})
}

func TestRLEDirection2Mutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, func(t *pbt.T) {
		n := pbt.IntRange(0, 6).Draw(t, "pairs")
		s := pbt.BytesN(2*n, 2*n).Draw(t, "s")
		dec, err := RLEDecode(s)
		if err != nil {
			t.Fatal(err)
		}
		if got := RLEEncode(dec); !bytes.Equal(got, s) {
			t.Fatalf("Encode(Decode(%v)) = %v", s, got)
		}
	}, opt...)
}

func TestRLECanonExamples(t *testing.T) {
	for _, c := range []struct{ s, want []byte }{
		{[]byte{0, 0}, nil},
		{[]byte{1, 'a', 1, 'a'}, []byte{2, 'a'}},
	} {
		dec, err := RLEDecode(c.s)
		if err != nil {
			t.Fatal(err)
		}
		if got := RLEEncode(dec); !bytes.Equal(got, c.want) {
			t.Fatalf("Encode(Decode(%v)) = %v, want %v", c.s, got, c.want)
		}
	}
}

func TestRLEBytesLen(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		b := pbt.Bytes().Draw(t, "b")
		t.Measure("len", float64(len(b)))
	}, pbt.Checks(5000))
}

func TestRLEExample(t *testing.T) {
	if got := RLEEncode([]byte("aaab")); !bytes.Equal(got, []byte{3, 'a', 1, 'b'}) {
		t.Fatalf("RLEEncode = %v", got)
	}
}

// ---- 6.6 URL のパス

// BuildPath はセグメントの列を "/a/b" の形のパスにする。
func BuildPath(segs []string) string {
	var b strings.Builder
	for _, s := range segs {
		b.WriteByte('/')
		b.WriteString(url.PathEscape(s))
	}
	return b.String()
}

// ParsePath は BuildPath の逆で、"/" で始まらないパスにはエラーを返す。
func ParsePath(p string) ([]string, error) {
	if p == "" {
		return nil, nil
	}
	if p[0] != '/' {
		return nil, fmt.Errorf("path %q does not start with /", p)
	}
	parts := strings.Split(p[1:], "/")
	out := make([]string, len(parts))
	for i, part := range parts {
		s, err := url.PathUnescape(part)
		if err != nil {
			return nil, err
		}
		out[i] = s
	}
	return out, nil
}

// 変異：セグメントの中の / をエスケープし忘れる
func buildPathSlash(segs []string) string {
	var b strings.Builder
	for _, s := range segs {
		b.WriteByte('/')
		b.WriteString(strings.ReplaceAll(url.PathEscape(s), "%2F", "/"))
	}
	return b.String()
}

var segGen = pbt.StringOf(pbt.SampledFrom([]rune("ab/%? .")))

func pathRound(build func([]string) string, seg pbt.Gen[string]) pbt.Property {
	return func(t *pbt.T) {
		segs := pbt.SliceOf(seg).Draw(t, "segs")
		t.Classify(slices.ContainsFunc(segs, func(s string) bool { return strings.Contains(s, "/") }), "has slash")
		got, err := ParsePath(build(segs))
		if err != nil || !slices.Equal(got, segs) {
			t.Fatalf("Parse(Build(%q)) = %q, %v via %q", segs, got, err, build(segs))
		}
	}
}

func TestPathRound(t *testing.T)       { pbt.Check(t, pathRound(BuildPath, segGen)) }
func TestPathRoundString(t *testing.T) { pbt.Check(t, pathRound(BuildPath, pbt.String())) }
func TestPathRoundBytes(t *testing.T) {
	pbt.Check(t, pathRound(BuildPath, pbt.Map(pbt.Bytes(), func(b []byte) string { return string(b) })))
}
func TestPathSlashMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, pathRound(buildPathSlash, segGen), opt...)
}
func TestPathSlashStringView(t *testing.T) {
	pbt.Check(t, pathRound(BuildPath, pbt.String()), opt...)
}
func TestPathSlashStringMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, pathRound(buildPathSlash, pbt.String()), opt...)
}

// 途中に path.Clean で整える経路が入る場合
func cleanRound(seg pbt.Gen[string]) pbt.Property {
	return func(t *pbt.T) {
		segs := pbt.SliceOfN(seg, 1, -1).Draw(t, "segs")
		got, err := ParsePath(path.Clean(BuildPath(segs)))
		if err != nil || !slices.Equal(got, segs) {
			t.Fatalf("Parse(Clean(Build(%q))) = %q, %v", segs, got, err)
		}
	}
}

// 空のセグメントを持たない値に限る仕様
var nonEmptySeg = pbt.StringOfN(pbt.SampledFrom([]rune("ab/%? .")), 1, -1)

func TestPathCleanEmptyMutant(t *testing.T) { mutant.Only(t); pbt.Check(t, cleanRound(segGen), opt...) }
func TestPathCleanNonEmptyMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, cleanRound(nonEmptySeg), opt...)
}

// 方向2
var pathText = pbt.StringOf(pbt.SampledFrom([]rune("ab/%2Fz ")))

func TestPathDirection2Mutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, func(t *pbt.T) {
		p := "/" + pathText.Draw(t, "p")
		segs, err := ParsePath(p)
		t.Assume(err == nil)
		if got := BuildPath(segs); got != p {
			t.Fatalf("Build(Parse(%q)) = %q", p, got)
		}
	}, opt...)
}

func canonPath(p string) (string, error) {
	segs, err := ParsePath(p)
	if err != nil {
		return "", err
	}
	return BuildPath(segs), nil
}

func TestPathCanonIdempotent(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		p := "/" + pathText.Draw(t, "p")
		once, err := canonPath(p)
		t.Assume(err == nil)
		twice, err := canonPath(once)
		if err != nil || twice != once {
			t.Fatalf("canon(%q) = %q, then %q, %v", p, once, twice, err)
		}
	})
}

func TestPathCanonExamples(t *testing.T) {
	for in, want := range map[string]string{"/ ": "/%20", "/%41": "/A", "/a%2fb": "/a%2Fb"} {
		if got, err := canonPath(in); err != nil || got != want {
			t.Fatalf("canon(%q) = %q, %v, want %q", in, got, err, want)
		}
	}
	if got, err := ParsePath("/a//b"); err != nil || !slices.Equal(got, []string{"a", "", "b"}) {
		t.Fatalf("ParsePath = %q, %v", got, err)
	}
}

func TestPathExample(t *testing.T) {
	if got := BuildPath([]string{"a/b", "c d"}); got != "/a%2Fb/c%20d" {
		t.Fatalf("BuildPath = %q", got)
	}
	t.Logf("%q", BuildPath([]string{"..", ".", ""}))
}

func TestRLERunsStatsBySeed(t *testing.T) {
	for seed := uint64(1); seed <= 3; seed++ {
		pbt.Check(t, rleRound(RLEEncode, runsGen), pbt.Checks(5000), pbt.Seed(seed), pbt.Verbose(), pbt.NoSave(), pbt.NoReplay())
	}
}
