package strategy

import (
  "sync"
  "sync/atomic"
  "time"
)

type halfOpenTimer struct {
  inFlight           int32
  expireAt           int64
  expireInterval     time.Duration
  consecutiveSuccess uint32
  successThreshold   uint32
  l *sync.Mutex
}

func NewTimerStrategy(expireInterval time.Duration, threshold uint32) *halfOpenTimer {
  return &halfOpenTimer{
    inFlight:           0,
    expireAt:           0,
    successThreshold: threshold,
    l: &sync.Mutex{},
    expireInterval: expireInterval,
  }
}

func (s *halfOpenTimer) Reset(expire int64) {
  atomic.StoreInt64(&s.expireAt, expire)
  atomic.StoreUint32(&s.consecutiveSuccess, 0)
  atomic.StoreInt32(&s.inFlight, 0)
}

func (s *halfOpenTimer) Process(op func() error) (err error, toOpen bool, toClose bool) {
  s.l.Lock()
  defer s.l.Unlock()
  now := time.Now().UnixMicro()
  if s.expireAt > now{
    return s.stayHalfOpen(ErrHalfOpen)
  }

  // allow a single request at a time
  // if a request is already in flight, do nothing
  if !atomic.CompareAndSwapInt32(&s.inFlight, 0, 1) {
    return s.stayHalfOpen(ErrHalfOpen)
  }

  // do the operation
  err = op()

  // if the operation returns an error, open the circuit again
  // and reset the state to its initial value
  if err != nil {
    return s.openCircuit(err)
  }

  // reset the timer
  s.expireAt = time.Now().Add(s.expireInterval).UnixMicro()

  // set the inFlight flag to false in order to allow another request
  s.inFlight = 0

  // if the operation is a success, only close the circuit once we reached
  // the success threshold
  cs := atomic.AddUint32(&s.consecutiveSuccess, 1)
  if cs == s.successThreshold {
    return s.closeCircuit()
  } else {
    return s.stayHalfOpen(err)
  }
}

func (s *halfOpenTimer) openCircuit(err error) (error, bool, bool) {
  return err, true, false
}

func (s *halfOpenTimer) closeCircuit() (error, bool, bool) {
  return nil, false, true
}

func (s *halfOpenTimer) stayHalfOpen(err error) (error, bool, bool) {
  return err, false, false
}

