package example

import (
  "net/http"

  "github.com/ocampeau/gutils/circuitbreaker"
)

func createHttpCircuitBreaker() {
  // you can use any transport you want
  cb := circuitbreaker.NewHttpTransportCircuitBreaker("HttpCircuitBreaker", http.DefaultTransport)

  httpClient := http.Client{
    Transport: cb,
  }
  httpClient.Do(&http.Request{})
}

// in order to satisfy static checker
func init() {
  createHttpCircuitBreaker()
}
