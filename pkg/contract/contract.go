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

// A Contract implements some correctness-checking logic.
type Contract interface {
	// Enforce will be called on an instance of the Contract automatically
	// by the runtime. Any error returned by this method will be reported
	// against the declaration object.
	Enforce(ctx Context) error
}

// A Provider can construct new instances of a Contract.
type Provider struct {
	Help string
	New  func() Contract
}

// Providers defines a lookup map of providers.
type Providers map[string]*Provider
