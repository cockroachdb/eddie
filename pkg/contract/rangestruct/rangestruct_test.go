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
	"fmt"
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/cockroachdb/eddie/pkg/contract"
	"github.com/cockroachdb/eddie/pkg/rt"
	"github.com/stretchr/testify/assert"
)

func Test(t *testing.T) {
	a := assert.New(t)
	testDataDir, err := filepath.Abs("./testdata")
	if !a.NoError(err) {
		return
	}

	e := rt.Enforcer{
		AssertedInterfaces: true,
		Contracts: contract.Providers{
			"RangeValPointer": {New: func() contract.Contract { return &RangeValPointer{} }},
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
	fmt.Println("RES", results)
}
