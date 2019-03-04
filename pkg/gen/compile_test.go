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
	"io/ioutil"
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
	exe, err := ioutil.TempFile("", "eddie")
	if !a.NoError(err) {
		return
	}

	e := Eddie{
		Dir:      "./testdata",
		Name:     "gen_test",
		Outfile:  exe.Name(),
		Packages: []string{"."},
		Plugin:   true,
	}
	a.NoError(e.Execute())

	// Now try to open the plugin. This is only (currently) supported on
	// mac and linux platforms.
	plg, err := plugin.Open(exe.Name())
	if err != nil && err.Error() == "plugin: not implemented" {
		t.SkipNow()
	}
	if !a.NoError(err) {
		return
	}
	// Look for the top-level var in the generated code.
	sym, err := plg.Lookup("Enforcer")
	if !a.NoError(err) {
		return
	}

	impl := sym.(*rt.Enforcer)
	a.Len(impl.Contracts, 2)
}
