// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package pbt

import (
	"encoding/binary"
	"fmt"
	"hash/fnv"
)

const (
	duplicateMinCases    = 20
	duplicateMaxDistinct = 4096
)

// distinctInputs retains hashes, never choices or drawn values. If a new key
// would exceed the cap, disable the check instead of mistaking the cap for a
// small domain. Repeated inputs at the cap still count.
type distinctInputs struct {
	cases     int
	seen      map[uint64]struct{}
	saturated bool
}

func (d *distinctInputs) add(t *T) {
	if d.saturated {
		return
	}
	d.cases++
	h := fnv.New64a()
	var buf [8]byte
	write := func(n uint64) {
		binary.LittleEndian.PutUint64(buf[:], n)
		h.Write(buf[:])
	}
	write(uint64(t.size))
	write(uint64(len(t.choices)))
	for _, choice := range t.choices {
		write(choice)
	}
	key := h.Sum64()
	if _, ok := d.seen[key]; ok {
		return
	}
	if len(d.seen) == duplicateMaxDistinct {
		d.saturated = true
		d.seen = nil
		return
	}
	if d.seen == nil {
		d.seen = make(map[uint64]struct{})
	}
	d.seen[key] = struct{}{}
}

func (d *distinctInputs) note() *healthNote {
	if d.saturated || d.cases < duplicateMinCases || len(d.seen)*2 >= d.cases {
		return nil
	}
	return &healthNote{HealthDuplicate, fmt.Sprintf("only %d of %d generated inputs were distinct by choices and size; possible causes include a small domain such as Bool or SampledFrom, a low maximum size, or more Checks than the input space has values", len(d.seen), d.cases)}
}

func (r *runner) observeDistinct(t *T, targeted bool) {
	if r.health == nil || targeted || t.hasUtility || t.discarded || !r.cfg.healthEnabled(HealthDuplicate) {
		return
	}
	r.health.distinct.add(t)
}
