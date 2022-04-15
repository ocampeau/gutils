package strategy

import "errors"

var ErrHalfOpen = errors.New("circuit breaker is half open")

type Strategy interface {
  Reset(int64)
  Process(func() error ) (err error, toOpen bool, toClose bool)
}

