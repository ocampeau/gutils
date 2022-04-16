package circuitbreaker

import (
  "net/http"
)

type HttpTransport struct {
  next http.RoundTripper
  cb   *CircuitBreaker
}


func NewHttpTransportCircuitBreaker(rt http.RoundTripper, opts ... Options) *HttpTransport {
  tr := HttpTransport{
    next: rt,
    cb:   NewCircuitBreaker("test", opts...),
  }
  return &tr
}

func (t *HttpTransport) RoundTrip(req *http.Request) (res *http.Response, err error) {
  op := func() error {
    res, err = t.next.RoundTrip(req)
    return err
  }
  err = t.cb.Do(op)
  return
}
