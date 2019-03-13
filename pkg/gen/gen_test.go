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

package gen

import (
	"os"
	"os/exec"
	"plugin"
	"testing"

	"github.com/cockroachdb/eddie/pkg/rt"

	"github.com/stretchr/testify/assert"
)

// This test will compile an enforcer from the contents of the demo
// directory as a go plugin, then dynamically load it to continue
// testing.
func TestCompileAndLoad(t *testing.T) {
	a := assert.New(t)

	// Set up a temp file to hold the generated file.
	if !a.NoError(os.MkdirAll("./testdata/cmd", 0755)) {
		return
	}

	e := Eddie{
		Dir:      "./testdata",
		Name:     "gen_test",
		Outfile:  "./testdata/cmd/eddie.go",
		Packages: []string{"."},
		Plugin:   true,
	}
	a.NoError(e.Execute())

	a.Len(e.contracts, 2)

	cmd := exec.Command("go", "build", "--buildmode", "plugin", "-o", "eddie", ".")
	cmd.Dir = "./testdata/cmd"
	cmd.Env = os.Environ()
	if output, err := cmd.CombinedOutput(); err != nil {
		a.NoError(err, string(output))
	}
	plg, err := plugin.Open("./testdata/cmd/eddie")
	if err != nil {
		// Not available on all platforms
		if err.Error() == "plugin: not implemented" {
			return
		}
		a.NoError(err)
		return
	}
	sym, err := plg.Lookup("Enforcer")
	if !a.NoError(err) {
		return
	}
	enf := sym.(*rt.Enforcer)
	a.Len(enf.Contracts, 2)
}
