package pbt

import (
	"encoding/binary"
	"math"
	"math/bits"
	"math/rand/v2"
	"time"
)

func randomSeed() uint64 {
	return splitmix64(uint64(time.Now().UnixNano()))
}

// splitmix64 is the output function of SplitMix64. It spreads seeds and case
// indices so that neighbouring values give unrelated streams.
func splitmix64(x uint64) uint64 {
	x += 0x9e3779b97f4a7c15
	x = (x ^ (x >> 30)) * 0xbf58476d1ce4e5b9
	x = (x ^ (x >> 27)) * 0x94d049bb133111eb
	return x ^ (x >> 31)
}

// caseSeed derives the seed of test case i from the seed of the run.
func caseSeed(base, i uint64) uint64 {
	return splitmix64(base ^ splitmix64(i))
}

func newRand(seed uint64) *rand.Rand {
	var key [32]byte
	x := seed
	for i := range 4 {
		x = splitmix64(x)
		binary.LittleEndian.PutUint64(key[i*8:], x)
	}
	return rand.New(rand.NewChaCha8(key))
}

func fresh(r *rand.Rand, max uint64, mode drawMode, size, maxSize int) uint64 {
	if max == 0 {
		return 0
	}
	if mode == modeUniform {
		return uniform64(r, max)
	}
	return biased64(r, max, size, maxSize)
}

func uniform64(r *rand.Rand, max uint64) uint64 {
	if max == math.MaxUint64 {
		return r.Uint64()
	}
	return r.Uint64N(max + 1)
}

// biased64 draws from [0, max] favouring small values and the edges. The
// usable bit width grows with the size parameter, so early test cases use
// small values and the full range is reached at the maximum size.
func biased64(r *rand.Rand, max uint64, size, maxSize int) uint64 {
	switch r.IntN(32) {
	case 0:
		return max
	case 1:
		if max > 1 {
			return max - 1
		}
	}
	width := bits.Len64(max)
	if maxSize > 0 && size < maxSize {
		width = max64(1, width*size/maxSize)
	}
	if width > 1 {
		width = 1 + r.IntN(width)
	}
	var v uint64
	if width >= 64 {
		v = r.Uint64()
	} else {
		v = r.Uint64N(uint64(1) << width)
	}
	if v > max {
		v = uniform64(r, max)
	}
	return v
}

func max64(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// zigzagDecodeU maps a choice to a value in [lo, hi] around the center c:
// 0 is c, 1 is c+1, 2 is c-1, 3 is c+2 and so on. Once one side of the range
// is exhausted the remaining choices continue on the other side. Smaller
// choices are therefore closer to the center, which is what shrinking moves
// toward. Signed ranges go through signedToU first.
func zigzagDecodeU(u, c, lo, hi uint64) uint64 {
	up := hi - c
	down := c - lo
	both := min(up, down)
	if u <= 2*both {
		if u == 0 {
			return c
		}
		k := u/2 + u%2
		if u%2 == 1 {
			return c + k
		}
		return c - k
	}
	rest := u - 2*both
	if up > down {
		return c + both + rest
	}
	return c - both - rest
}

// zigzagMetaU describes the layout zigzagDecodeU uses for the range
// [lo, hi] around c, for the shrinker.
func zigzagMetaU(c, lo, hi uint64) choiceMeta {
	up := hi - c
	down := c - lo
	return choiceMeta{kind: kindZigzag, zz: 2 * min(up, down), tailPos: up > down}
}

// signedToU maps int64 values to uint64 values with the same ordering, so
// that the unsigned zigzag functions serve signed ranges too.
func signedToU(v int64) uint64 { return uint64(v) ^ (1 << 63) }

func uToSigned(u uint64) int64 { return int64(u ^ (1 << 63)) }

func zigzagDecode(u uint64, c, lo, hi int64) int64 {
	return uToSigned(zigzagDecodeU(u, signedToU(c), signedToU(lo), signedToU(hi)))
}

func zigzagMeta(c, lo, hi int64) choiceMeta {
	return zigzagMetaU(signedToU(c), signedToU(lo), signedToU(hi))
}

// offsetOf interprets a zigzag choice as a signed offset from its center.
// It agrees with zigzagDecode for every choice the generator can record.
func offsetOf(u uint64, m choiceMeta) int64 {
	if u <= m.zz {
		k := int64(u/2 + u%2)
		if u%2 == 0 {
			return -k
		}
		return k
	}
	d := int64(m.zz/2 + (u - m.zz))
	if m.tailPos {
		return d
	}
	return -d
}

// choiceOf is the inverse of offsetOf. An offset that the layout cannot
// represent is clamped to the farthest choice on its side.
func choiceOf(v int64, m choiceMeta) uint64 {
	if v == 0 {
		return 0
	}
	both := m.zz / 2
	mag := absInt64(v)
	switch {
	case mag <= both:
		if v > 0 {
			return 2*mag - 1
		}
		return 2 * mag
	case (v > 0) == m.tailPos:
		return m.zz + (mag - both)
	case both == 0:
		return 0
	case v > 0:
		return 2*both - 1
	default:
		return 2 * both
	}
}

func absInt64(v int64) uint64 {
	if v < 0 {
		return uint64(-v)
	}
	return uint64(v)
}
