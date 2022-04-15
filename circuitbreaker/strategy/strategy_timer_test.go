package strategy

import (
  "context"
  "github.com/stretchr/testify/assert"
  "runtime"
  "sync"
  "testing"
  "time"
)

func TestHalfOpenTimerStrategyProcessNotExpired(t *testing.T) {
  testCases := []struct {
    description        string
    expireAt         int64
    testDuration time.Duration
    shouldErr bool
  }{
    {
      description: "when delay interval is not expired, it shouldn't open nor close the circuit",
      expireAt: time.Now().Add(24 * time.Hour).UnixMicro(),
      testDuration: 1 * time.Second,
      shouldErr: true,
    },
  }
  for _, tc := range testCases {
    t.Run(tc.description, func(t *testing.T) {
      s := &halfOpenTimer{
        expireAt:           tc.expireAt,
        l: &sync.Mutex{},
      }

      op := func() error {return nil}
      ctx, cancel := context.WithCancel(context.Background())
      go func(){
        <- time.After(tc.testDuration)
        cancel()
      }()

      wg := sync.WaitGroup{}
      for i := 0; i < runtime.NumCPU(); i++{
        wg.Add(1)
        go func(){
          defer wg.Done()
          for{
            if ctx.Err() != nil{
              break
            }
            err, doOpen, doClose := s.Process(op)
            assert.Equal(t, tc.shouldErr, err != nil)
            assert.False(t, doOpen)
            assert.False(t, doClose)
          }
        }()
      }
      wg.Wait()
    })
  }
}

func TestHalfOpenTimerStrategyProcessInFlight(t *testing.T) {
  testCases := []struct {
    description        string
    expireAt         int64
    testDuration time.Duration
    delayDuration time.Duration
    successThreshold uint32
    shouldErr bool
  }{
    {
      description: "when delay interval is expired, allow only one in-flight request per interval",
      expireAt: time.Now().UnixMicro(),
      testDuration: 1 * time.Second,
      delayDuration: 5 * time.Millisecond,
      successThreshold: 99999,
      shouldErr: true,
    },
    {
      description: "when delay interval is expired, allow only one in-flight request per interval",
      expireAt: time.Now().UnixMicro(),
      testDuration: 500 * time.Millisecond,
      delayDuration: 7 * time.Millisecond,
      successThreshold: 99999,
      shouldErr: true,
    },
  }
  for _, tc := range testCases {
    t.Run(tc.description, func(t *testing.T) {
      s := &halfOpenTimer{
        expireAt:           tc.expireAt,
        expireInterval:     tc.delayDuration,
        successThreshold:   tc.successThreshold,
        l: &sync.Mutex{},
      }

      l := sync.Mutex{}
      var numInFlightRequest uint32 = 0
      op := func() error {
        l.Lock()
        defer l.Unlock()
        numInFlightRequest += 1
        return nil
      }

      ctx, cancel := context.WithCancel(context.Background())
      go func(){
        <- time.After(tc.testDuration)
        cancel()
      }()

      start := time.Now()
      wg := sync.WaitGroup{}
      for i := 0; i < runtime.NumCPU(); i++{
        wg.Add(1)
        go func(){
          defer wg.Done()
          for{
            s.Process(op)
            if ctx.Err() != nil{
              break
            }
          }
        }()
      }


      wg.Wait()
      numIntervals := time.Since(start).Microseconds() / tc.delayDuration.Microseconds()

      // 1 for the immediate start + 1 in case the goroutine finished before the time calculation
      const errorMargin = 2
      assert.InDelta(t, numIntervals, numInFlightRequest, errorMargin)
    })

  }
}
