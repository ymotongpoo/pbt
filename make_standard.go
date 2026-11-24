// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package pbt

import (
	"fmt"
	"net"
	"net/netip"
	"reflect"
	"time"
)

// standardDraw recognizes exact standard-library types before inspecting
// their representation. Defined types with the same underlying type keep
// the ordinary kind-based behavior; aliases have the same identity.
func standardDraw(typ reflect.Type, tg tag) drawFn {
	var g Gen[any]
	switch typ {
	case reflect.TypeFor[time.Time]():
		if tg.min == "" && tg.max == "" {
			g = asAny(Time())
		} else {
			lo, hi := minTime, maxTime
			// Numeric offsets stay fixed instead of inheriting the host Local zone.
			parse := func(s string) (time.Time, error) { return time.ParseInLocation(time.RFC3339, s, time.UTC) }
			if tg.min != "" {
				lo = parseBound(tg.min, parse)
			}
			if tg.max != "" {
				hi = parseBound(tg.max, parse)
			}
			if lo.After(hi) {
				panic(fmt.Sprintf("pbt: Make: min %s is above max %s", lo, hi))
			}
			g = asAny(TimeRange(lo, hi))
		}
	case reflect.TypeFor[*time.Location]():
		g = asAny(FixedZone())
	case reflect.TypeFor[netip.Addr]():
		g = asAny(Addr())
	case reflect.TypeFor[netip.Prefix]():
		g = asAny(Prefix())
	case reflect.TypeFor[netip.AddrPort]():
		g = asAny(AddrPort())
	case reflect.TypeFor[net.IP]():
		g = asAny(Custom("net.IP", func(t *T) net.IP {
			a := Addr().Draw(t, "")
			if a.Is4() && Bool().Draw(t, "") {
				return net.IP(a.AsSlice())
			}
			b := a.As16()
			return net.IP(b[:])
		}))
	case reflect.TypeFor[net.HardwareAddr]():
		g = asAny(HardwareAddr())
	default:
		return nil
	}
	return func(t *T, _ *budget) reflect.Value { return reflect.ValueOf(UnboundedGrowth(g).drawSpan(t)) }
}
