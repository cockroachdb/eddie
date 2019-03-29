// Copyright 2019 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.

// Package retcon defines a contract which requires
package retcon

import (
	"fmt"

	"github.com/cockroachdb/eddie/pkg/contract"
	"golang.org/x/tools/go/ssa"
)

type RangeValPointer struct{}

var _ contract.Contract = &RangeValPointer{}

// Enforce implement the BigEddie Contract interface.
func (l *RangeValPointer) Enforce(ctx contract.Context) error {
	pgm := ctx.Program()
	_ = pgm

	for _, m := range ctx.Objects() {
		fmt.Printf("TYP: %T, %[1]v\n", m)
		switch t := m.(type) {
		case *ssa.Type:
			_ = t
		}

	}

	return nil
}
