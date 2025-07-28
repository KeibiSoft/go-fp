package chain

import "fmt"

// Chain provides a generic chainable wrapper with error handling.
// It supports chaining functions returning (T, error) in a semi-functional style
type Chain[T any] struct {
	val T
	err error
}

// Wrap creates a new Chain wrapping the given value.
func Wrap[T any](v T) Chain[T] {
	return Chain[T]{val: v}
}

func (c Chain[T]) WithError(err error) Chain[T] {
	c.err = err
	return c
}

// Then calls f if no error yet, else skips.
// f returns the updated value and optional error.
// If the function is nil, return old value unchanged.
func (c Chain[T]) Then(f func(T) (T, error)) Chain[T] {
	if c.err != nil || f == nil {
		return c
	}
	newVal, err := f(c.val)
	return Chain[T]{val: newVal, err: err}
}

// Result returns the final value and error of the chain.
func (c Chain[T]) Result() (T, error) {
	return c.val, c.err
}

// Map applies f to the value if no error, ignoring errors.
func (c Chain[T]) Map(f func(T) T) Chain[T] {
	if c.err != nil {
		return c
	}
	return Chain[T]{val: f(c.val)}
}

func (c Chain[T]) Filter(pred func(T) bool, err error) Chain[T] {
	if c.err != nil {
		return c
	}
	if !pred(c.val) {
		return Chain[T]{c.val, err}
	}

	return c
}

// Match invokes success with the value if no error,
// otherwise invokes failure with the error.
// Nil functions are safely ignored.
func (c Chain[T]) Match(success func(T), failure func(error)) {
	if c.err != nil {
		if failure != nil {
			failure(c.err)
		}
		return
	}

	if success != nil {
		success(c.val)
	}
}

// OrElse returns a new Chain with defaultVal if there was an error,
// otherwise returns the original Chain unchanged.
// it replaces the value, but it does not clear the error.
func (c Chain[T]) OrElse(defaultVal T) Chain[T] {
	if c.err != nil {
		return Chain[T]{val: defaultVal, err: c.err} // preserve error
	}
	return c
}

// Flatten collapses a nested Chain (Chain[Chain[U]]) into a single Chain[U].
// If the outer or inner chain has an error, it propagates that error.
func Flatten[U any](c Chain[Chain[U]]) Chain[U] {
	if c.err != nil {
		return Chain[U]{err: c.err}
	}
	inner := c.val
	if inner.err != nil {
		return Chain[U]{err: inner.err}
	}
	return Chain[U]{val: inner.val}
}

// Recover executes fn and recovers from any panic,
// converting it into an error stored in the chain.
// If the chain already has an error or if fn is nil, it does nothing.
func (c Chain[T]) Recover(fn func() (T, error)) (result Chain[T]) {
	if c.err != nil || fn == nil {
		return c
	}

	defer func() {
		if r := recover(); r != nil {
			result = Chain[T]{err: fmt.Errorf("panic recovered: %v", r)}
		}
	}()

	val, err := fn()
	result = Chain[T]{val: val, err: err}
	return
}

// FilterChains returns a slice of Chain[T] where predicate is true and no error occurred.
// If chains or predicate is nil, it returns the input slice unchanged.
// Nil Chain pointers inside the slice are skipped.
func FilterChains[T any](chains []Chain[T], predicate func(T) bool) []Chain[T] {
	if chains == nil || predicate == nil {
		return chains
	}

	var filtered []Chain[T]

	for _, c := range chains {
		if c.err != nil {
			continue
		}
		if predicate(c.val) {
			filtered = append(filtered, c)
		}
	}

	return filtered
}

// MapReduceChains maps each Chain's value using mapFn, then reduces the results using reduceFn.
// Returns zero value if chains is empty or if mapFn or reduceFn is nil.
// Skips Chains with errors in the slice.
func MapReduceChains[T any, R any](chains []Chain[T], mapFn func(T) R, reduceFn func(R, R) R, zero R) R {
	if chains == nil || mapFn == nil || reduceFn == nil {
		return zero
	}

	var result R
	first := true

	for _, c := range chains {
		if c.err != nil {
			continue
		}
		mapped := mapFn(c.val)
		if first {
			result = mapped
			first = false
		} else {
			result = reduceFn(result, mapped)
		}
	}

	return result
}

// Unwrap returns the contained value if no error occurred,
// otherwise it panics with the error.
// Similar to Rust's unwrap().
func (c Chain[T]) Unwrap() T {
	if c.err != nil {
		panic(fmt.Sprintf("called Unwrap on error: %v", c.err))
	}
	return c.val
}

// IsSuccess returns true if the chain has no error.
func (c Chain[T]) IsSuccess() bool {
	return c.err == nil
}

// IsFailure returns true if the chain has an error.
func (c Chain[T]) IsFailure() bool {
	return c.err != nil
}

// HasError returns the error if any, otherwise nil.
func (c Chain[T]) HasError() error {
	return c.err
}

// Bind applies a function that returns a Chain[U] to the current Chain's value if there is no error.
// It returns a new Chain[U] with the result of applying the function.
// If the current Chain has an error, Bind propagates it without calling the function.
// If the function f is nil, Bind returns the original Chain converted to Chain[U] with zero value U.
func Bind[T any, U any](c Chain[T], f func(T) Chain[U]) Chain[U] {
	if c.err != nil {
		return Chain[U]{err: c.err}
	}
	if f == nil {
		// Can't apply nil function; return zero value with no error.
		var zeroU U
		return Chain[U]{val: zeroU}
	}
	return f(c.val)
}

// Apply applies a wrapped function (Chain of func(T) U) to the current Chain's value if there are no errors.
// It returns a new Chain[U] containing the result of applying the function.
// If either the current Chain or the function Chain has an error, Apply propagates the error and does not call the function.
// If the function Chain's value is nil, or if the function Chain itself is nil, returns a zero value Chain[U].
func Apply[T any, U any](c Chain[T], f Chain[func(T) U]) Chain[U] {
	if c.err != nil {
		return Chain[U]{err: c.err}
	}
	if f.err != nil {
		return Chain[U]{err: f.err}
	}
	if f.val == nil {
		var zeroU U
		return Chain[U]{val: zeroU}
	}
	return Chain[U]{val: f.val(c.val)}
}

func Lift[T any](v T) Chain[T] {
	return Wrap(v)
}

// LiftResult lifts a function that returns (T, error) into a Chain[T].
// If the function is nil, return empty Chain.
func LiftResult[T any](fn func() (T, error)) Chain[T] {
	if fn == nil {
		return Chain[T]{}
	}

	val, err := fn()
	if err != nil {
		return Wrap(val).WithError(err)
	}
	return Wrap(val)
}

// LiftM lifts a pure function into the Chain monadic context.
// It takes a function f: T -> U and returns a function that transforms
// a Chain[T] into a Chain[U], applying f only if the Chain has no error.
//
// If the input chain has an error, it returns a new Chain[U] with the same error.
// If the function f is nil, it returns a zero value Chain[U] with no error.
func LiftM[T any, U any](f func(T) U) func(Chain[T]) Chain[U] {
	return func(c Chain[T]) Chain[U] {
		if c.err != nil {
			return Chain[U]{err: c.err}
		}
		if f == nil {
			var zeroU U
			return Chain[U]{val: zeroU}
		}
		return Wrap(f(c.val))
	}
}
