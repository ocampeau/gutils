package circuitbreaker

import (
  "net/http"
)

type HttpTransport struct {
  next http.RoundTripper
  Circuit   *CircuitBreaker
}


func NewHttpTransportCircuitBreaker(name string, rt http.RoundTripper, opts ... Options) *HttpTransport {
  tr := HttpTransport{
    next: rt,
    Circuit:   NewCircuitBreaker(name, opts...),
  }
  return &tr
}

func (t *HttpTransport) RoundTrip(req *http.Request) (res *http.Response, err error) {
  op := func() error {
    res, err = t.next.RoundTrip(req)
    return err
  }
  err = t.Circuit.Do(op)
  return
}
