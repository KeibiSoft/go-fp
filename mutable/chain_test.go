package chain

import (
	"errors"
	"testing"
)

type MyStruct struct {
	Val int
}

func (m *MyStruct) Inc() (*MyStruct, error) {
	m.Val++
	return m, nil
}

func (m *MyStruct) Double() (*MyStruct, error) {
	m.Val *= 2
	return m, nil
}

func (m *MyStruct) FailIfThree() (*MyStruct, error) {
	if m.Val == 3 {
		return nil, errors.New("val cannot be 3")
	}
	return m, nil
}

func TestWrapper_Success(t *testing.T) {
	ms := &MyStruct{Val: 1}
	final, err := New(ms, nil).
		Then((*MyStruct).Inc).    // 2
		Then((*MyStruct).Double). // 4
		Result()

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if final.Val != 4 {
		t.Fatalf("expected value 4, got %d", final.Val)
	}
}

func TestWrapper_ErrorStopsChain(t *testing.T) {
	ms := &MyStruct{Val: 2}
	final, err := New(ms, func(e error) error { return e }). // propagate error
									Then((*MyStruct).Inc).         // 3
									Then((*MyStruct).FailIfThree). // error
									Then((*MyStruct).Double).      // skipped
									Result()

	if err == nil || err.Error() != "val cannot be 3" {
		t.Fatalf("expected error 'val cannot be 3', got %v", err)
	}
	if final.Val != 3 {
		t.Fatalf("expected value 3, got %d", final.Val)
	}
}

func TestWrapper_IgnoreErrorAndContinue(t *testing.T) {
	ms := &MyStruct{Val: 2}
	final, err := New(ms, func(e error) error { return e }).
		Then((*MyStruct).Inc).         // 3
		Then((*MyStruct).FailIfThree). // error
		Then((*MyStruct).Double).      // skipped
		Result()

	if err == nil || err.Error() != "val cannot be 3" {
		t.Fatalf("expected error 'val cannot be 3', got %v", err)
	}
	if final.Val != 3 {
		t.Fatalf("expected value 3, got %d", final.Val)
	}
}

func TestWrapper_Map(t *testing.T) {
	ms := &MyStruct{Val: 1}
	final, err := New(ms, nil).
		Map(func(m *MyStruct) {
			m.Val += 10
		}).
		Result()

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if final.Val != 11 {
		t.Fatalf("expected value 11, got %d", final.Val)
	}
}

func TestWrapper_FlatMap_Success(t *testing.T) {
	ms := &MyStruct{Val: 1}
	final, err := New(ms, nil).
		FlatMap(func(m *MyStruct) Wrapper[MyStruct] {
			m.Val *= 3
			return New(m, nil)
		}).
		Result()

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if final.Val != 3 {
		t.Fatalf("expected value 3, got %d", final.Val)
	}
}

func TestWrapper_FlatMap_ErrorPropagation(t *testing.T) {
	ms := &MyStruct{Val: 3}
	final, err := New(ms, nil).
		FlatMap(func(m *MyStruct) Wrapper[MyStruct] {
			if m.Val == 3 {
				return Wrapper[MyStruct]{val: m, err: errors.New("fail on 3")}
			}
			return New(m, nil)
		}).
		Then((*MyStruct).Inc).
		Result()

	if err == nil || err.Error() != "fail on 3" {
		t.Fatalf("expected error 'fail on 3', got %v", err)
	}
	if final.Val != 3 {
		t.Fatalf("expected value 3, got %d", final.Val)
	}
}

func TestWrapper_NilValueHandling(t *testing.T) {
	var ms *MyStruct = nil
	w := New(ms, nil)
	// Map should handle nil pointer gracefully
	w = w.Map(func(m *MyStruct) {
		if m != nil {
			m.Val = 999
		}
	})
	val, err := w.Result()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if val != nil {
		t.Fatalf("expected nil value, got %v", val)
	}
}

func TestWrapper_ErrorHandlerReturnsNil(t *testing.T) {
	ms := &MyStruct{Val: 3}
	final, err := New(ms, func(e error) error {
		// ignore error and continue
		return nil
	}).
		Then((*MyStruct).FailIfThree).
		Then((*MyStruct).Inc).
		Result()

	if err != nil {
		t.Fatalf("expected no error due to error handler, got %v", err)
	}
	if final.Val != 4 {
		t.Fatalf("expected value 4 after ignoring error, got %d", final.Val)
	}
}

func TestWrapper_ThenWithNilFunc(t *testing.T) {
	ms := &MyStruct{Val: 1}
	w := New(ms, nil)
	// passing nil func to Then should not panic but simply skip
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Then panicked with nil function: %v", r)
		}
	}()
	w = w.Then(nil)
	val, err := w.Result()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if val.Val != 1 {
		t.Fatalf("expected value 1, got %d", val.Val)
	}
}

func TestMatch(t *testing.T) {
	ms := &MyStruct{Val: 10}

	// success callback called when no error
	w := New(ms, nil)
	called := false
	w.Match(func(m *MyStruct) {
		called = true
		if m.Val != 10 {
			t.Fatalf("expected Val=10, got %d", m.Val)
		}
	}, nil)
	if !called {
		t.Fatal("expected success callback to be called")
	}

	// failure callback called when error present
	errWrapper := Wrapper[MyStruct]{val: ms, err: errors.New("fail"), errHandler: nil}
	failed := false
	errWrapper.Match(nil, func(err error) {
		failed = true
		if err.Error() != "fail" {
			t.Fatalf("expected error 'fail', got %v", err)
		}
	})
	if !failed {
		t.Fatal("expected failure callback to be called")
	}

	// nil callbacks do not panic
	w.Match(nil, nil)
	errWrapper.Match(nil, nil)
}

func TestOrElse(t *testing.T) {
	orig := &MyStruct{Val: 5}
	def := &MyStruct{Val: 100}

	w := New(orig, nil)
	// no error, original retained
	res := w.OrElse(def)
	if res.val != orig || res.err != nil {
		t.Fatal("expected original Wrapper unchanged when no error")
	}

	// with error, default used and error cleared
	wErr := Wrapper[MyStruct]{val: orig, err: errors.New("fail"), errHandler: nil}
	res2 := wErr.OrElse(def)
	if res2.val != def {
		t.Fatal("expected default value after OrElse on error")
	}
	if res2.err != nil {
		t.Fatal("expected error cleared after OrElse")
	}
}

func TestFlatten(t *testing.T) {
	inner := New(&MyStruct{Val: 2}, nil)
	outer := New(&inner, nil)

	flat := Flatten(outer)
	if flat.err != nil || flat.val.Val != 2 {
		t.Fatalf("expected flattened value 2 with no error, got val=%v err=%v", flat.val, flat.err)
	}

	// error on outer
	outerErr := Wrapper[Wrapper[MyStruct]]{val: &inner, err: errors.New("outer error")}
	flatErr := Flatten(outerErr)
	if flatErr.err == nil || flatErr.err.Error() != "outer error" {
		t.Fatalf("expected outer error propagated, got %v", flatErr.err)
	}

	// error on inner
	innerErr := Wrapper[MyStruct]{val: &MyStruct{Val: 3}, err: errors.New("inner error")}
	outer2 := New(&innerErr, nil)
	flatErr2 := Flatten(outer2)
	if flatErr2.err == nil || flatErr2.err.Error() != "inner error" {
		t.Fatalf("expected inner error propagated, got %v", flatErr2.err)
	}
}

func TestRecover(t *testing.T) {
	ms := &MyStruct{Val: 1}
	w := New(ms, nil)

	// normal no panic
	res := w.Recover(func() (*MyStruct, error) {
		return &MyStruct{Val: 42}, nil
	})
	if res.err != nil {
		t.Fatalf("expected no error from Recover, got %v", res.err)
	}
	if res.val.Val != 42 {
		t.Fatalf("expected val 42, got %d", res.val.Val)
	}

	// panic caught
	res2 := w.Recover(func() (*MyStruct, error) {
		panic("oh no")
	})
	if res2.err == nil || res2.err.Error() != "panic recovered: oh no" {
		t.Fatalf("expected panic recovered error, got %v", res2.err)
	}

	// Recover does nothing if error already present
	wErr := Wrapper[MyStruct]{val: ms, err: errors.New("fail")}
	res3 := wErr.Recover(func() (*MyStruct, error) {
		t.Fatalf("fn should not be called when error present")
		return nil, nil
	})
	if res3.err == nil || res3.err.Error() != "fail" {
		t.Fatalf("expected original error retained, got %v", res3.err)
	}

	// Recover does nothing if fn nil
	res4 := w.Recover(nil)
	if res4.err != nil {
		t.Fatalf("expected no error, got %v", res4.err)
	}
}

func TestFilterWrappers(t *testing.T) {
	ms1 := &MyStruct{Val: 1}
	ms2 := &MyStruct{Val: 2}
	ms3 := &MyStruct{Val: 3}

	ws := []Wrapper[MyStruct]{
		New(ms1, nil),
		New(ms2, nil),
		{val: ms3, err: errors.New("fail")},
		{val: nil, err: nil}, // nil val should be skipped
	}

	filtered := FilterWrappers(ws, func(m *MyStruct) bool {
		return m.Val%2 == 0
	})

	if len(filtered) != 1 || filtered[0].val.Val != 2 {
		t.Fatalf("expected only Val=2 after filter, got %v", filtered)
	}

	// nil wrappers or predicate returns original slice
	if got := FilterWrappers(nil, func(m *MyStruct) bool { return true }); got != nil {
		t.Fatal("expected nil input to return nil")
	}

	got := FilterWrappers(ws, nil)
	if len(got) != len(ws) {
		t.Fatal("expected nil predicate to return original slice")
	}

	for i := range got {
		if got[i].err == nil {
			if ws[i].err != nil {
				t.Fatal("expected nil predicate to return original slice")
			}
		}

		if ws[i].err == nil {
			if got[i].err != nil {
				t.Fatal("expected nil predicate to return original slice")
			}
		}

		if got[i].err != nil && got[i].err.Error() != ws[i].err.Error() {
			t.Fatal("expected nil predicate to return original slice")
		}

		if ws[i].val == nil && got[i].val == nil {
			// both nil, OK
			continue
		}

		if got[i].val == nil {
			if ws[i].val != nil {
				t.Fatal("expected nil predicate to return original slice")
			}
		}

		if ws[i].val == nil {
			if got[i].val != nil {
				t.Fatal("expected nil predicate to return original slice")
			}
		}

		if ws[i].val.Val != got[i].val.Val {
			t.Fatal("expected nil predicate to return original slice")
		}
	}

}

func TestMapReduceWrappers(t *testing.T) {
	ms1 := &MyStruct{Val: 1}
	ms2 := &MyStruct{Val: 2}
	ms3 := &MyStruct{Val: 3}

	w1 := New(ms1, nil)
	w2 := New(ms2, nil)
	w3 := New(ms3, nil)
	wErr := Wrapper[MyStruct]{val: ms3, err: errors.New("fail")}

	// sum values using MapReduce
	wrappers := []*Wrapper[MyStruct]{&w1, &w2, &w3, &wErr, nil}

	sum := MapReduceWrappers(wrappers,
		func(m *MyStruct) int { return m.Val },
		func(a, b int) int { return a + b },
		0)

	if sum != 6 {
		t.Fatalf("expected sum 6, got %d", sum)
	}

	// zero return on nil inputs or funcs
	if MapReduceWrappers[int, int](nil, nil, nil, 999) != 999 {
		t.Fatal("expected zero value on nil inputs")
	}
	if MapReduceWrappers(wrappers, nil, nil, 999) != 999 {
		t.Fatal("expected zero value on nil funcs")
	}
}

func TestUnwrap(t *testing.T) {
	ms := &MyStruct{Val: 5}
	w := New(ms, nil)

	got := w.Unwrap()
	if got != ms {
		t.Fatal("expected Unwrap to return value")
	}

	wErr := Wrapper[MyStruct]{val: ms, err: errors.New("fail")}
	defer func() {
		r := recover()
		if r == nil || r.(string) != "called Unwrap on error: fail" {
			t.Fatalf("expected panic on Unwrap error, got %v", r)
		}
	}()
	wErr.Unwrap()
}

func TestIsSuccessIsFailureHasError(t *testing.T) {
	w := Wrapper[MyStruct]{val: &MyStruct{}, err: nil}
	if !w.IsSuccess() || w.IsFailure() {
		t.Fatal("expected IsSuccess true and IsFailure false for no error")
	}
	if w.HasError() != nil {
		t.Fatal("expected HasError to be nil")
	}

	wErr := Wrapper[MyStruct]{val: &MyStruct{}, err: errors.New("fail")}
	if wErr.IsSuccess() || !wErr.IsFailure() {
		t.Fatal("expected IsSuccess false and IsFailure true for error")
	}
	if wErr.HasError() == nil {
		t.Fatal("expected HasError to be non-nil")
	}
}

func TestBind(t *testing.T) {
	ms := &MyStruct{Val: 2}
	w := New(ms, nil)

	double := func(m *MyStruct) Wrapper[int] {
		return New(&[]int{m.Val * 2}[0], nil)
	}

	// normal bind
	result := Bind(&w, double)
	if result.err != nil {
		t.Fatal("expected no error")
	}
	if *result.val != 4 {
		t.Fatalf("expected val 4, got %v", *result.val)
	}

	// error propagates without calling func
	wErr := Wrapper[MyStruct]{val: ms, err: errors.New("fail")}
	called := false
	result2 := Bind(&wErr, func(m *MyStruct) Wrapper[int] {
		called = true
		return New(new(int), nil)
	})
	if called {
		t.Fatal("expected function not called on error")
	}
	if result2.err == nil || result2.err.Error() != "fail" {
		t.Fatal("expected original error propagated")
	}

	type Str2 struct{}

	// nil function returns zero-value Wrapper
	result3 := Bind[MyStruct, Str2](&w, nil)
	if result3.err != nil || result3.val != nil {
		t.Fatal("expected zero-value Wrapper when func nil")
	}
}

func TestApply(t *testing.T) {
	ms := &MyStruct{Val: 3}
	w := New(ms, nil)

	f := func(m *MyStruct) (*int, error) {
		val := m.Val * 10
		return &val, nil
	}

	funcWrapper := New(&f, nil)

	// apply works normally
	res := Apply(&w, funcWrapper)
	if res.err != nil {
		t.Fatal("expected no error")
	}
	if *res.val != 30 {
		t.Fatalf("expected val 30, got %v", *res.val)
	}

	// error in wrapper propagates
	wErr := Wrapper[MyStruct]{val: ms, err: errors.New("fail")}
	res2 := Apply(&wErr, funcWrapper)
	if res2.err == nil || res2.err.Error() != "fail" {
		t.Fatal("expected error propagated from input wrapper")
	}

	// error in func wrapper propagates
	funcWrapperErr := Wrapper[func(*MyStruct) (*int, error)]{val: &f, err: errors.New("func fail")}
	res3 := Apply(&w, funcWrapperErr)
	if res3.err == nil || res3.err.Error() != "func fail" {
		t.Fatal("expected error propagated from func wrapper")
	}

	// nil function value returns zero-value Wrapper
	funcWrapperNil := Wrapper[func(*MyStruct) (*int, error)]{val: nil}
	res4 := Apply(&w, funcWrapperNil)
	if res4.err != nil || res4.val != nil {
		t.Fatal("expected zero-value Wrapper when func is nil")
	}
}
