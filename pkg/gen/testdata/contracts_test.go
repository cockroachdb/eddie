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

package testdata

import "github.com/cockroachdb/eddie/pkg/contract"

// The framework will look for these kinds of type-assertion
// declarations when looking for structs that implement an interface
// which participates in a contract.
var (
	_ ReturnsNumber = ShouldPass{}
	_ ReturnsNumber = ShouldFail{}
)

// ReturnsNumber defines a contract on its only method.
//contract:CanGoHere
type ReturnsNumber interface {
	// This is a normal doc-comment, except that it has magic
	// comments below, consisting of a contract name and a
	// JSON block which will be unmarshalled into the contract
	// struct instance.
	//
	//contract:CanGoHere
	/* contract:MustReturnInt {
		"Expected" : 1
	} */
	ReturnOne() int
}

type (
	//contract:CanGoHere
	ShouldPass struct{}
)

//contract:CanGoHere
func (ShouldPass) ReturnOne() int {
	return 1
}

type ShouldFail struct{}

func (ShouldFail) ReturnOne() int {
	return 0
}

//contract:CanGoHere
//contract:MustReturnInt { "Expected" : 2 }
type AnAlias contract.Contract

//contract:AnAlias
func HasAlias() {}

func Example() {}
