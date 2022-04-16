package circuitbreaker

import (
  "github.com/prometheus/client_golang/prometheus"
  "sync/atomic"
)

const (
  LabelsCircuitBreakerName = "circuit_breaker_name"
)

type PromCollector struct {
  cb                    *CircuitBreaker
  didOpen               uint64
  didClose              uint64
  didHalfOpen           uint64
  descCbOpenCounter     *prometheus.Desc
  descCbHalfOpenCounter *prometheus.Desc
  descCbCloseCounter    *prometheus.Desc
  descCbState           *prometheus.Desc
}

func NewPromCollector(cb *CircuitBreaker) prometheus.Collector {
  col := &PromCollector{
    descCbOpenCounter: prometheus.NewDesc("circuit_breaker_open_state",
      "A counter indicating the number of times the circuit has been in the open state",
      nil, prometheus.Labels{LabelsCircuitBreakerName: cb.name}),
    descCbCloseCounter: prometheus.NewDesc("circuit_breaker_close_state",
      "A counter indicating the number of times the circuit has been in the close state",
      nil, prometheus.Labels{LabelsCircuitBreakerName: cb.name}),
    descCbHalfOpenCounter: prometheus.NewDesc("circuit_breaker_halfopen_state",
      "A counter indicating the number of times the circuit has been in the half-open state",
      nil, prometheus.Labels{LabelsCircuitBreakerName: cb.name}),
    descCbState: prometheus.NewDesc("circuit_breaker_current_state",
      "A gauge that indicates the current state of the circuit",
      nil, prometheus.Labels{LabelsCircuitBreakerName: cb.name}),
  }

  cb.RegisterOnHalfOpenHooks(col.circuitBreakerOpen)
  cb.RegisterOnCloseHooks(col.circuitBreakerClose)
  cb.RegisterOnOpenHooks(col.circuitBreakerOpen)

  return &PromCollector{cb: cb}
}

func (col *PromCollector) circuitBreakerOpen() {
  atomic.AddUint64(&col.didOpen, 1)
}

func (col *PromCollector) circuitBreakerClose() {
  atomic.AddUint64(&col.didClose, 1)
}

func (col *PromCollector) circuitBreakerHalfOpen() {
  atomic.AddUint64(&col.didHalfOpen, 1)
}

func (col *PromCollector) Describe(ch chan<- *prometheus.Desc) {
  ch <- col.descCbCloseCounter
  ch <- col.descCbOpenCounter
  ch <- col.descCbHalfOpenCounter
  ch <- col.descCbState
}

func (col *PromCollector) Collect(ch chan<- prometheus.Metric) {
  ch <- prometheus.MustNewConstMetric(col.descCbCloseCounter, prometheus.CounterValue, float64(col.didClose))
  ch <- prometheus.MustNewConstMetric(col.descCbOpenCounter, prometheus.CounterValue, float64(col.didOpen))
  ch <- prometheus.MustNewConstMetric(col.descCbHalfOpenCounter, prometheus.CounterValue, float64(col.didHalfOpen))
  ch <- prometheus.MustNewConstMetric(col.descCbState, prometheus.GaugeValue, float64(col.cb.state))
}
