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
	"context"
	"log"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"

	"github.com/cockroachdb/eddie/pkg/contract"
	"github.com/cockroachdb/eddie/pkg/rt"
	"github.com/cockroachdb/eddie/pkg/util"
	"github.com/stretchr/testify/assert"
)

type aggregator struct {
	fakePkgName string
	mu          struct {
		sync.Mutex
		dirty []dirtyFunction
	}
}

func (a *aggregator) Enforce(ctx contract.Context) error {
	hints := ctx.Hints().Get(ctx.Objects()[0], ReturnConcrete{})
	linter := hints[0].(*ReturnConcrete)
	err := linter.Enforce(ctx)
	a.mu.Lock()
	a.mu.dirty = append(a.mu.dirty, linter.reported...)
	a.mu.Unlock()
	return err
}

// ImpliedHints implements the ImpliesHints interface.
func (a *aggregator) ImpliedHints() []interface{} {
	return []interface{}{&ReturnConcrete{
		AllowedNames: []string{
			a.fakePkgName + "/GoodPtrError",
			a.fakePkgName + "/GoodValError",
		},
		TargetName: "error",
	}}
}

var _ contract.Contract = &aggregator{}

func Test(t *testing.T) {
	a := assert.New(t)
	testDataDir, err := filepath.Abs("./testdata")
	if !a.NoError(err) {
		return
	}

	aggregator := &aggregator{fakePkgName: util.Base + "contract/retcon/testdata"}

	e := rt.Enforcer{
		AssertedInterfaces: true,
		Contracts: contract.Providers{
			"ReturnConcrete": {New: func() contract.Contract { return aggregator }},
		},
		Dir:      testDataDir,
		Logger:   log.New(os.Stdout, "", 0),
		Name:     "test",
		Packages: []string{"."},
	}

	results, err := e.Execute(context.Background())
	if !a.NoError(err) {
		return
	}

	tcs := []struct {
		name      string
		whyLength int
	}{
		{name: "(*BadError).Self", whyLength: 1},
		{name: "DirectBad", whyLength: 2},
		{name: "DirectTupleBad", whyLength: 2},
		{name: "MakesIndirectCall", whyLength: 1},
		{name: "PhiBad", whyLength: 2},
		{name: "ShortestWhyPath", whyLength: 1},
		{name: "TodoNoTypeInference", whyLength: 1}, // See doc on fn
	}
	a.Len(results, len(tcs))

	dirty := aggregator.mu.dirty
	t.Run("good extraction", func(t *testing.T) {
		a := assert.New(t)
		tcNames := make([]string, len(tcs))
		for i, tc := range tcs {
			tcNames[i] = tc.name
		}
		sort.Strings(tcNames)
		dirtyNames := make([]string, len(dirty))
		for i, d := range dirty {
			dirtyNames[i] = d.Fn().RelString(d.Fn().Pkg.Pkg)
		}
		sort.Strings(dirtyNames)
		a.Equal(tcNames, dirtyNames)
	})

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			a := assert.New(t)

			for _, d := range dirty {
				if d.Fn().RelString(d.Fn().Pkg.Pkg) == tc.name {
					a.Equalf(tc.whyLength, len(d.Why()), "unexpected why length:\n%s", d)
					return
				}
			}
			a.Fail("did not find function", tc.name)
		})
	}
}
