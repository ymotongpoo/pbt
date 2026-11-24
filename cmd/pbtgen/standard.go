// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"go/types"
	"time"
)

// standardExpr matches type identity, including aliases, rather than the
// representation of types whose useful state may be unexported.
func (g *generator) standardExpr(t types.Type, tg tag) (string, error) {
	t = types.Unalias(t)
	pointer := false
	if p, ok := t.(*types.Pointer); ok {
		pointer = true
		t = types.Unalias(p.Elem())
	}
	n, ok := t.(*types.Named)
	if !ok || n.Obj().Pkg() == nil {
		return "", nil
	}
	key := n.Obj().Pkg().Path() + "." + n.Obj().Name()
	if pointer {
		key = "*" + key
	}
	switch key {
	case "time.Time":
		if tg.min == "" && tg.max == "" {
			return "pbt.Time()", nil
		}
		lo, err := time.ParseInLocation(time.RFC3339, orDefault(tg.min, "0001-01-01T00:00:00Z"), time.UTC)
		if err != nil {
			return "", fmt.Errorf("invalid time min bound %q: %w", tg.min, err)
		}
		hi, err := time.ParseInLocation(time.RFC3339, orDefault(tg.max, "9999-12-31T23:59:59.999999999Z"), time.UTC)
		if err != nil {
			return "", fmt.Errorf("invalid time max bound %q: %w", tg.max, err)
		}
		if lo.After(hi) {
			return "", fmt.Errorf("time min %s is above max %s", lo, hi)
		}
		g.imports["time"] = "time"
		return fmt.Sprintf("pbt.TimeRange(%s, %s)", timeConstant(lo), timeConstant(hi)), nil
	case "*time.Location":
		return "pbt.FixedZone()", nil
	case "net/netip.Addr":
		return "pbt.Addr()", nil
	case "net/netip.Prefix":
		return "pbt.Prefix()", nil
	case "net/netip.AddrPort":
		return "pbt.AddrPort()", nil
	case "net.HardwareAddr":
		return "pbt.HardwareAddr()", nil
	case "net.IP":
		g.imports["net"] = "net"
		return `pbt.Custom("net.IP", func(t *pbt.T) net.IP {
   a := pbt.Addr().Draw(t, "")
   if a.Is4() && pbt.Bool().Draw(t, "") { return net.IP(a.AsSlice()) }
   b := a.As16()
   return net.IP(b[:])
  })`, nil
	}
	return "", nil
}

func timeConstant(v time.Time) string {
	loc := "time.UTC"
	if v.Location() != time.UTC {
		_, offset := v.Zone()
		loc = fmt.Sprintf("time.FixedZone(\"\", %d)", offset)
	}
	return fmt.Sprintf("time.Date(%d, %d, %d, %d, %d, %d, %d, %s)", v.Year(), v.Month(), v.Day(), v.Hour(), v.Minute(), v.Second(), v.Nanosecond(), loc)
}
