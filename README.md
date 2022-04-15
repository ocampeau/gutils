


# gutils

A collection of Go utilities for Cloud-Native applications.

This package aims at providing a collection of tools to build cloud-native applications.

It's something I do on my own time, mostly for fun and to challenge myself to build
tools that are as efficient as possible (cpu, memory, latency, etc.).

For now, it only contains a circuit breaker, but I plan to add more utilities when I have the time.

I build these tools with a primary focus on two objectives:
* Making the most efficient tools possible (cpu efficient, memory efficient, fast, etc.)
* Making the tools as easy to use as possible

I try to make tools with sensible defaults, so that its easy to get started out of the box
without configuration. I also try to offer as much configurations as possible, and to make
configuration easy to manage.

I also try to benchmark everything, and aim for the fastest solution possible with less
memory allocation as possible (see benchmarks below).

Feel free to use, provide feedback and contribute if you want to.

## Circuit breaker

The `circuitbreaker` package contains a circuit breaker that can be used for almost anything
that involves making request to another component of your application.

### Strategies
Strategies are a way to customize the logic of your circuit breaker when it is in the half-open state.
This package provides only one strategy for now: 
* `halfOpenTimer` strategy.

#### HalfOpenTimer
The halfOpenTimer strategy is a simple timer, that will retry request at periodic intervals
(the timer interval). Once a success threshold is met, the circuit will be closed. If a request returns 
an error, the circuit goes back to the closed state.


#### Custom strategies
It is possible to provide your own strategy by implementing the `Strategy` interface.

### Usage
#### Custom usage
You can use the circuit breaker for any operations by wrapping your operation
inside a closure. 

```go
func createCircuitBreaker(){
  cb := circuitbreaker.NewCircuitBreaker(
    circuitbreaker.WithTimerStrategy(1 * time.Second, 3),
    circuitbreaker.WithOpenDuration(3 * time.Second),
    circuitbreaker.WithFailuresThreshold(5))

  err := cb.Do(operationInClosure())
  if err != nil{
    log.Fatal("some error")
  }
}

// since the circuit breaker only takes a function
// of the type `func() error, you need to wrap your operation
// in a closure, like here
func operationInClosure() func() error  {
  var res = 0
  var err error = nil
  
  op := func() error {
    res, err = someComplexOperation(1, 2, 3)
    return nil
  }
  
  return op
}

// the function you want to call from the circuit breaker
func someComplexOperation(a, b, c int) (int, error) {
  return a + b * c, nil
}
```
#### HTTP requests
One of the most common use case for circuit breaker is for HTTP request. This package
provides an easy to do that by creating an `http.Transport` that delegates the HTTP requests 
to the circuit breaker. See `circuitbreaker/httpcircuit.go` for more details:

```go
func createHttpCircuitBreaker(){
  cb := circuitbreaker.NewHttpTransportCircuitBreaker(http.DefaultTransport)

  httpClient := http.Client{
    Transport:     cb,
  }
  httpClient.Do(&http.Request{})
}
```

#### gRPC requests
Not supported yet, but should come soon.

### Benchmarks

The circuit breaker adds little overhead to a request. As we can see here:

* when the circuit is `open`, it adds between 0 and 2 nanoseconds of overhead to the latency of the request
* when the circuit is `closed`, it adds around 30 nanoseconds of overhead
* when the circuit is `half-closed`, it adds between 0 and 2 nanoseconds of overhead


```bash
goos: darwin
goarch: amd64
pkg: gutils/circuitbreaker
cpu: Intel(R) Core(TM) i7-6920HQ CPU @ 2.90GHz
BenchmarkDoOpen                                         574301286     2.119 ns/op            0 B/op          0 allocs/op
BenchmarkDoOpen-2                                       1000000000    1.008 ns/op            0 B/op          0 allocs/op
BenchmarkDoOpen-4                                       1000000000    0.5261 ns/op           0 B/op          0 allocs/op
BenchmarkDoOpen-8                                       1000000000    0.5207 ns/op           0 B/op          0 allocs/op
BenchmarkDoClose/operation_always_return_nil            145047079     8.814 ns/op            0 B/op          0 allocs/op
BenchmarkDoClose/operation_always_return_nil-2          36377684      30.32 ns/op            0 B/op          0 allocs/op
BenchmarkDoClose/operation_always_return_nil-4          33612763      30.44 ns/op            0 B/op          0 allocs/op
BenchmarkDoClose/operation_always_return_nil-8          50707626      26.77 ns/op            0 B/op          0 allocs/op
BenchmarkDoClose/operation_always_return_error          115203723     10.19 ns/op            0 B/op          0 allocs/op
BenchmarkDoClose/operation_always_return_error-2        38438550      33.46 ns/op            0 B/op          0 allocs/op
BenchmarkDoClose/operation_always_return_error-4        39111943      30.76 ns/op            0 B/op          0 allocs/op
BenchmarkDoClose/operation_always_return_error-8        35006012      28.71 ns/op            0 B/op          0 allocs/op
BenchmarkDoHalfOpen                                     573045352      2.117 ns/op           0 B/op          0 allocs/op
BenchmarkDoHalfOpen-2                                   1000000000     1.033 ns/op           0 B/op          0 allocs/op
BenchmarkDoHalfOpen-4                                   1000000000     0.5272 ns/op          0 B/op          0 allocs/op
BenchmarkDoHalfOpen-8                                   1000000000     0.5199 ns/op          0 B/op          0 allocs/op
```