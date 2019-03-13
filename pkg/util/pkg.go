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

// Package pkg defines a constant.
package util

import (
	"go/types"
	"strings"
)

// Base is the path of this package.  We need this because we
// are looking for the use of specific types in user-written code.
// Perhaps in go1.12, the new BuildInfo api could provide this.
const Base = "github.com/cockroachdb/eddie/pkg/"

// InPackage checks to see if the given object is declared in a package
// with the given path or is in a vendored version of the same.
func InPackage(obj types.Object, path string) bool {
	pkg := obj.Pkg().Path()
	if pkg == path {
		return true
	}
	if strings.HasSuffix(pkg, "/vendor/"+path) {
		return true
	}
	return false
}
