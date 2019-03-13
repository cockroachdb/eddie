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

// Package rt contains the runtime code which will support a generated
// enforcer binary.
package rt

import (
	"fmt"
	"go/token"
	"go/types"
	"path/filepath"
	"regexp"
	"sort"

	"github.com/cockroachdb/eddie/pkg/contract"
	"golang.org/x/tools/go/packages"
)

// Example:
//   // contract:SomeContract {....}
//   /* contract:SomeContract {....} */
var commentSyntax = regexp.MustCompile(
	`^(?s)(?://|/\*)[[:space:]]*contract:([[:alnum:]]+)(.*?)(?:\*/)?$`)

//   ^ s-flag enables dot to match newline
//          ^ Non-capturing group to match leading comment marker
// Ignore leading WS   ^
// Look for the literal contract: ^
//          Contract names are golang idents ^
//                Configuration data, non-greedy match ^
//                              Ignore closing block comment ^

// TODO(bob): Allow contracts to be bound to functions where a type
// is in scope.  For instance, we might want to examine all uses
// of where a context.Context is in scope to look for select{}
// constructs that don't check the Done() channel.

// TODO(bob): Allow contracts to be bound to packages (and sub-packages).

// A target describes a binding between a contract and a named object.
type target struct {
	// The raw JSON configuration string, extracted from the doc comment.
	config string
	// The name of the contract, which could be an alias.
	contract string
	// The object which encloses object, if any.
	enclosing types.Object
	// Underlying source data, for position lookups.
	fset *token.FileSet
	// Memoizes the behavior to apply to the target.
	kind contract.Kind
	// The object on which the contract is bound,
	// a type, field, method, or function.
	object types.Object
	// The position of the binding comment.
	pos token.Pos
}

// Pos implement the Located interface.
func (t *target) Pos() token.Pos {
	return t.pos
}

// String is for debugging use only.
func (t *target) String() string {
	pos := t.fset.Position(t.Pos())
	return fmt.Sprintf("%s:%d:%d %s %s := %s %s",
		filepath.Base(pos.Filename), pos.Line, pos.Column,
		t.kind, t.object.Name(), t.contract, t.config)
}

// An assertion represents a top-level declaration of the forms
//   var _ SomeInterface = SomeStruct{}
//   var _ SomeInterface = &SomeStruct{}
type assertion struct {
	fset *token.FileSet
	// A named interface type.
	intf types.Object
	// The implementing type.
	impl types.Object
	pos  token.Pos
}

// Pos implements the Located interface.
func (a *assertion) Pos() token.Pos {
	return a.pos
}

// String is for debugging use only.
func (a *assertion) String() string {
	pos := a.fset.Position(a.Pos())
	return fmt.Sprintf("%s:%d:%d: var _ %s = %s{}",
		filepath.Base(pos.Filename), pos.Line, pos.Column,
		a.intf.Id(), a.impl.Id())
}

// These slice types will sort based on their element's token.Pos.
var (
	_ sort.Interface = assertions{}
	_ sort.Interface = targets{}
)

type assertions []*assertion

func (a assertions) Len() int           { return len(a) }
func (a assertions) Less(i, j int) bool { return a[i].Pos() < a[j].Pos() }
func (a assertions) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }

type targets []*target

func (t targets) Len() int           { return len(t) }
func (t targets) Less(i, j int) bool { return t[i].Pos() < t[j].Pos() }
func (t targets) Swap(i, j int)      { t[i], t[j] = t[j], t[i] }

type targetAliases map[string]targets

// flattenImports will return the given packages and their transitive
// imports as a map keyed by package ID.
func flattenImports(pkgs []*packages.Package) map[string]*packages.Package {
	seen := make(map[string]*packages.Package)
	for pkgs != nil {
		work := pkgs
		pkgs = nil
		for _, pkg := range work {
			if seen[pkg.ID] == nil {
				seen[pkg.ID] = pkg
				for _, imp := range pkg.Imports {
					pkgs = append(pkgs, imp)
				}
			}
		}
	}
	return seen
}
