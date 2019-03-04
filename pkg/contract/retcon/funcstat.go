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

package retcon

import (
	"fmt"
	"strings"

	"golang.org/x/tools/go/ssa"
)

//go:generate stringer -type state -trimprefix state

type state int

const (
	stateUnknown state = iota
	stateAnalyzing
	stateClean
	stateDirty
)

// clean is a sentinel value.
var clean = &funcStat{state: stateClean}

type funcStat struct {
	dirties       map[*funcStat]*ssa.Call
	fn            *ssa.Function
	returns       []*ssa.Return
	state         state
	targetIndexes []int
	// why contains the shortest-dirty-path
	why []dirtyReason
}

var _ dirtyFunction = &funcStat{}

// Fn implements dirtyFunction
func (s *funcStat) Fn() *ssa.Function {
	return s.fn
}

// String is for debugging use only.  See ReturnConcrete.Report().
func (s *funcStat) String() string {
	if s == clean {
		return "<Clean>"
	}
	sb := &strings.Builder{}

	fset := s.fn.Prog.Fset
	fmt.Fprintf(sb, "%s: func %s",
		fset.Position(s.fn.Pos()), s.fn.RelString(s.fn.Pkg.Pkg))
	for _, reason := range s.why {
		fmt.Fprintf(sb, "\n  %s: %s: %s",
			fset.Position(reason.Value.Pos()),
			reason.Reason,
			reason.Value)
	}

	return sb.String()
}

// Why implements dirtyFunction.
func (s *funcStat) Why() []dirtyReason {
	return s.why
}
