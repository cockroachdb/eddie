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

package rt

import (
	"bytes"
	"fmt"
	"go/token"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/cockroachdb/eddie/pkg/contract"
)

// A Result describes a message associated with a position in an input
// file.
type Result struct {
	// Nested Results.
	Children Results
	// The position of the contract declaration which caused the Result.
	Contract token.Position
	// The data written by the Contract.
	Data bytes.Buffer
	// The position that the message is associated with.
	Pos token.Position
	// The name of the contract which produced the result.
	Producer string
}

// String is suitable for human consumption.
func (r Result) String() string {
	return r.StringRelative("")
}

// StringRelative is suitable for human consumption and makes all
// emitted file paths relative to the given base path.
func (r Result) StringRelative(basePath string) string {
	sb := &strings.Builder{}
	sb.WriteString(fmt.Sprintf("From %s at %s", r.Producer, r.Contract))
	r.string("", basePath, sb)
	return sb.String()
}

func (r Result) string(prefix, basePath string, into *strings.Builder) {
	if r.Data.Len() > 0 {
		prefix = "  " + prefix

		into.WriteString("\n")
		into.WriteString(prefix)

		if basePath == "" {
			into.WriteString(r.Pos.Filename)
		} else if rel, err := filepath.Rel(basePath, r.Pos.Filename); err != nil {
			into.WriteString(r.Pos.Filename)
		} else {
			into.WriteString(rel)
		}
		into.WriteString(fmt.Sprintf(":%d:%d: ", r.Pos.Line, r.Pos.Column))

		for idx, line := range strings.Split(r.Data.String(), "\n") {
			if idx > 0 {
				into.WriteString("\n")
				into.WriteString(prefix)
			}
			into.WriteString(line)
		}
	}

	for _, child := range r.Children {
		child.string(prefix, basePath, into)
	}
}

// Results is a sortable slice of Result.
type Results []*Result

var _ sort.Interface = Results{}

// Len implements sort.Interface.
func (r Results) Len() int { return len(r) }

// Less implements sort.Interface. It orders results by their filenames,
// position within the file, producer, and then by data.
func (r Results) Less(i, j int) bool {
	a, b := r[i], r[j]

	if c := strings.Compare(a.Pos.Filename, b.Pos.Filename); c != 0 {
		return c < 0
	}
	if c := a.Pos.Offset - b.Pos.Offset; c != 0 {
		return c < 0
	}
	if c := strings.Compare(a.Producer, b.Producer); c != 0 {
		return c < 0
	}
	return bytes.Compare(a.Data.Bytes(), b.Data.Bytes()) < 0
}

// Swap implements sort.Interface.
func (r Results) Swap(i, j int) { r[i], r[j] = r[j], r[i] }

// String is for debugging use only.
func (r Results) String() string {
	sb := &strings.Builder{}
	// We can sort the top-level and second-level results to create
	// more stable output, but we don't want to sort the remainder
	// of the levels, since we don't know what the Contracts intended.
	copy := append(r[:0], r...)
	sort.Sort(copy)
	for _, result := range copy {
		sort.Sort(result.Children)
		sb.WriteString(fmt.Sprintf("%s\n\n", result.String()))
	}
	return sb.String()
}

// resultReporter adapts a Result to the ext.Reporter interface.
type resultReporter struct {
	fset *token.FileSet
	mu   struct {
		sync.Mutex
		result *Result
	}
}

var _ contract.Reporter = &resultReporter{}

func newResultReporter(
	fset *token.FileSet, contract token.Pos, producer string, pos token.Pos,
) (*Result, contract.Reporter) {
	r := &resultReporter{fset: fset}
	r.mu.result = &Result{
		Contract: fset.Position(contract),
		Producer: producer,
		Pos:      fset.Position(pos),
	}
	return r.mu.result, r
}

// Detail implements ext.Reporter.
func (r *resultReporter) Detail(l contract.Located) contract.Reporter {
	r.mu.Lock()
	parent := r.mu.result
	child, ret := newResultReporter(r.fset, token.NoPos, parent.Producer, l.Pos())
	child.Contract = parent.Contract
	parent.Children = append(parent.Children, child)
	r.mu.Unlock()
	return ret
}

// Print implements ext.Reporter.
func (r *resultReporter) Print(args ...interface{}) {
	_, _ = fmt.Fprint(r, args...)
}

// Printf implements ext.Reporter.
func (r *resultReporter) Printf(format string, args ...interface{}) {
	_, _ = fmt.Fprintf(r, format, args...)
}

// Println implements ext.Reporter.
func (r *resultReporter) Println(args ...interface{}) {
	_, _ = fmt.Fprintln(r, args...)
}

// Write implements ext.Reporter.
func (r *resultReporter) Write(p []byte) (n int, err error) {
	r.mu.Lock()
	n, err = r.mu.result.Data.Write(p)
	r.mu.Unlock()
	return
}
