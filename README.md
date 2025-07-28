# go-fp

`go-fp` provides generic, chainable wrappers in Go for semi-functional programming with robust error handling, enabling a clear, composable style of coding.

## Overview
`go-fp` offers two packages:

`immutable`: For immutable value types and chaining pure functions.

`mutable`: For pointer-based types, allowing mutation and error propagation.

The `Wrapper[T]` type wraps values or pointers with embedded error handling and supports chaining with methods such as `Then`, `FlatMap`, and `Match`.

## Installation

Requires Go 1.24+ with Go modules.

## Installation

Use Go modules (Go 1.24+ required):

```
go get github.com/KeibiSoft/go-fp/immutable
```

```
go get github.com/KeibiSoft/go-fp/mutable
```

## Examples

For an immutable client and mutable server implementation check examples folder.


### Immutable

```
package main

import (
    "fmt"
    "github.com/KeibiSoft/go-fp/immutable/chain"
)

type MyStruct struct {
    Value int
}

func (m MyStruct) Inc() (MyStruct, error) {
    m.Value++
    return m, nil
}

func (m MyStruct) Double() (MyStruct, error) {
    m.Value *= 2
    return m, nil
}

func main() {
    initial := MyStruct{Value: 1}

    result, err := chain.Wrap(initial).
        Then(MyStruct.Inc).
        Then(MyStruct.Double).
        Then(MyStruct.Inc).
        Result()

    if err != nil {
        fmt.Println("Error:", err)
        return
    }
    fmt.Println("Result:", result.Value) // Output: Result: 5
}

```

### Mutable

```
package main

import (
    "errors"
    "fmt"
    "github.com/KeibiSoft/go-fp/mutable/chain"
)

type MyStruct struct {
    Value int
}

func (m *MyStruct) Inc() (*MyStruct, error) {
    m.Value++
    return m, nil
}

func (m *MyStruct) Double() (*MyStruct, error) {
    m.Value *= 2
    return m, nil
}

func (m *MyStruct) FailIfFive() (*MyStruct, error) {
    if m.Value == 5 {
        return nil, errors.New("value cannot be 5")
    }
    return m, nil
}

func main() {
    initial := &MyStruct{Value: 1}

    errHandler := func(err error) error {
        fmt.Println("Handled error:", err)
        // Return nil to continue chain despite error, or return err to stop
        return err
    }

    wrapper := chain.New(initial, errHandler)

    result, err := wrapper.
        Then((*MyStruct).Inc).
        Then((*MyStruct).Double).
        Then((*MyStruct).Inc).
        Then((*MyStruct).FailIfFive).
        Result()

    if err != nil {
        fmt.Println("Chain stopped with error:", err)
        return
    }
    fmt.Println("Result:", result.Value)
}

```

# License

MIT License
