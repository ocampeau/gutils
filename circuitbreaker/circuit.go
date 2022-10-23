package circuitbreaker

import (
  "errors"
  "sync/atomic"
  "time"

  "github.com/ocampeau/gutils/circuitbreaker/strategy"
)

type OnStateChangeHook = func()
type Op = func() error
type Options func(breaker *CircuitBreaker)

var (
  ErrCircuitOpen     = errors.New("http circuit breaker is open")
  ErrCircuitInternal = errors.New("internal error with circuit breaker")
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
  name                         string
  openDuration                 time.Duration
  halfOpenStrategy             strategy.Strategy
  consecutiveFailures          uint32
  consecutiveFailuresThreshold uint32
  state                        uint32
  openHooks                    []OnStateChangeHook
  halfOpenHooks                []OnStateChangeHook
  closeHooks                   []OnStateChangeHook
}

func NewCircuitBreaker(name string, opts ...Options) *CircuitBreaker {
  c := &CircuitBreaker{
    name:                         name,
    openDuration:                 DefaultOpenTimerDuration,
    consecutiveFailuresThreshold: 5,
    halfOpenStrategy: strategy.NewTimerStrategy(
      DefaultHalfOpenTimerDuration,
      DefaultHalfOpenConsecutiveSuccess),
    state:         Closed,
    openHooks:     []OnStateChangeHook{},
    halfOpenHooks: []OnStateChangeHook{},
    closeHooks:    []OnStateChangeHook{},
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
    execHooks(c.openHooks)
  }
}

func (c *CircuitBreaker) halfOpenCircuit(from uint32) {
  if atomic.CompareAndSwapUint32(&c.state, from, HalfOpen) {
    execHooks(c.halfOpenHooks)
  }
}
func (c *CircuitBreaker) closeCircuit(from uint32) {
  if atomic.CompareAndSwapUint32(&c.state, from, Closed) {
    execHooks(c.closeHooks)
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

func (c *CircuitBreaker) RegisterOnOpenHooks(h OnStateChangeHook) {
  c.openHooks = append(c.openHooks, h)
}

func (c *CircuitBreaker) RegisterOnCloseHooks(h OnStateChangeHook) {
  c.closeHooks = append(c.closeHooks, h)
}

func (c *CircuitBreaker) RegisterOnHalfOpenHooks(h OnStateChangeHook) {
  c.halfOpenHooks = append(c.halfOpenHooks, h)
}

func execHooks(hooks []OnStateChangeHook) {
  if hooks == nil {
    return
  }
  for _, h := range hooks {
    h()
  }
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
