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

import "errors"

type Selfish interface {
	//contract:ReturnConcrete
	Self() error
}

type BadError struct{}

func (*BadError) Error() string { return "Bad" }
func (e *BadError) Self() error { return e }

type GoodPtrError struct{}

func (*GoodPtrError) Error() string { return "Good" }
func (e *GoodPtrError) Self() error { return e }

type GoodValError struct{}

func (GoodValError) Error() string { return "Good" }

type ReturnsGood interface {
	// All known implementors of this method return a good value.
	//contract:ReturnConcrete
	error() error
}

type ReturnsGoodImpl struct{}

func (ReturnsGoodImpl) error() error {
	return &GoodPtrError{}
}

type ReturnsGoodImpl2 struct{}

func (ReturnsGoodImpl2) error() error { return GoodValError{} }

var (
	_ error       = &GoodPtrError{}
	_ error       = GoodValError{}
	_ Selfish     = &BadError{}
	_ Selfish     = &GoodPtrError{}
	_ ReturnsGood = ReturnsGoodImpl{}
	_ ReturnsGood = ReturnsGoodImpl2{}
)

//contract:ReturnConcrete
func DirectBad() error {
	return errors.New("nope")
}

//contract:ReturnConcrete
func DirectGood() error {
	switch choose() {
	case 0:
		return GoodValError{}
	case 1:
		return &GoodPtrError{}
	default:
		panic("fake code")
	}
}

//contract:ReturnConcrete
func DirectTupleBad() (int, error) {
	return 0, errors.New("nope")
}

// Calls another function with the contract, so it's still good.
//contract:ReturnConcrete
func DirectTupleBadCallerIsGood() error {
	_, err := DirectTupleBad()
	return err
}

//contract:ReturnConcrete
func DirectTupleBadChainButIsStillGood() (int, error) {
	x, err := DirectTupleBad()
	return x + 1, err
}

//contract:ReturnConcrete
func EnsureGoodValWithCommaOk(err error) error {
	if tested, ok := err.(GoodValError); ok {
		return tested
	}
	return GoodValError{}
}

//contract:ReturnConcrete
func ExplictReturnVarNoOp() (err error) {
	return
}

//contract:ReturnConcrete
func ExplicitReturnVarBadButIsStillGood() (err error) {
	err = DirectBad()
	return
}

//contract:ReturnConcrete
func ExplicitReturnVarGood() (err error) {
	err = DirectGood()
	return
}

//contract:ReturnConcrete
func ExplicitReturnVarPhiBadButIsStillGood() (err error) {
	switch choose() {
	case 0:
		err = DirectBad()
	case 1:
		err = DirectGood()
	}
	return
}

//contract:ReturnConcrete
func ExplicitReturnVarPhiGood() (err error) {
	switch choose() {
	case 0:
		err = DirectGood()
	case 1:
		err = &GoodValError{}
	case 2:
		err = &GoodPtrError{}
	}
	return
}

//contract:ReturnConcrete
func EnsureGoodValWithSwitch(err error) error {
	switch t := err.(type) {
	case GoodValError:
		return t
	case *GoodPtrError:
		return t
	default:
		return GoodValError{}
	}
}

//contract:ReturnConcrete
func MakesIndirectCall(fn func() error) error {
	return fn()
}

// Selfish.Self is marked with the contract, so this function
// is still good.
//contract:ReturnConcrete
func MakesInterfaceCallBadButIsStillGood(g Selfish) error {
	return g.Self()
}

//contract:ReturnConcrete
func MakesInterfaceCallGood(g ReturnsGood) error {
	return g.error()
}

//contract:ReturnConcrete
func NoopGood() {}

//contract:ReturnConcrete
func NoopCallGood() {
	NoopGood()
}

//contract:ReturnConcrete
func PhiBad() error {
	var ret error
	switch choose() {
	case 0:
		ret = GoodValError{}
	case 1:
		ret = &GoodPtrError{}
	case 2:
		ret = DirectGood()
	case 3:
		ret = DirectBad()
	case 4:
		ret = errors.New("Nope")
	default:
		panic("fake code")
	}
	return ret
}

//contract:ReturnConcrete
func PhiGood() error {
	var ret error
	switch choose() {
	case 0:
		ret = GoodValError{}
	case 1:
		ret = &GoodPtrError{}
	case 2:
		ret = DirectGood()
	default:
		panic("fake code")
	}
	return ret
}

// ShortestWhyPath demonstrates that we choose the shortest "why" path.
//contract:ReturnConcrete
func ShortestWhyPath() error {
	switch choose() {
	case 0:
		return &BadError{}
	case 1:
		return errors.New("shouldn't see this")
	default:
		return PhiBad()
	}
}

//contract:ReturnConcrete
func ReturnNilGood() error {
	return nil
}

// Trying to run down this particular form, where we don't return the
// value of the TypeAssert has proven to be excessively convoluted
// to get right.
//contract:ReturnConcrete
func TodoNoTypeInference(err error) error {
	if _, ok := err.(GoodValError); ok {
		return err
	}
	return GoodValError{}
}

// Since BadError.Self is supposed to pass the contract, this function
// will be marked as good.
//contract:ReturnConcrete
func UsesSelfBadButIsStillGood() error {
	return (&BadError{}).Self()
}

//contract:ReturnConcrete
func UsesSelfGood() error {
	return (&GoodPtrError{}).Self()
}

func choose() int {
	return -1
}
