package example

import (
  "log"
  "time"

  "github.com/ocampeau/gutils/circuitbreaker"
)

func createCircuitBreaker() {
  cb := circuitbreaker.NewCircuitBreaker("myCircuitBreaker",
    circuitbreaker.WithTimerStrategy(1*time.Second, 3),
    circuitbreaker.WithOpenDuration(3*time.Second),
    circuitbreaker.WithFailuresThreshold(5))

  err := cb.Do(operationInClosure())
  if err != nil {
    log.Fatal("some error")
  }
}

// since the circuit breaker only takes a function
// of the type `func() error, you need to wrap your operation
// in a closure, like here
func operationInClosure() func() error {
  var err error = nil

  op := func() error {
    _, err = someComplexOperation(1, 2, 3)
    return err
  }

  return op
}

// the function you want to call from the circuit breaker
func someComplexOperation(a, b, c int) (int, error) {
  return a + b*c, nil
}

func init() {
  createCircuitBreaker()
}
