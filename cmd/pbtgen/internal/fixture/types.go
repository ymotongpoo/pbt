// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

// Package fixture holds the types that the pbtgen tests generate
// generators for.
package fixture

import (
	"net"
	"net/netip"
	"time"
)

//go:generate go run go.ymotongpoo.com/pbt/cmd/pbtgen -type Order,Customer,Tree,Celsius,Standard

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

//go:generate go run go.ymotongpoo.com/pbt/cmd/pbtgen -type Dept,Employee -output mutual_gen.go

// Dept and Employee are mutually recursive.
type Dept struct {
	Name  string
	Staff []Employee
}

// Employee refers back to its department and to peers.
type Employee struct {
	Dept  *Dept
	Peers []Employee
}

// Standard exercises generation of standard-library types with private state.
type Standard struct {
	Time     time.Time
	Zone     *time.Location
	Addr     netip.Addr
	Prefix   netip.Prefix
	Port     netip.AddrPort
	IP       net.IP
	MAC      net.HardwareAddr
	Bounded  time.Time `pbt:"min=2020-01-01T01:00:00+01:00,max=2021-01-01T00:00:00.123Z"`
	MinOnly  time.Time `pbt:"min=2020-01-01T00:00:00Z"`
	MaxOnly  time.Time `pbt:"max=2021-01-01T00:00:00Z"`
	Duration time.Duration
	Override time.Time `pbt:"gen=standardTimeGen"`
}
