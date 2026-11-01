// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package ch14

import (
	"fmt"
	"maps"
	"testing"

	"go.ymotongpoo.com/pbt"
	"go.ymotongpoo.com/pbt/examples/discovery/internal/mutant"
)

// embeddedAuditProperty は embeddedProperty に全体観測の監査を足した版である。
func embeddedAuditProperty(newStore func() Store) pbt.Property {
	return func(t *pbt.T) {
		s := newStore()
		model := map[int]int{}

		overwrite := pbt.Map(putGen, func(op putOp) pbt.Command {
			return pbt.Command{
				Name: "overwrite", Args: []any{op.k, op.v},
				Run: func(pbt.Env) any {
					s.Put(op.k, -1)
					s.Put(op.k, op.v)
					return nil
				},
				Next: func(pbt.Var) { model[op.k] = op.v },
			}
		})
		audit := pbt.Just(pbt.Command{
			Name: "audit",
			Run:  func(pbt.Env) any { return s.Snapshot() },
			Post: func(_ pbt.Env, got any) error {
				if !maps.Equal(got.(map[int]int), model) {
					return fmt.Errorf("snapshot %v, model %v", got, model)
				}
				return nil
			},
		})
		commands := func() pbt.CommandGen { return pbt.OneOf(overwrite, audit) }
		init := func() { clear(model) }

		prog := pbt.Commands(init, commands).Draw(t, "cmds")
		prog.RunSequential(t)
	}
}

func TestEmbeddedAudit(t *testing.T) { pbt.Check(t, embeddedAuditProperty(mkKV), opts...) }
func TestEmbeddedAuditStrayKeyMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, embeddedAuditProperty(mkStrayKey), opts...)
}
