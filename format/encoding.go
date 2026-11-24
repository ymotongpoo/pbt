// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package format

import (
	"encoding/base32"
	"encoding/base64"
	"encoding/hex"
	"go.ymotongpoo.com/pbt"
	"strings"
)

// encodedBytes draws up to 96 bytes before applying a canonical encoder.
func encodedBytes(name string, encode func([]byte) string) pbt.Gen[string] {
	raw := pbt.SliceOfN(pbt.Byte(), 0, 96)
	return pbt.Custom(name+"()", func(t *pbt.T) string { return encode(raw.Draw(t, "")) })
}

// Base64 generates the padded base64 encoding of RFC 4648 §4, with zero
// pad bits (§3.5) and no line breaks. Inputs have 0 to 96 bytes; lengths
// grow with size. Values shrink toward the empty string.
func Base64() pbt.Gen[string] { return encodedBytes("Base64", base64.StdEncoding.EncodeToString) }

// Base64URL generates padded base64url of RFC 4648 §5, with zero pad bits
// and no line breaks, from 0 to 96 bytes. Values shrink toward "".
func Base64URL() pbt.Gen[string] { return encodedBytes("Base64URL", base64.URLEncoding.EncodeToString) }

// RawBase64 generates base64 of RFC 4648 §4 without padding, for protocols
// that explicitly allow its omission (§3.2). Inputs have 0 to 96 bytes,
// with zero pad bits and no line breaks. Values shrink toward "".
func RawBase64() pbt.Gen[string] {
	return encodedBytes("RawBase64", base64.RawStdEncoding.EncodeToString)
}

// RawBase64URL generates unpadded base64url of RFC 4648 §5 and §3.2 from
// 0 to 96 bytes, with zero pad bits and no line breaks. Values shrink to "".
func RawBase64URL() pbt.Gen[string] {
	return encodedBytes("RawBase64URL", base64.RawURLEncoding.EncodeToString)
}

// Base32 generates padded, upper-case base32 of RFC 4648 §6, with zero
// pad bits and no line breaks, from 0 to 96 bytes. Values shrink to "".
func Base32() pbt.Gen[string] { return encodedBytes("Base32", base32.StdEncoding.EncodeToString) }

// Base32Hex generates padded, upper-case base32hex of RFC 4648 §7, with
// zero pad bits and no line breaks, from 0 to 96 bytes. Values shrink to "".
func Base32Hex() pbt.Gen[string] { return encodedBytes("Base32Hex", base32.HexEncoding.EncodeToString) }

// RawBase32 generates base32 of RFC 4648 §6 without padding, for protocols
// allowing omission under §3.2, from 0 to 96 bytes. Values shrink to "".
func RawBase32() pbt.Gen[string] {
	return encodedBytes("RawBase32", base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString)
}

// RawBase32Hex generates base32hex of RFC 4648 §7 without padding, for
// protocols allowing omission under §3.2, from 0 to 96 bytes. Values shrink to "".
func RawBase32Hex() pbt.Gen[string] {
	return encodedBytes("RawBase32Hex", base32.HexEncoding.WithPadding(base32.NoPadding).EncodeToString)
}

// Base16 generates the upper-case base16 encoding of RFC 4648 §8, from
// 0 to 96 bytes. Table 5 specifies upper-case letters; §8 calls the encoding
// case-insensitive, so Hex also exercises lower-case decoders. No padding
// is needed. Values shrink toward "".
func Base16() pbt.Gen[string] {
	return encodedBytes("Base16", func(b []byte) string { return strings.ToUpper(hex.EncodeToString(b)) })
}

// Hex generates the common lower-case hexadecimal encoding, accepted by
// encoding/hex and the case-insensitive base16 of RFC 4648 §8, from 0 to
// 96 bytes. Values shrink toward "".
func Hex() pbt.Gen[string] { return encodedBytes("Hex", hex.EncodeToString) }
