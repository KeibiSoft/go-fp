package chain

import (
	"errors"
	"testing"
)

type MyStruct struct {
	Val int
}

// Methods to be chained:

func AddOne(ms MyStruct) (MyStruct, error) {
	ms.Val += 1
	return ms, nil
}

func FailIfThree(ms MyStruct) (MyStruct, error) {
	if ms.Val == 3 {
		return ms, errors.New("val cannot be 3")
	}
	return ms, nil
}

func MultiplyTwo(ms MyStruct) (MyStruct, error) {
	ms.Val *= 2
	return ms, nil
}

func TestChainWithStruct_Success(t *testing.T) {
	initial := MyStruct{Val: 1}
	result, err := Wrap(initial).
		Then(AddOne).      // Val = 2
		Then(MultiplyTwo). // Val = 4
		Result()

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Val != 4 {
		t.Fatalf("expected Val=4, got %d", result.Val)
	}
}

func TestChainWithStruct_ErrorPropagation(t *testing.T) {
	initial := MyStruct{Val: 2}
	_, err := Wrap(initial).
		Then(AddOne). // Val = 3
		Then(FailIfThree).
		Then(MultiplyTwo). // skipped due to error
		Result()

	if err == nil || err.Error() != "val cannot be 3" {
		t.Fatalf("expected error 'val cannot be 3', got %v", err)
	}
}

func TestChainWithStruct_ReuseOriginal(t *testing.T) {
	initial := MyStruct{Val: 1}

	wrapped := Wrap(initial).
		Then(AddOne).     // Val=2
		Then(MultiplyTwo) // Val=4

	// get result but don't discard original
	result, err := wrapped.Result()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Val != 4 {
		t.Fatalf("expected Val=4, got %d", result.Val)
	}

	// Reuse wrapped to chain again from Val=4
	result2, err2 := wrapped.
		Then(AddOne). // Val=5
		Result()

	if err2 != nil {
		t.Fatalf("expected no error, got %v", err2)
	}
	if result2.Val != 5 {
		t.Fatalf("expected Val=5, got %d", result2.Val)
	}
}

func TestChain_NilFunction(t *testing.T) {
	initial := MyStruct{Val: 1}
	chain := Wrap(initial)

	// Passing nil func should NOT panic and just skip
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Chain panicked on nil function: %v", r)
		}
	}()

	chain2 := chain.Then(nil)
	val, err := chain2.Result()

	if err != nil {
		t.Fatalf("expected no error with nil func, got %v", err)
	}
	if val.Val != 1 {
		t.Fatalf("expected unchanged value 1 with nil func, got %d", val.Val)
	}
}

func TestChain_MapWithErrorChain(t *testing.T) {
	initial := MyStruct{Val: 3} // this will cause error in FailIfThree

	chain := Wrap(initial).
		Then(FailIfThree). // error here
		Map(func(ms MyStruct) MyStruct {
			// This should NOT be called because of error
			ms.Val = 999
			return ms
		})

	val, err := chain.Result()
	if err == nil || err.Error() != "val cannot be 3" {
		t.Fatalf("expected error 'val cannot be 3', got %v", err)
	}
	if val.Val != 3 {
		t.Fatalf("expected Val=3 due to error skipping Map, got %d", val.Val)
	}
}

func TestChain_ReuseOriginalChainAfterError(t *testing.T) {
	initial := MyStruct{Val: 2}

	chain := Wrap(initial).
		Then(AddOne).     // 3
		Then(FailIfThree) // error

	val, err := chain.Result()
	if err == nil || err.Error() != "val cannot be 3" {
		t.Fatalf("expected error 'val cannot be 3', got %v", err)
	}

	if val.Val != 3 {
		t.Fatalf("expected Val=3 on reused chain after error, got %d", val.Val)
	}

	// Try to reuse chain after error: Then should skip functions
	chain2 := chain.Then(AddOne)
	val2, err2 := chain2.Result()

	if err2 == nil || err2.Error() != "val cannot be 3" {
		t.Fatalf("expected error 'val cannot be 3' on reused chain, got %v", err2)
	}
	if val2.Val != 3 {
		t.Fatalf("expected Val=3 on reused chain after error, got %d", val2.Val)
	}
}

func TestChain_ThenFunctionPanics(t *testing.T) {
	initial := MyStruct{Val: 1}
	chain := Wrap(initial)

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic from Then function but none occurred")
		}
	}()

	chain.Then(func(ms MyStruct) (MyStruct, error) {
		panic("panic inside Then function")
	})
}

func TestChain_ThenNilFunctionMultipleTimes(t *testing.T) {
	initial := MyStruct{Val: 1}
	chain := Wrap(initial)

	// chaining multiple nil Then calls should be safe and no panic
	chain = chain.Then(nil).Then(nil).Then(nil)
	val, err := chain.Result()

	if err != nil {
		t.Fatalf("expected no error with multiple nil funcs, got %v", err)
	}
	if val.Val != 1 {
		t.Fatalf("expected unchanged value 1 with multiple nil funcs, got %d", val.Val)
	}
}

func TestChain_MapIsNotCalledOnError(t *testing.T) {
	initial := MyStruct{Val: 2}

	chain := Wrap(initial).
		Then(FailIfThree). // no error, Val=2
		Then(func(ms MyStruct) (MyStruct, error) {
			return MyStruct{Val: 3}, errors.New("forced error")
		}).
		Map(func(ms MyStruct) MyStruct {
			t.Fatal("Map called after error, which should not happen")
			return ms
		})

	val, err := chain.Result()
	if err == nil || err.Error() != "forced error" {
		t.Fatalf("expected forced error, got %v", err)
	}
	if val.Val != 3 {
		t.Fatalf("expected Val=3 after forced error, got %d", val.Val)
	}
}

func TestMap(t *testing.T) {
	c := Wrap(MyStruct{Val: 1})
	mapped := c.Map(func(ms MyStruct) MyStruct {
		ms.Val += 5
		return ms
	})

	if mapped.err != nil {
		t.Fatal("expected no error on Map")
	}
	if mapped.val.Val != 6 {
		t.Fatalf("expected Val=6, got %d", mapped.val.Val)
	}

	// Map should skip if error is present
	withErr := Wrap(MyStruct{Val: 1})
	withErr = Chain[MyStruct]{val: withErr.val, err: errors.New("fail")}
	mapped2 := withErr.Map(func(ms MyStruct) MyStruct {
		ms.Val += 10
		return ms
	})
	if mapped2.err == nil {
		t.Fatal("expected error to propagate in Map")
	}
}

func TestFilter(t *testing.T) {
	errTest := errors.New("filtered out")
	c := Wrap(MyStruct{Val: 2})

	// Predicate true keeps the chain unchanged
	c2 := c.Filter(func(ms MyStruct) bool { return ms.Val == 2 }, errTest)
	if c2.err != nil {
		t.Fatal("expected no error if predicate true")
	}

	// Predicate false sets the error
	c3 := c.Filter(func(ms MyStruct) bool { return ms.Val != 2 }, errTest)
	if c3.err == nil {
		t.Fatal("expected error when predicate is false")
	}
	if c3.err != errTest {
		t.Fatalf("expected error %v, got %v", errTest, c3.err)
	}
}

func TestMatch(t *testing.T) {
	c := Wrap(MyStruct{Val: 42})
	var successCalled, failureCalled bool

	c.Match(func(ms MyStruct) {
		successCalled = true
		if ms.Val != 42 {
			t.Fatalf("expected Val=42 in success, got %d", ms.Val)
		}
	}, func(err error) {
		failureCalled = true
	})

	if !successCalled {
		t.Fatal("expected success callback to be called")
	}
	if failureCalled {
		t.Fatal("did not expect failure callback to be called")
	}

	// Test failure callback called on error
	cErr := Wrap(MyStruct{Val: 0})
	cErr = Chain[MyStruct]{val: cErr.val, err: errors.New("fail")}
	successCalled = false
	failureCalled = false

	cErr.Match(func(_ MyStruct) {
		successCalled = true
	}, func(err error) {
		failureCalled = true
		if err.Error() != "fail" {
			t.Fatalf("expected fail error, got %v", err)
		}
	})

	if successCalled {
		t.Fatal("did not expect success callback")
	}
	if !failureCalled {
		t.Fatal("expected failure callback")
	}

	// Test nil funcs don't panic
	c.Match(nil, nil)
	cErr.Match(nil, nil)
}

func TestOrElse(t *testing.T) {
	c := Wrap(MyStruct{Val: 5})
	// no error, should return same chain
	got := c.OrElse(MyStruct{Val: 99})
	if got.val.Val != 5 {
		t.Fatalf("expected original val 5, got %d", got.val.Val)
	}

	// with error, should replace val but keep error cleared?
	cErr := Chain[MyStruct]{val: MyStruct{Val: 0}, err: errors.New("fail")}
	got2 := cErr.OrElse(MyStruct{Val: 10})
	if got2.err == nil {
		t.Fatal("expected error to remain in OrElse result")
	}
	if got2.val.Val != 10 {
		t.Fatalf("expected val to be replaced to 10, got %d", got2.val.Val)
	}
}

func TestFlatten(t *testing.T) {
	inner := Chain[MyStruct]{val: MyStruct{Val: 7}}
	outer := Chain[Chain[MyStruct]]{val: inner}
	flat := Flatten(outer)
	if flat.err != nil {
		t.Fatalf("expected no error in flatten, got %v", flat.err)
	}
	if flat.val.Val != 7 {
		t.Fatalf("expected val 7 after flatten, got %d", flat.val.Val)
	}

	// error in outer
	outerErr := Chain[Chain[MyStruct]]{err: errors.New("outer error")}
	flat2 := Flatten(outerErr)
	if flat2.err == nil || flat2.err.Error() != "outer error" {
		t.Fatal("expected outer error propagated in Flatten")
	}

	// error in inner
	innerErr := Chain[MyStruct]{err: errors.New("inner error")}
	outer2 := Chain[Chain[MyStruct]]{val: innerErr}
	flat3 := Flatten(outer2)
	if flat3.err == nil || flat3.err.Error() != "inner error" {
		t.Fatal("expected inner error propagated in Flatten")
	}
}

func TestRecover(t *testing.T) {
	c := Wrap(MyStruct{Val: 0})

	// normal case no panic
	res := c.Recover(func() (MyStruct, error) {
		return MyStruct{Val: 100}, nil
	})
	if res.err != nil {
		t.Fatal("expected no error in Recover normal case")
	}
	if res.val.Val != 100 {
		t.Fatalf("expected Val=100, got %d", res.val.Val)
	}

	// panic case
	res2 := c.Recover(func() (MyStruct, error) {
		panic("ouch")
	})
	if res2.err == nil || res2.err.Error() != "panic recovered: ouch" {
		t.Fatalf("expected panic recovered error, got %v", res2.err)
	}

	// if chain already has error, Recover does nothing
	cErr := Chain[MyStruct]{val: MyStruct{}, err: errors.New("fail")}
	res3 := cErr.Recover(func() (MyStruct, error) {
		t.Fatalf("should not call Recover fn if chain has error")
		return MyStruct{}, nil
	})
	if res3.err == nil || res3.err.Error() != "fail" {
		t.Fatal("expected original error to remain in Recover")
	}
}

func TestFilterChains(t *testing.T) {
	chains := []Chain[MyStruct]{
		Wrap(MyStruct{Val: 1}),
		Wrap(MyStruct{Val: 2}),
		Chain[MyStruct]{val: MyStruct{Val: 3}, err: errors.New("fail")},
	}

	filtered := FilterChains(chains, func(ms MyStruct) bool {
		return ms.Val%2 == 0
	})

	if len(filtered) != 1 {
		t.Fatalf("expected 1 filtered chain, got %d", len(filtered))
	}
	if filtered[0].val.Val != 2 {
		t.Fatalf("expected Val=2 in filtered result, got %d", filtered[0].val.Val)
	}

	// nil predicate returns original slice unchanged
	got := FilterChains(chains, nil)
	if len(got) != len(chains) {
		t.Fatalf("expected same length when predicate nil, got %d", len(got))
	}
}

func TestMapReduceChains(t *testing.T) {
	chains := []Chain[MyStruct]{
		Wrap(MyStruct{Val: 1}),
		Wrap(MyStruct{Val: 2}),
		Wrap(MyStruct{Val: 3}),
		Chain[MyStruct]{val: MyStruct{Val: 4}, err: errors.New("fail")},
	}

	sum := MapReduceChains(chains,
		func(ms MyStruct) int { return ms.Val },
		func(a, b int) int { return a + b },
		0)
	if sum != 6 { // 1 + 2 + 3
		t.Fatalf("expected sum 6, got %d", sum)
	}

	// nil mapFn returns zero
	got := MapReduceChains(chains, nil,
		func(a, b int) int { return a + b },
		-1)
	if got != -1 {
		t.Fatal("expected zero value with nil mapFn")
	}

	// nil reduceFn returns zero
	got2 := MapReduceChains(chains,
		func(ms MyStruct) int { return ms.Val }, nil,
		-1)
	if got2 != -1 {
		t.Fatal("expected zero value with nil reduceFn")
	}
}

func TestUnwrap(t *testing.T) {
	c := Wrap(MyStruct{Val: 9})
	got := c.Unwrap()
	if got.Val != 9 {
		t.Fatalf("expected Val=9, got %d", got.Val)
	}

	cErr := Chain[MyStruct]{val: MyStruct{}, err: errors.New("fail")}
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic from Unwrap on error")
		}
	}()
	cErr.Unwrap()
}

func TestIsSuccessFailureHasError(t *testing.T) {
	c := Wrap(MyStruct{Val: 10})
	if !c.IsSuccess() || c.IsFailure() || c.HasError() != nil {
		t.Fatal("unexpected IsSuccess/IsFailure/HasError on no error")
	}

	cErr := Chain[MyStruct]{val: MyStruct{}, err: errors.New("fail")}
	if cErr.IsSuccess() || !cErr.IsFailure() || cErr.HasError() == nil {
		t.Fatal("unexpected IsSuccess/IsFailure/HasError on error")
	}
}

func TestBind(t *testing.T) {
	c := Wrap(MyStruct{Val: 3})

	double := func(ms MyStruct) Chain[MyStruct] {
		ms.Val *= 2
		return Wrap(ms)
	}

	bound := Bind(c, double)
	if bound.err != nil || bound.val.Val != 6 {
		t.Fatal("Bind failed to apply function correctly")
	}

	// error propagates
	cErr := Chain[MyStruct]{val: MyStruct{}, err: errors.New("fail")}
	bound2 := Bind(cErr, double)
	if bound2.err == nil {
		t.Fatal("Bind did not propagate error")
	}

	type Str2 struct{}
	// nil func returns zero Chain
	bound3 := Bind[MyStruct, Str2](c, nil)
	if bound3.err != nil || bound3.val != (Str2{}) {
		t.Fatal("Bind with nil func did not return zero Chain")
	}
}

func TestApply(t *testing.T) {
	c := Wrap(MyStruct{Val: 4})

	incFn := func(ms MyStruct) MyStruct {
		ms.Val++
		return ms
	}
	fnChain := Wrap(incFn)

	applied := Apply(c, fnChain)
	if applied.err != nil || applied.val.Val != 5 {
		t.Fatal("Apply failed to apply function")
	}

	// error in either chain propagates
	cErr := Chain[MyStruct]{val: MyStruct{}, err: errors.New("fail")}
	applied2 := Apply(cErr, fnChain)
	if applied2.err == nil {
		t.Fatal("Apply did not propagate error from c")
	}

	fnChainErr := Chain[func(MyStruct) MyStruct]{val: nil, err: errors.New("fail")}
	applied3 := Apply(c, fnChainErr)
	if applied3.err == nil {
		t.Fatal("Apply did not propagate error from function chain")
	}

	// nil val function returns zero Chain
	nilFnChain := Chain[func(MyStruct) MyStruct]{val: nil}
	applied4 := Apply(c, nilFnChain)
	if applied4.err != nil {
		t.Fatal("Apply with nil function val should not error")
	}
	if applied4.val != (MyStruct{}) {
		t.Fatal("Apply with nil function val should return zero value")
	}
}
