// Package ch01cash ports the cash register of Chapter 1 of the PropEr Testing
// book. Amounts are integer cents rather than floats, so that sums are exact.
package ch01cash

import (
	"errors"
	"fmt"
)

// Denomination is a bill or coin value in cents together with how many of
// them the register holds.
type Denomination struct {
	Value int
	Count int
}

// ErrCannotMakeChange reports that the register cannot hand back the change
// from the largest bill down to the smallest coin.
var ErrCannotMakeChange = errors.New("cannot make change")

// Cash returns the change for a purchase, handed back from the largest
// denomination down, as the specification of the chapter describes. The
// register must list its denominations from the largest value to the
// smallest.
func Cash(register []Denomination, price, paid int) ([]Denomination, error) {
	if paid < price {
		return nil, fmt.Errorf("paid %d is less than the price %d", paid, price)
	}
	remaining := paid - price
	var change []Denomination
	for _, d := range register {
		if d.Value <= 0 || remaining < d.Value {
			continue
		}
		n := min(remaining/d.Value, d.Count)
		if n == 0 {
			continue
		}
		change = append(change, Denomination{Value: d.Value, Count: n})
		remaining -= n * d.Value
	}
	if remaining != 0 {
		return nil, fmt.Errorf("%w: %d cents left over", ErrCannotMakeChange, remaining)
	}
	return change, nil
}
