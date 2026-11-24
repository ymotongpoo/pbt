// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package format

import (
	"fmt"
	"strings"

	"go.ymotongpoo.com/pbt"
)

// uuidKinds draws the Nil UUID, versions 1 to 8, or the Max UUID, with
// versions 4 and 7, the random and the time-ordered one, the most common.
var uuidKinds = choice(1, 1, 1, 1, 6, 1, 1, 4, 1, 1)

// UUID generates UUIDs in the text form of RFC 9562 §4, such as
// "f81d4fae-7dec-11d0-a765-00a0c91e6bf6": 32 hexadecimal digits grouped
// 8-4-4-4-12, here always in lower case. The variant bits
// are those of RFC 9562 (10 in binary), the version is in the thirteenth
// digit, and the other bits are drawn, so every value is valid for its
// version: the fields that versions 1, 6 and 7 fill with a timestamp take
// any value. With no arguments UUID draws every version from 1 to 8, with
// 4 and 7 the most common, and in a share of the cases the Nil UUID of
// §5.9 and the Max UUID of §5.10; with arguments it draws only those
// versions, and panics on a version outside 1 to 8. Values shrink toward
// the Nil UUID, or toward the first version given with only zero bits.
func UUID(versions ...int) pbt.Gen[string] {
	for _, v := range versions {
		if v < 1 || v > 8 {
			panic(fmt.Sprintf("format: UUID(%v): versions are 1 to 8", versions))
		}
	}
	kinds := uuidKinds
	if len(versions) > 0 {
		kinds = pbt.SampledFrom(versions)
	}
	bits := pbt.Uint64()
	name := "UUID()"
	if len(versions) > 0 {
		name = strings.ReplaceAll(fmt.Sprintf("UUID(%v)", versions), " ", ", ")
		name = strings.NewReplacer("[", "", "]", "").Replace(name)
	}
	return pbt.Custom(name, func(t *pbt.T) string {
		k := kinds.Draw(t, "")
		hi, lo := bits.Draw(t, ""), bits.Draw(t, "")
		var b [16]byte
		for i := range 8 {
			b[i] = byte(hi >> (56 - 8*i))
			b[8+i] = byte(lo >> (56 - 8*i))
		}
		switch {
		case len(versions) == 0 && k == 0: // Nil
			b = [16]byte{}
		case len(versions) == 0 && k == 9: // Max
			for i := range b {
				b[i] = 0xff
			}
		default:
			b[6] = b[6]&0x0f | byte(k)<<4
			b[8] = b[8]&0x3f | 0x80
		}
		return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
	})
}
