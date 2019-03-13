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
	"go/token"
	"reflect"
	"sync"

	"github.com/pkg/errors"
)

// ImpliesHints allows a hint or Contract to add additional hints
// when it is associated with an object.
type ImpliesHints interface {
	ImpliedHints() []interface{}
}

type hintKey struct {
	target   token.Pos
	contract reflect.Type
}

// Hints allows contracts to store and to share information about
// program elements.  Every contract instance defined in a program will
// be available through the hints mechanism, allowing contracts to make
// assumptions based solely on source-code declaration.
//
// The methods on hints are safe to call from multiple goroutines,
// however no guarantees are made about the enclosed hint data.
type Hints struct {
	mu struct {
		sync.RWMutex
		data map[hintKey][]interface{}
	}
}

// NewHints constructs a new Hints helper.  Contract implementations
// should prefer to use the shared instance available from Context.
func NewHints() *Hints {
	h := &Hints{}
	h.mu.data = make(map[hintKey][]interface{})
	return h
}

// Add associates the hint with the program member.
//
// Add panics if the supplied hint value cannot be dereferenced to a
// struct type.
func (h *Hints) Add(member Located, hint interface{}) {
	v := reflect.ValueOf(hint)
	for v.Kind() == reflect.Interface {
		v = v.Elem()
	}
	if v.Kind() == reflect.Struct {
		temp := reflect.New(v.Type())
		temp.Set(v)
		v = temp
	} else if v.Kind() != reflect.Ptr || v.Elem().Kind() != reflect.Struct {
		panic(errors.Errorf("%T is not a struct type", hint))
	}

	k := hintKey{member.Pos(), v.Elem().Type()}

	h.mu.Lock()
	h.mu.data[k] = append(h.mu.data[k], v.Interface())
	h.mu.Unlock()

	if implies, ok := hint.(ImpliesHints); ok {
		for _, implied := range implies.ImpliedHints() {
			h.Add(member, implied)
		}
	}
}

// Get will retrieve the hints of the target type associated with a
// program member.
//
//   for _, intf := range hints.Get(member, &SomeHintType{}) {
//     hint := intf.(SomeHintType)
//   }
//
// Get will panic if the target is not an addressable pointer to a
// struct type.
func (h *Hints) Get(member Located, target interface{}) []interface{} {
	v := reflect.ValueOf(target)
	for v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		panic(errors.Errorf("%T is not a struct type", target))
	}

	h.mu.RLock()
	found := h.mu.data[hintKey{member.Pos(), v.Type()}]
	h.mu.RUnlock()

	return append(found[:0], found...)
}
