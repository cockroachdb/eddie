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
	"context"
	"fmt"
	"go/types"
	"log"
	"os"
	"testing"

	"github.com/cockroachdb/eddie/pkg/contract"
	"github.com/stretchr/testify/assert"
	"golang.org/x/tools/go/ssa"
)

type checkKey struct {
	contract string
	name     string
	kind     contract.Kind
}
type check func(a *assert.Assertions, ctx contract.Context, r *recorder)

type cases map[checkKey]check

type recorder struct {
	t     *testing.T
	cases cases
	// To look like a MustReturnInt to the json decoder.
	Expected int
}

func (r *recorder) Enforce(ctx contract.Context) error {
	key := checkKey{contract: ctx.Contract(), name: ctx.Declaration().Name(), kind: ctx.Kind()}
	r.t.Run(fmt.Sprint(key), func(t *testing.T) {
		a := assert.New(t)
		fn := r.cases[key]
		ctx.Reporter().Println("here")
		if a.NotNilf(fn, "missing check %#v", key) {
			fn(a, ctx, r)
		}
	})
	return nil
}

// This test creates a statically-configured Enforcer using the demo package.
func Test(t *testing.T) {
	a := assert.New(t)

	// Test cases are selected by the values return from
	// ext.Context.Contract(), Name(), and Kind().
	//
	// NB: Keep these entries in the order in which they appear in the
	// demo source to improve readability.
	tcs := cases{
		{
			contract: "CanGoHere",
			name:     "ReturnsNumber",
			kind:     contract.KindInterface,
		}: func(a *assert.Assertions, ctx contract.Context, r *recorder) {
			// We expect to thee the interface and the two implementing types.
			a.IsType(&types.Interface{}, ctx.Declaration().(*ssa.Type).Type().Underlying())

			// Verify that we see the two implementing types.
			a.Len(ctx.Objects(), 2)
			for _, obj := range ctx.Objects() {
				a.Contains([]string{"ShouldPass", "ShouldFail"}, obj.Name())
			}
		},

		{
			contract: "CanGoHere",
			name:     "ReturnsNumber",
			kind:     contract.KindInterfaceMethod,
		}: func(a *assert.Assertions, ctx contract.Context, _ *recorder) {
			// Verify that we see the declaring interface.
			a.IsType(&types.Interface{}, ctx.Declaration().(*ssa.Type).Type().Underlying())

			// Verify that we see the two implementing methods.
			a.Len(ctx.Objects(), 2)
			for _, obj := range ctx.Objects() {
				fn := obj.(*ssa.Function)
				a.Contains([]string{"ShouldPass", "ShouldFail"},
					fn.Signature.Recv().Type().(*types.Named).Obj().Name())
			}
		},

		{
			contract: "MustReturnInt",
			name:     "ReturnsNumber",
			kind:     contract.KindInterfaceMethod,
		}: func(a *assert.Assertions, ctx contract.Context, r *recorder) {
			// Verify that configuration actually happened; otherwise as above.
			a.Equal(1, r.Expected)
		},

		{
			contract: "CanGoHere",
			name:     "ShouldPass",
			kind:     contract.KindType,
		}: func(a *assert.Assertions, ctx contract.Context, r *recorder) {
			// We should only see the type.
			a.IsType(&types.Struct{}, ctx.Declaration().(*ssa.Type).Type().Underlying())
			if a.Len(ctx.Objects(), 1) {
				a.Equal(ctx.Declaration(), ctx.Objects()[0])
			}
		},

		{
			contract: "CanGoHere",
			name:     "ReturnOne",
			kind:     contract.KindMethod,
		}: func(a *assert.Assertions, ctx contract.Context, r *recorder) {
			// We should only see the function.
			a.IsType(&ssa.Function{}, ctx.Declaration())
			if a.Len(ctx.Objects(), 1) {
				a.Equal(ctx.Declaration(), ctx.Objects()[0])
			}
		},

		// Verify multiple-contract alias expansion.
		{
			contract: "CanGoHere",
			name:     "HasAlias",
			kind:     contract.KindFunction,
		}: func(a *assert.Assertions, ctx contract.Context, r *recorder) {
			// We should only see the function.
			a.IsType(&ssa.Function{}, ctx.Declaration())
			if a.Len(ctx.Objects(), 1) {
				a.Equal(ctx.Declaration(), ctx.Objects()[0])
			}
		},

		{
			contract: "MustReturnInt",
			name:     "HasAlias",
			kind:     contract.KindFunction,
		}: func(a *assert.Assertions, ctx contract.Context, r *recorder) {
			// We should only see the function.
			a.IsType(&ssa.Function{}, ctx.Declaration())
			a.Equal(2, r.Expected)
			if a.Len(ctx.Objects(), 1) {
				a.Equal(ctx.Declaration(), ctx.Objects()[0])
			}
		},
	}

	newRecorder := &contract.Provider{
		New: func() contract.Contract { return &recorder{t, tcs, -1} },
	}

	e := &Enforcer{
		AssertedInterfaces: true,
		Contracts: contract.Providers{
			"CanGoHere":     newRecorder,
			"MustReturnInt": newRecorder,
		},
		Dir:      "../gen/testdata",
		Logger:   log.New(os.Stdout, "", 0),
		Packages: []string{"."},
		Tests:    true,
	}

	res, err := e.Execute(context.Background())
	a.NoError(err)
	a.Len(res, len(tcs), "invocation / test-case mismatch")
}
