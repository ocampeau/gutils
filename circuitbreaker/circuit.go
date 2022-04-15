package circuitbreaker

import (
  "errors"
  "gutils/circuitbreaker/strategy"
  "sync/atomic"
  "time"
)

type OnStateChangeHook = func()
type Op = func() error
type Options func(breaker *CircuitBreaker)

var (
  ErrCircuitOpen     = errors.New("http circuit breaker is open")
  ErrCircuitInternal = errors.New("internal error with circuit breaker")

  noopHook = func() {}
)

const (
  Open = iota
  Closed
  HalfOpen

  DefaultOpenTimerDuration                 = 3 * time.Second
  DefaultHalfOpenTimerDuration             = 2 * time.Second
  DefaultHalfOpenConsecutiveSuccess uint32 = 3
)

type CircuitBreaker struct {
  openDuration                 time.Duration
  halfOpenStrategy             strategy.Strategy
  consecutiveFailures          uint32
  consecutiveFailuresThreshold uint32
  state                        uint32
  openHook                     OnStateChangeHook
  halfOpenHook                 OnStateChangeHook
  closeHook                    OnStateChangeHook
}

func NewCircuitBreaker(opts ...Options) *CircuitBreaker {
  c := &CircuitBreaker{
    openDuration:                 DefaultOpenTimerDuration,
    consecutiveFailuresThreshold: 5,
    halfOpenStrategy: strategy.NewTimerStrategy(
      DefaultHalfOpenTimerDuration,
      DefaultHalfOpenConsecutiveSuccess),
    state:        Closed,
    openHook:     noopHook,
    halfOpenHook: noopHook,
    closeHook:    noopHook,
  }

  for _, apply := range opts {
    apply(c)
  }
  return c
}

func (c *CircuitBreaker) Do(op Op) error {
  if c.state == Closed {
    return c.doClose(op)
  }
  if c.state == Open {
    return c.doOpen(op)
  }
  return c.doHalfOpen(op)
}

func (c *CircuitBreaker) doOpen(_ Op) error {
  return ErrCircuitOpen
}

func (c *CircuitBreaker) doClose(op Op) (err error) {
  err = op()
  if err == nil {
    atomic.StoreUint32(&c.consecutiveFailures, 0)
    return
  }
  cf := atomic.AddUint32(&c.consecutiveFailures, 1)
  if cf == c.consecutiveFailuresThreshold {
    c.openCircuit(Closed)
  }
  return
}

func (c *CircuitBreaker) doHalfOpen(op Op) error {
  err, toOpen, toClose := c.halfOpenStrategy.Process(op)
  if toOpen {
    c.openCircuit(HalfOpen)
  } else if toClose {
    c.closeCircuit(HalfOpen)
  }
  return err
}

func (c *CircuitBreaker) openCircuit(from uint32) {
  if atomic.CompareAndSwapUint32(&c.state, from, Open) {
    // c.openCancelFunc()
    // c.openCtx, c.openCancelFunc = context.WithCancel(context.Background())
    go func() {
      openDelayDone := time.After(c.openDuration)
      <-openDelayDone
      c.halfOpenStrategy.Reset(0)
      c.halfOpenCircuit(Open)
    }()
    c.openHook()
  }
}

func (c *CircuitBreaker) halfOpenCircuit(from uint32) {
  if atomic.CompareAndSwapUint32(&c.state, from, HalfOpen) {
    c.halfOpenHook()
  }
}
func (c *CircuitBreaker) closeCircuit(from uint32) {
  if atomic.CompareAndSwapUint32(&c.state, from, Closed) {
    c.closeHook()
  }
}

func (c *CircuitBreaker) CurrentState() string {
  if c.state == Open {
    return "open"
  }
  if c.state == Closed {
    return "close"
  }
  return "halfopen"
}

func WithFailuresThreshold(threshold uint32) func(breaker *CircuitBreaker) {
  return func(breaker *CircuitBreaker) {
    breaker.consecutiveFailuresThreshold = threshold
  }
}

func WithOpenDuration(d time.Duration) func(breaker *CircuitBreaker) {
  return func(breaker *CircuitBreaker) {
    breaker.openDuration = d
  }
}

func WithTimerStrategy(interval time.Duration, consecutiveSuccess uint32) func(breaker *CircuitBreaker) {
  s := strategy.NewTimerStrategy(interval, consecutiveSuccess)
  return func(breaker *CircuitBreaker) {
    breaker.halfOpenStrategy = s
  }
}

func WithCustomStrategy(s strategy.Strategy) func(breaker *CircuitBreaker) {
  return func(breaker *CircuitBreaker) {
    breaker.halfOpenStrategy = s
  }
}
