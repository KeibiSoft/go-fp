package chain

import "fmt"

// Wrapper provides a chainable wrapper for pointers to T with error handling.
// It supports chaining methods returning (*T, error) in a semi-functional style.
type Wrapper[T any] struct {
	val        *T
	err        error
	errHandler func(error) error
}

// New creates a new Wrapper with an initial value and an optional error handler.
func New[T any](val *T, errHandler func(error) error) Wrapper[T] {
	return Wrapper[T]{val, nil, errHandler}
}

func (w *Wrapper[T]) WithError(err error) *Wrapper[T] {
	w.err = err
	return w
}

// Then calls fn if no error yet, else skips.
// If fn returns error, it is passed to errHandler, which can modify or suppress it.
// If fn is nil, just return the current wrapper unchanged.
func (w Wrapper[T]) Then(fn func(*T) (*T, error)) Wrapper[T] {
	if w.err != nil || fn == nil {
		return w
	}
	newVal, err := fn(w.val)
	if err != nil {
		if w.errHandler != nil {
			err = w.errHandler(err)
		}
		if err != nil {
			// preserve previous value to avoid nil deref
			return Wrapper[T]{w.val, err, w.errHandler}
		}
		// errHandler returned nil, continue with old value
		return w
	}
	return Wrapper[T]{newVal, nil, w.errHandler}
}

// Result returns the wrapped value and the last error encountered.
func (w Wrapper[T]) Result() (*T, error) {
	return w.val, w.err
}

// Map applies a side-effecting function to the wrapped value if no error.
func (w Wrapper[T]) Map(f func(*T)) Wrapper[T] {
	if w.err != nil {
		return w
	}
	f(w.val)
	return w
}

// FlatMap allows chaining with functions returning Wrapper[T].
func (w Wrapper[T]) FlatMap(f func(*T) Wrapper[T]) Wrapper[T] {
	if w.err != nil {
		return w
	}
	return f(w.val)
}

// Match invokes success with the value if no error,
// otherwise invokes failure with the error.
// Nil functions are safely ignored.
func (w Wrapper[T]) Match(success func(*T), failure func(error)) {
	if w.err != nil {
		if failure != nil {
			failure(w.err)
		}
		return
	}

	if success != nil {
		success(w.val)
	}
}

// OrElse returns a new Wrapper with defaultVal and clears the error if any error occurred,
// otherwise returns the original Wrapper unchanged.
func (w Wrapper[T]) OrElse(defaultVal *T) Wrapper[T] {
	if w.err != nil {
		return Wrapper[T]{val: defaultVal, err: nil, errHandler: w.errHandler}
	}
	return w
}

// Flatten collapses a nested Wrapper (Wrapper[Wrapper[U]]) into a single Wrapper[U].
// If the outer or inner wrapper has an error, it propagates that error.
func Flatten[U any](w Wrapper[Wrapper[U]]) Wrapper[U] {
	if w.err != nil {
		return Wrapper[U]{err: w.err}
	}
	inner := w.val
	if inner.err != nil {
		return Wrapper[U]{err: inner.err}
	}
	return Wrapper[U]{val: inner.val, err: nil, errHandler: w.errHandler}
}

// Recover executes fn and recovers from any panic,
// converting it into an error stored in the wrapper.
// If the wrapper already has an error or if fn is nil, it does nothing.
func (w Wrapper[T]) Recover(fn func() (*T, error)) (result Wrapper[T]) {
	if w.err != nil || fn == nil {
		return w
	}

	defer func() {
		if r := recover(); r != nil {
			result = Wrapper[T]{err: fmt.Errorf("panic recovered: %v", r)}
		}
	}()

	val, err := fn()
	result = Wrapper[T]{val: val, err: err, errHandler: w.errHandler}
	return
}

// FilterWrappers returns a slice of Wrapper[T] where predicate is true and no error occurred.
// If wrappers or predicate is nil, it returns the input slice unchanged.
// Nil Wrapper pointers inside the slice are skipped.
func FilterWrappers[T any](wrappers []Wrapper[T], predicate func(*T) bool) []Wrapper[T] {
	if wrappers == nil || predicate == nil {
		return wrappers
	}

	var filtered []Wrapper[T]

	for _, w := range wrappers {
		// Skip wrappers with error or nil value
		if w.err != nil || w.val == nil {
			continue
		}
		if predicate(w.val) {
			filtered = append(filtered, w)
		}
	}

	return filtered
}

// MapReduceWrappers maps each Wrapper's value using mapFn, then reduces the results using reduceFn.
// Returns zero value if wrappers is empty or if mapFn or reduceFn is nil.
// Skips Wrappers with errors or nil pointers in the slice.
func MapReduceWrappers[T any, R any](wrappers []*Wrapper[T], mapFn func(*T) R, reduceFn func(R, R) R, zero R) R {
	if wrappers == nil || mapFn == nil || reduceFn == nil {
		return zero
	}

	var result R
	first := true

	for _, w := range wrappers {
		if w == nil || w.err != nil || w.val == nil {
			continue
		}
		mapped := mapFn(w.val)
		if first {
			result = mapped
			first = false
		} else {
			result = reduceFn(result, mapped)
		}
	}

	return result
}

// Unwrap returns the wrapped value if no error occurred,
// otherwise it panics with the error.
// Similar to Rust's unwrap().
func (w Wrapper[T]) Unwrap() *T {
	if w.err != nil {
		panic(fmt.Sprintf("called Unwrap on error: %v", w.err))
	}
	return w.val
}

// IsSuccess returns true if the wrapper has no error.
func (w Wrapper[T]) IsSuccess() bool {
	return w.err == nil
}

// IsFailure returns true if the wrapper has an error.
func (w Wrapper[T]) IsFailure() bool {
	return w.err != nil
}

// HasError returns the error if any, otherwise nil.
func (w Wrapper[T]) HasError() error {
	return w.err
}

// Bind applies a function that returns a Wrapper[U] to the current Wrapper's value if there is no error.
// It returns a new Wrapper[U] with the result of applying the function.
// If the current Wrapper has an error, Bind propagates it without calling the function.
// If the function f is nil, Bind returns a zero-value Wrapper[U] with no error.
func Bind[T any, U any](w *Wrapper[T], f func(*T) Wrapper[U]) Wrapper[U] {
	if w.err != nil {
		return Wrapper[U]{val: nil, err: w.err, errHandler: w.errHandler}
	}
	if f == nil {
		return Wrapper[U]{val: nil, err: nil, errHandler: w.errHandler}
	}
	return f(w.val)
}

// Apply applies a wrapped function (Wrapper of func(*T) (*U, error)) to the current Wrapper's value if there are no errors.
// It returns a new Wrapper[U] containing the result of applying the function.
// If either the current Wrapper or the function Wrapper has an error, Apply propagates the error and does not call the function.
// If the function Wrapper's value is nil, returns a zero-value Wrapper[U].
func Apply[T any, U any](w *Wrapper[T], f Wrapper[func(*T) (*U, error)]) Wrapper[U] {
	if w.err != nil {
		return Wrapper[U]{val: nil, err: w.err, errHandler: w.errHandler}
	}
	if f.err != nil {
		return Wrapper[U]{val: nil, err: f.err, errHandler: w.errHandler}
	}
	if f.val == nil {
		return Wrapper[U]{val: nil, err: nil, errHandler: w.errHandler}
	}

	newVal, err := (*f.val)(w.val)
	return Wrapper[U]{val: newVal, err: err, errHandler: w.errHandler}
}

// Lift wraps a value into a Wrapper[T] using the provided error handler.
// If the value is nil, it returns a Wrapper with a nil value and no error.
func Lift[T any](v *T, errHandler func(error) error) *Wrapper[T] {
	return &Wrapper[T]{val: v, err: nil, errHandler: errHandler}
}

// LiftM lifts a pure function into the Wrapper monadic context.
// It takes a function f: T -> U and returns a function that transforms
// a Wrapper[T] into a Wrapper[U], applying f only if there is no error.
//
// If the input wrapper has an error, it returns a new Wrapper[U] with the same error.
// If the function f is nil, it returns a zero value Wrapper[U] with no error.
func LiftM[T any, U any](f func(*T) *U) func(Wrapper[T]) Wrapper[U] {
	return func(w Wrapper[T]) Wrapper[U] {
		if w.err != nil {
			return Wrapper[U]{err: w.err}
		}
		if f == nil {
			var zeroU U
			return Wrapper[U]{val: &zeroU, err: nil}
		}
		res := f(w.val)
		return Wrapper[U]{val: res, err: nil, errHandler: w.errHandler}
	}
}

func FlatMapU[T any, U any](w Wrapper[T], f func(*T) Wrapper[U]) Wrapper[U] {
	if w.err != nil {
		return Wrapper[U]{err: w.err}
	}

	if f == nil {
		var zeroU U
		return Wrapper[U]{val: &zeroU, err: nil}
	}

	return f(w.val)
}
