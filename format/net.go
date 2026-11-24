// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package format

import (
	"net/netip"
	"strconv"
	"strings"

	"go.ymotongpoo.com/pbt"
)

// IPv4 generates IPv4 addresses in dotted-decimal form, four decimal
// octets without leading zeros, as netip.Addr.String writes them, for the
// addresses of pbt.IPv4, special-purpose blocks included. Values shrink
// toward "0.0.0.0".
func IPv4() pbt.Gen[string] {
	addrs := pbt.IPv4()
	return pbt.Custom("IPv4()", func(t *pbt.T) string { return addrs.Draw(t, "").String() })
}

// RFC5952IPv6 generates IPv6 addresses in the canonical text form of RFC
// 5952 §4: hexadecimal digits in lower case without leading zeros, the
// longest run of two or more zero groups replaced by "::", the first one
// on a tie, and IPv4-mapped addresses in the mixed notation of §5, such as
// "::ffff:192.0.2.1". That is what netip.Addr.String writes. The addresses
// are those of pbt.IPv6. Values shrink toward "::".
func RFC5952IPv6() pbt.Gen[string] {
	addrs := pbt.IPv6()
	return pbt.Custom("RFC5952IPv6()", func(t *pbt.T) string { return addrs.Draw(t, "").String() })
}

// RFC4291IPv6 generates IPv6 addresses in every text form of RFC 4291
// §2.2, which a parser must accept: groups of one to four hexadecimal
// digits, with or without leading zeros, in lower, upper or mixed case;
// "::" in place of any one run of zero groups, the longest or another, of
// any length down to a single group, or no "::" at all; and the last 32
// bits written as a dotted-decimal IPv4 address, which §2.2 allows for any
// address. The addresses are those of pbt.IPv6, and netip.ParseAddr
// returns the address for every form. Values shrink toward "::".
func RFC4291IPv6() pbt.Gen[string] {
	addrs := pbt.IPv6()
	return pbt.Custom("RFC4291IPv6()", func(t *pbt.T) string { return ipv6Text(t, addrs, 1) })
}

var (
	ipv6Embed    = choice(3, 1)    // hexadecimal only, the last 32 bits in dotted decimal
	ipv6Case     = choice(6, 1, 1) // lower, upper, mixed
	ipv6Compress = pbt.IntRange(0, 9)
	ipv6Sub      = pbt.IntRange(0, 7)
	ipv6Pad      = pbt.IntRange(0, 3)
	mixedCase    = pbt.Uint32()
)

// ipv6Text writes an address of addrs in a text form of RFC 4291 §2.2 in
// which "::" stands for at least minRun groups. The form is drawn before
// the address and always takes the same number of choices, so that the
// shrinker can simplify the address without changing the form.
func ipv6Text(t *pbt.T, addrs pbt.Gen[netip.Addr], minRun int) string {
	embed := ipv6Embed.Draw(t, "") == 1
	letters := ipv6Case.Draw(t, "")
	sel := ipv6Compress.Draw(t, "")
	subStart, subLen := ipv6Sub.Draw(t, ""), ipv6Sub.Draw(t, "")
	var pad [8]int
	for i := range pad {
		pad[i] = ipv6Pad.Draw(t, "")
	}
	mixed := mixedCase.Draw(t, "")
	b := addrs.Draw(t, "").As16()

	nhex := 8
	if embed {
		nhex = 6
	}
	var g [8]uint16
	for i := range g {
		g[i] = uint16(b[2*i])<<8 | uint16(b[2*i+1])
	}
	// The runs of zero groups among the hexadecimal groups.
	var runs [][2]int // start, length
	for i := 0; i < nhex; {
		if g[i] != 0 {
			i++
			continue
		}
		j := i
		for j < nhex && g[j] == 0 {
			j++
		}
		runs = append(runs, [2]int{i, j - i})
		i = j
	}
	cut, cutEnd := -1, -1
	switch sel {
	case 0: // the longest run, as RFC 5952 §4.2.3 picks it
		best := [2]int{-1, 0}
		for _, r := range runs {
			if r[1] > best[1] && r[1] >= max(2, minRun) {
				best = r
			}
		}
		if best[0] >= 0 {
			cut, cutEnd = best[0], best[0]+best[1]
		}
	case 1: // no "::"
	default: // a part of any run long enough
		var long [][2]int
		for _, r := range runs {
			if r[1] >= minRun {
				long = append(long, r)
			}
		}
		if len(long) > 0 {
			r := long[(sel-2)%len(long)]
			off := subStart % (r[1] - minRun + 1)
			n := minRun + subLen%(r[1]-off-minRun+1)
			cut, cutEnd = r[0]+off, r[0]+off+n
		}
	}

	var out strings.Builder
	pos := 0 // index of the next hexadecimal digit, for mixed case
	group := func(i int) {
		hex := strconv.FormatUint(uint64(g[i]), 16)
		if n := min(4, len(hex)+pad[i]); n > len(hex) {
			hex = strings.Repeat("0", n-len(hex)) + hex
		}
		for k := range len(hex) {
			c := hex[k]
			up := letters == 1 || (letters == 2 && mixed>>(pos%32)&1 == 1)
			if up && c >= 'a' {
				c -= 'a' - 'A'
			}
			out.WriteByte(c)
			pos++
		}
	}
	for i := 0; i < nhex; i++ {
		if i == cut {
			out.WriteString("::")
			i = cutEnd - 1
			continue
		}
		if i > 0 && i != cutEnd {
			out.WriteByte(':')
		}
		group(i)
	}
	if embed {
		if cutEnd != nhex {
			out.WriteByte(':')
		}
		out.WriteString(netip.AddrFrom4([4]byte(b[12:])).String())
	}
	return out.String()
}

// CIDR generates IP prefixes in the notation of RFC 4632 §3.1 for IPv4,
// such as "192.0.2.0/24", and of RFC 4291 §2.3 for IPv6, such as
// "2001:db8::/32", with the host bits cleared and the address in canonical
// form, as netip.Prefix.String writes a masked prefix. The prefixes are
// those of pbt.Prefix. Values shrink toward "0.0.0.0/0".
func CIDR() pbt.Gen[string] {
	prefixes := pbt.Prefix()
	return pbt.Custom("CIDR()", func(t *pbt.T) string { return prefixes.Draw(t, "").Masked().String() })
}

var macSep = choice(4, 2, 1) // colons, hyphens, dots

// MAC generates MAC addresses in the text forms net.ParseMAC reads:
// EUI-48 and EUI-64 addresses of pbt.HardwareAddr written as groups of two
// hexadecimal digits separated by colons ("01:23:45:67:89:ab") or hyphens,
// the form IEEE 802 uses ("01-23-45-67-89-AB"), or as groups of four
// separated by dots ("0123.4567.89ab"), in lower, upper or mixed case.
// Values shrink toward "00:00:00:00:00:00".
func MAC() pbt.Gen[string] {
	macs := pbt.HardwareAddr()
	return pbt.Custom("MAC()", func(t *pbt.T) string {
		sep := macSep.Draw(t, "")
		letters := ipv6Case.Draw(t, "")
		mixed := mixedCase.Draw(t, "")
		mac := macs.Draw(t, "")
		var b strings.Builder
		for i, octet := range mac {
			switch {
			case i == 0:
			case sep == 2 && i%2 == 0:
				b.WriteByte('.')
			case sep < 2:
				b.WriteByte(":-"[sep])
			}
			for k, nibble := range [2]byte{octet >> 4, octet & 0xf} {
				up := letters == 1 || (letters == 2 && mixed>>((2*i+k)%32)&1 == 1)
				b.WriteByte(hexDigit(nibble, up))
			}
		}
		return b.String()
	})
}

var (
	hostMode   = choice(15, 1) // labels as drawn, labels of 63 characters up to 253 in total
	labelCount = pbt.IntRange(0, 4)
	labelLen   = pbt.IntRange(1, 63)
)

// Hostname generates host names in the syntax of RFC 1123 §2.1: labels of
// one to 63 letters, digits and hyphens that neither start nor end with a
// hyphen, joined by dots, at most 253 characters, the longest name DNS
// carries without a final dot (RFC 1035 §2.3.4). Labels may start with a
// digit, which RFC 1123 allows and RFC 1035 does not, but the last label
// starts with a letter, so that a name never looks like an IPv4 address.
// Letters are in both cases. Labels of 63 characters are common, and about
// one name in sixteen is exactly 253 characters long. There is no final
// dot. Values shrink toward "a".
func Hostname() pbt.Gen[string] {
	return pbt.Custom("Hostname()", func(t *pbt.T) string { return hostname(t, alnum, 253) })
}

// RFC1035Domain generates domain names in the preferred name syntax of RFC
// 1035 §2.3.1, which is stricter than Hostname: every label starts with a
// letter and ends with a letter or a digit, with letters, digits and
// hyphens between, one to 63 characters, and the name has at most 253
// characters. Values shrink toward "a".
func RFC1035Domain() pbt.Gen[string] {
	return pbt.Custom("RFC1035Domain()", func(t *pbt.T) string { return hostname(t, alpha, 253) })
}

// hostname draws a name whose labels start with a character of first, the
// last one with a letter, in at most budget characters, which must be at
// least 1.
func hostname(t *pbt.T, first string, budget int) string {
	long := hostMode.Draw(t, "") == 1
	n := labelCount.Draw(t, "")
	var labels []string
	if long {
		for i, l := range []int{63, 63, 63, 61} {
			set := first
			if i == 3 {
				set = alpha
			}
			labels = append(labels, ldhLabel(t, set, l))
		}
	} else {
		for range n {
			labels = append(labels, labelOf(first).Draw(t, ""))
		}
		labels = append(labels, labelOf(alpha).Draw(t, ""))
	}
	// Drop leading labels, then shorten the last one, to fit the budget.
	for len(labels) > 1 && len(strings.Join(labels, ".")) > budget {
		labels = labels[1:]
	}
	if last := labels[len(labels)-1]; len(last) > budget {
		labels[len(labels)-1] = strings.TrimRight(last[:budget], "-")
	}
	return strings.Join(labels, ".")
}
