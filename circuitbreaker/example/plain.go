package example

import (
  "gutils/circuitbreaker"
  "log"
  "time"
)

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
