// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

// Package fixture holds the types that the pbtgen tests generate
// generators for.
package fixture

import "time"

//go:generate go run go.ymotongpoo.com/pbt/cmd/pbtgen -type Order,Customer,Tree,Celsius

// Celsius is a named basic type.
type Celsius float64

// Customer has exported and unexported fields and a tagged field.
type Customer struct {
	ID    int `pbt:"min=1"`
	Name  string
	Email string `pbt:"gen=emailGen"`
	vip   bool
	Since time.Time `pbt:"-"`
}

// Order refers to Customer, which has its own generator, and to an
// anonymous struct and collections.
type Order struct {
	Customer Customer
	Lines    []struct {
		SKU string `pbt:"min=1,max=8"`
		Qty uint8  `pbt:"min=1"`
	}
	Notes  map[string]string `pbt:"max=3"`
	Temps  [2]Celsius
	Rush   *bool
	Weight float32 `pbt:"min=0,max=100"`
}

// Tree is recursive through a slice and a pointer.
type Tree struct {
	Label    string
	Children []Tree
	Parent   *Tree
}
