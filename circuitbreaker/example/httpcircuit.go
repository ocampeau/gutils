package example

import (
  "github.com/ocampeau/gutils/circuitbreaker"
  "net/http"
)

func createHttpCircuitBreaker(){
  // you can use any transport you want
  cb := circuitbreaker.NewHttpTransportCircuitBreaker(http.DefaultTransport)

  httpClient := http.Client{
    Transport:     cb,
  }
  httpClient.Do(&http.Request{})
}
