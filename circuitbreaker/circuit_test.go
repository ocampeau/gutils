package circuitbreaker

import (
  "context"
  "errors"
  "fmt"
  "github.com/golang/mock/gomock"
  "github.com/ocampeau/gutils/circuitbreaker/strategy"
  "github.com/ocampeau/gutils/circuitbreaker/strategy/mocks"
  "github.com/stretchr/testify/assert"
  "runtime"
  "sync"
  "sync/atomic"
  "testing"
  "time"
)

func TestCircuitShouldOpenAfterNConsecutiveFailuresConcurrent(t *testing.T) {
  testCasesThreshold := []uint32{
    1, 100, 125, 250, 401, 502, 678, 1001, 109745,
  }

  stop := make(chan interface{}, 1)
  defer close(stop)

  for _, failureThreshold := range testCasesThreshold {
    _, cancel := context.WithCancel(context.Background())

    go func() {
      <-stop
      cancel()
    }()

    op := func() error {return ErrCircuitInternal}
    cb := NewCircuitBreaker(WithTimerStrategy(1 * time.Second, 5))
    cb.consecutiveFailuresThreshold = failureThreshold

    var cf, stateChangeTime uint32 = 0, 0
    cb.openHook = func() {
      cf = cb.consecutiveFailures
      atomic.AddUint32(&stateChangeTime, 1)
    }

    wg := sync.WaitGroup{}
    var numReq uint32 = 0
    for {
      if atomic.LoadUint32(&numReq) >= failureThreshold {
        break
      }
      wg.Add(1)
      go func() {
        defer wg.Done()
        cb.Do(op)
        atomic.AddUint32(&numReq, 1)
      }()
    }

    wg.Wait()

    // because it is hard to get the exact value in concurrent environment we expect the
    // number of concurrent failure to be at least equal to the `consecutiveFailuresThreshold`
    // and at most equal to `consecutiveFailuresThreshold` + `runtime.NumCPU` to allow for the
    // other concurrent in-flight requests to finish
    assert.GreaterOrEqual(t, cf, failureThreshold)
    assert.LessOrEqual(t, cf, failureThreshold + uint32(runtime.NumCPU()))

    // assert the openHook is only called once
    assert.Equal(t, uint32(1), stateChangeTime)
  }

}

func TestCircuitShouldHalfOpenOnceTimerExpire(t *testing.T) {
  durations := []time.Duration{
    1 * time.Millisecond,
    10 * time.Millisecond,
    50 * time.Millisecond,
    500 * time.Millisecond,
    1 * time.Second,
  }

  for _, testDuration := range durations{
    t.Run(fmt.Sprintf("with duration %s", testDuration.String()), func(t *testing.T){
      cb := NewCircuitBreaker(WithOpenDuration(testDuration))
      cb.state = Closed
      cb.openCircuit(Closed)

      // we add 5 millisecond to the timer to allow time for the synchronization of the state
      // between the goroutines. That is to say that the state transition from open to half-open
      // is precise at 5 milliseconds
      <- time.After(testDuration + (10 * time.Millisecond))
      assert.Equalf(t, HalfOpen, int(cb.state), "expected half-open but got %s", cb.CurrentState())
    })
  }

}

func TestCircuitShouldOpenWhenHalfOpenReturnsTrue(t *testing.T) {
    ctrl := gomock.NewController(t)
    s := mocks.NewMockStrategy(ctrl)

    s.EXPECT().Process(gomock.Any()).Times(1).Return(strategy.ErrHalfOpen, true, false)
    s.EXPECT().Reset(gomock.Any()).AnyTimes()
    cb := NewCircuitBreaker(WithCustomStrategy(s))
    cb.state = HalfOpen
    err := cb.doHalfOpen(func()error{return ErrCircuitInternal})
    assert.Equal(t, strategy.ErrHalfOpen, err)
    assert.Equal(t, Open, int(cb.state))
}

func TestCircuitShouldCloseWhenHalfOpenReturnsTrue(t *testing.T) {
  ctrl := gomock.NewController(t)
  s := mocks.NewMockStrategy(ctrl)

  s.EXPECT().Process(gomock.Any()).Times(1).Return(nil, false, true)

  cb := NewCircuitBreaker(WithCustomStrategy(s))
  cb.state = HalfOpen
  err := cb.doHalfOpen(func()error{return ErrCircuitInternal})
  assert.Equal(t, nil, err)
  assert.Equal(t, Closed, int(cb.state))
}

func TestCircuitShouldReturnErrWhenHalfOpenReturnsErr(t *testing.T) {
  ctrl := gomock.NewController(t)
  s := mocks.NewMockStrategy(ctrl)

  s.EXPECT().Process(gomock.Any()).Times(1).Return(ErrCircuitInternal, false, false)

  cb := NewCircuitBreaker(WithCustomStrategy(s))
  cb.state = HalfOpen
  err := cb.doHalfOpen(func()error{return ErrCircuitInternal})
  assert.NotNil(t, err)
  assert.Equal(t, HalfOpen, int(cb.state))
}

func TestCircuitE2E(t *testing.T){
  start := time.Now()

  numTimeToOpen := 0
  numTimeToHalfOpen := 0
  numTimeToClose := 0

  op := func() error {
    now := time.Now().UnixMicro()
    // return no error for the first 2 seconds
    if now <= start.Add(2 * time.Second).UnixMicro(){
      return nil
    }

    // return only errors for the next 2 seconds
    if now >= start.Add(2 * time.Second).UnixMicro() && now < start.Add(4 * time.Second).UnixMicro(){
      return errors.New("operation error")
    }

    // after 4 seconds, return no error
    return nil
  }

  cb := NewCircuitBreaker(WithTimerStrategy(50 * time.Millisecond, 5))
  cb.openDuration = 100 * time.Millisecond
  cb.openHook = func(){
    numTimeToOpen++
  }
  cb.closeHook = func(){
    numTimeToClose++
  }
  cb.halfOpenHook = func(){
    numTimeToHalfOpen++
  }

  endTestTime := time.Now().Add(5 * time.Second)
  for{
    if time.Now().UnixMicro() > endTestTime.UnixMicro(){
      break
    }
    cb.Do(op)
  }

  assert.Equal(t, numTimeToHalfOpen, numTimeToOpen)
  assert.Equal(t, numTimeToClose, 1)
}

func BenchmarkDoOpen(b *testing.B) {
  cb := NewCircuitBreaker()
  cb.state = Open

  op := func() error {return nil}

  b.ReportAllocs()
  b.RunParallel(func(pb *testing.PB) {
    for pb.Next() {
      cb.Do(op)
    }
  })

}

func BenchmarkDoClose(b *testing.B) {
  benchCases := []struct{
    description string
    opErr error
  }{
    {
      description: "operation always return nil",
      opErr: nil,
    },
    {
      description: "operation always return error",
      opErr: ErrCircuitInternal,
    },
  }

  for _, bc := range benchCases{
    b.Run(bc.description, func(b *testing.B){
      cb := NewCircuitBreaker()
      cb.state = Closed
      cb.consecutiveFailuresThreshold = 0

      op := func() error {return bc.opErr}

      b.ReportAllocs()
      b.RunParallel(func(pb *testing.PB) {
        for pb.Next() {
          cb.Do(op)
        }
      })
    })
  }

}

func BenchmarkDoHalfOpen(b *testing.B) {
  cb := NewCircuitBreaker()
  cb.state = HalfOpen

  op := func() error {return ErrCircuitInternal}
  b.ReportAllocs()
  b.RunParallel(func(pb *testing.PB) {
    for pb.Next() {
      cb.Do(op)
    }
  })
}

func BenchmarkOpenCircuit(b *testing.B) {
  cb := NewCircuitBreaker()
  for i := 0; i < b.N; i++{
    cb.state = Closed
    cb.openCircuit(Closed)
  }
}


