// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package pbt

import (
	"fmt"
	"math"
	"unsafe"
)

type integer interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 | ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr
}

// Bool generates booleans. False is the simpler value.
func Bool() Gen[bool] {
	return Gen[bool]{name: "Bool()", draw: func(t *T) bool { return t.drawBool(0.5) }}
}

// Integer generates every value of the integer type I. Signed types shrink
// toward 0, unsigned types toward their minimum.
func Integer[I integer]() Gen[I] {
	lo, hi := integerBounds[I]()
	return named(IntegerRange(lo, hi), fmt.Sprintf("Integer[%T]()", lo))
}

// IntegerRange generates integers of type I in [lo, hi]. Values shrink
// toward 0 when 0 is in the range, and toward the bound nearest to 0
// otherwise.
func IntegerRange[I integer](lo, hi I) Gen[I] {
	var zero I
	if lo > hi {
		panic(fmt.Sprintf("pbt: IntegerRange[%T](%d, %d): lo is above hi", zero, lo, hi))
	}
	return integerRange(lo, hi, min(max(lo, zero), hi), fmt.Sprintf("IntegerRange[%T](%d, %d)", zero, lo, hi))
}

// IntegerRangeToward generates integers of type I in [lo, hi] that shrink
// toward center instead of 0. It re-centers shrinking the way PropEr's
// ?SHRINK does in the book's year example.
func IntegerRangeToward[I integer](lo, hi, center I) Gen[I] {
	var zero I
	if lo > hi || center < lo || center > hi {
		panic(fmt.Sprintf("pbt: IntegerRangeToward[%T](%d, %d, %d): need lo <= center <= hi", zero, lo, hi, center))
	}
	return integerRange(lo, hi, center, fmt.Sprintf("IntegerRangeToward[%T](%d, %d, %d)", zero, lo, hi, center))
}

// IntRangeToward generates ints in [lo, hi] that shrink toward center.
func IntRangeToward(lo, hi, center int) Gen[int] {
	return named(IntegerRangeToward(lo, hi, center), fmt.Sprintf("IntRangeToward(%d, %d, %d)", lo, hi, center))
}

// integerRange draws a choice in [0, hi-lo] and decodes it around the center
// in the unsigned domain, which serves signed and unsigned types alike.
func integerRange[I integer](lo, hi, center I, name string) Gen[I] {
	var zero I
	signed := ^zero < zero
	toU := func(x I) uint64 {
		if signed {
			return signedToU(int64(x))
		}
		return uint64(x)
	}
	loU, hiU, cU := toU(lo), toU(hi), toU(center)
	meta := zigzagMetaU(cU, loU, hiU)
	width := hiU - loU
	return Gen[I]{name: name, draw: func(t *T) I {
		u := zigzagDecodeU(t.draw(width, meta, modeBiased), cU, loU, hiU)
		if signed {
			return I(uToSigned(u))
		}
		return I(u)
	}}
}

func integerBounds[I integer]() (I, I) {
	var zero I
	ones := ^zero
	if ones < zero { // signed
		lo := ones << (unsafe.Sizeof(zero)*8 - 1)
		return lo, ^lo
	}
	return zero, ones
}

// Int generates every int.
func Int() Gen[int] { return named(Integer[int](), "Int()") }

// IntRange generates ints in [lo, hi].
func IntRange(lo, hi int) Gen[int] {
	return named(IntegerRange(lo, hi), fmt.Sprintf("IntRange(%d, %d)", lo, hi))
}

// IntMin generates ints that are at least lo.
func IntMin(lo int) Gen[int] {
	return named(IntegerRange(lo, math.MaxInt), fmt.Sprintf("IntMin(%d)", lo))
}

// IntMax generates ints that are at most hi.
func IntMax(hi int) Gen[int] {
	return named(IntegerRange(math.MinInt, hi), fmt.Sprintf("IntMax(%d)", hi))
}

// Int64 generates every int64.
func Int64() Gen[int64] { return named(Integer[int64](), "Int64()") }

// Int64Range generates int64 values in [lo, hi].
func Int64Range(lo, hi int64) Gen[int64] {
	return named(IntegerRange(lo, hi), fmt.Sprintf("Int64Range(%d, %d)", lo, hi))
}

// Int32 generates every int32.
func Int32() Gen[int32] { return named(Integer[int32](), "Int32()") }

// Int32Range generates int32 values in [lo, hi].
func Int32Range(lo, hi int32) Gen[int32] {
	return named(IntegerRange(lo, hi), fmt.Sprintf("Int32Range(%d, %d)", lo, hi))
}

// Uint generates every uint.
func Uint() Gen[uint] { return named(Integer[uint](), "Uint()") }

// UintRange generates uints in [lo, hi].
func UintRange(lo, hi uint) Gen[uint] {
	return named(IntegerRange(lo, hi), fmt.Sprintf("UintRange(%d, %d)", lo, hi))
}

// Uint64 generates every uint64.
func Uint64() Gen[uint64] { return named(Integer[uint64](), "Uint64()") }

// Uint64Range generates uint64 values in [lo, hi].
func Uint64Range(lo, hi uint64) Gen[uint64] {
	return named(IntegerRange(lo, hi), fmt.Sprintf("Uint64Range(%d, %d)", lo, hi))
}

// Uint32 generates every uint32.
func Uint32() Gen[uint32] { return named(Integer[uint32](), "Uint32()") }

// Uint32Range generates uint32 values in [lo, hi].
func Uint32Range(lo, hi uint32) Gen[uint32] {
	return named(IntegerRange(lo, hi), fmt.Sprintf("Uint32Range(%d, %d)", lo, hi))
}

// Byte generates every byte.
func Byte() Gen[byte] { return named(Integer[byte](), "Byte()") }

// ByteRange generates bytes in [lo, hi].
func ByteRange(lo, hi byte) Gen[byte] {
	return named(IntegerRange(lo, hi), fmt.Sprintf("ByteRange(%d, %d)", lo, hi))
}

// Float64 generates finite float64 values. They shrink toward 0 and toward
// integral values.
func Float64() Gen[float64] {
	return named(Float64Range(-math.MaxFloat64, math.MaxFloat64), "Float64()")
}

// Float64Range generates finite float64 values in [lo, hi].
func Float64Range(lo, hi float64) Gen[float64] {
	if math.IsNaN(lo) || math.IsNaN(hi) || math.IsInf(lo, 0) || math.IsInf(hi, 0) || lo > hi {
		panic(fmt.Sprintf("pbt: Float64Range(%v, %v): bounds must be finite with lo <= hi", lo, hi))
	}
	return Gen[float64]{name: fmt.Sprintf("Float64Range(%v, %v)", lo, hi), draw: func(t *T) float64 {
		return drawFloat(t, lo, hi)
	}}
}

// Float32 generates finite float32 values.
func Float32() Gen[float32] {
	return named(Float32Range(-math.MaxFloat32, math.MaxFloat32), "Float32()")
}

// Float32Range generates finite float32 values in [lo, hi].
func Float32Range(lo, hi float32) Gen[float32] {
	g := Float64Range(float64(lo), float64(hi))
	return Gen[float32]{name: fmt.Sprintf("Float32Range(%v, %v)", lo, hi), draw: func(t *T) float32 {
		return float32(min(max(g.draw(t), float64(lo)), float64(hi)))
	}}
}

// drawFloat draws a finite float in [lo, hi] in one of three forms, in
// order of simplicity: an integral value, an integer part plus a binary
// fraction with as few bits as the shrinker can get away with, or a value
// assembled from a sign, an exponent and a mantissa, which reaches every
// magnitude including the extremes. Ranges without an integral value use
// the second form for the first as well.
func drawFloat(t *T, lo, hi float64) float64 {
	const limit = 1 << 53
	ilo, ihi := math.Ceil(lo), math.Floor(hi)
	form := t.draw(2, flagMeta, modeBiased)
	if form == 0 && ilo > ihi {
		form = 1
	}
	switch form {
	case 0:
		a, b := int64(max(ilo, -limit)), int64(min(ihi, limit))
		c := min(max(a, 0), b)
		return float64(zigzagDecode(t.draw(uint64(b)-uint64(a), zigzagMeta(c, a, b), modeBiased), c, a, b))
	case 1:
		a, b := int64(max(math.Floor(lo), -limit)), int64(min(math.Ceil(hi), limit))
		c := min(max(a, 0), b)
		ipart := float64(zigzagDecode(t.draw(uint64(b)-uint64(a), zigzagMeta(c, a, b), modeBiased), c, a, b))
		bits := 1 + t.draw(51, plainMeta, modeBiased)
		mant := t.draw(uint64(1)<<bits-1, plainMeta, modeBiased)
		frac := float64(mant) / float64(uint64(1)<<bits)
		if ipart < 0 || (ipart == 0 && lo < 0 && hi <= 0) {
			return min(max(ipart-frac, lo), hi)
		}
		return min(max(ipart+frac, lo), hi)
	default:
		sign := t.draw(1, plainMeta, modeBiased)
		exp := t.draw(2046, plainMeta, modeBiased)
		mant := t.draw(1<<52-1, plainMeta, modeBiased)
		v := math.Float64frombits(sign<<63 | exp<<52 | mant)
		return min(max(v, lo), hi)
	}
}
