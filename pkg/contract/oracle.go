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

package contract

import (
	"go/types"
	"sync"

	"golang.org/x/tools/go/ssa"
)

// Assertions define type relationships that have been explicitly
// asserted in source.  Generally, these are declarations of the form
//   var _ A = B{}
type Assertions map[types.Object][]types.Object

// A TypeOracle answers questions about a program's typesystem.
// All methods are safe to call from multiple goroutines.
type TypeOracle struct {
	assertedImplementors map[*types.Interface][]types.Object
	pgm                  *ssa.Program
	mu                   struct {
		sync.RWMutex
		typeImplementors map[*types.Interface][]types.Object
	}
}

// NewOracle constructs a TypeOracle.  In general, contract
// implementations should prefer the shared instance provided by
// Context, rather than constructing a new one.
func NewOracle(pgm *ssa.Program, assertions Assertions) *TypeOracle {
	ret := &TypeOracle{
		assertedImplementors: make(map[*types.Interface][]types.Object, len(assertions)),
		pgm:                  pgm,
	}
	for k, v := range assertions {
		if intf, ok := k.Type().Underlying().(*types.Interface); ok {
			ret.assertedImplementors[intf] = v
		}
	}
	ret.mu.typeImplementors = make(map[*types.Interface][]types.Object)
	return ret
}

// MethodImplementors finds the named method on all types which implement
// the given interface.
func (o *TypeOracle) MethodImplementors(
	intf *types.Interface, name string, assertedOnly bool,
) []*ssa.Function {
	impls := o.TypeImplementors(intf, assertedOnly)
	ret := make([]*ssa.Function, len(impls))
	for i, impl := range impls {
		// Try on the type directly
		sel := o.pgm.MethodSets.MethodSet(impl.Type()).Lookup(impl.Pkg(), name)
		if sel == nil {
			sel = o.pgm.MethodSets.MethodSet(types.NewPointer(impl.Type())).Lookup(impl.Pkg(), name)
		}
		ret[i] = o.pgm.MethodValue(sel)
	}
	return ret
}

// TypeImplementors returns the runtime times which implement the
// given interface, according to explicit assertions made by the user.
func (o *TypeOracle) TypeImplementors(intf *types.Interface, assertedOnly bool) []types.Object {
	var ret []types.Object

	if assertedOnly {
		ret = o.assertedImplementors[intf]
	} else {
		o.mu.RLock()
		// We may insert nil slices later on, so use comma-ok.
		maybe, found := o.mu.typeImplementors[intf]
		o.mu.RUnlock()

		if !found {
			for _, typ := range o.pgm.RuntimeTypes() {
				if !types.Implements(typ, intf) {
					continue
				}
				var lastName types.Object
			chase:
				for {
					switch t := typ.(type) {
					case *types.Pointer:
						typ = t.Elem()
					case *types.Named:
						lastName = t.Obj()
						typ = t.Underlying()
					default:
						break chase
					}
				}
				if lastName != nil {
					maybe = append(maybe, lastName)
				}
			}

			ret = maybe
			o.mu.Lock()
			o.mu.typeImplementors[intf] = maybe
			o.mu.Unlock()
		}
	}

	// Return copies of non-nil slices.
	if ret != nil {
		ret = append(ret[:0], ret...)
	}
	return ret
}
