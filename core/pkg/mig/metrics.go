package mig

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type Metrics struct {
	requestTotal   *prometheus.CounterVec
	requestLatency *prometheus.HistogramVec
	requestErrors  *prometheus.CounterVec
	activeStreams  *prometheus.GaugeVec
}

func NewMetrics(registry *prometheus.Registry) *Metrics {
	factory := promauto.With(registry)
	return &Metrics{
		requestTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Namespace: "mig",
			Subsystem: "gateway",
			Name:      "http_requests_total",
			Help:      "Total number of HTTP requests processed by migd.",
		}, []string{"method", "path", "status"}),
		requestLatency: factory.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "mig",
			Subsystem: "gateway",
			Name:      "http_request_duration_seconds",
			Help:      "Request duration by route.",
			Buckets:   prometheus.DefBuckets,
		}, []string{"method", "path"}),
		requestErrors: factory.NewCounterVec(prometheus.CounterOpts{
			Namespace: "mig",
			Subsystem: "gateway",
			Name:      "errors_total",
			Help:      "MIG errors emitted by code.",
		}, []string{"code", "operation"}),
		activeStreams: factory.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "mig",
			Subsystem: "gateway",
			Name:      "active_streams",
			Help:      "Active stream count by type.",
		}, []string{"type"}),
	}
}

func (m *Metrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := normalizeMetricPath(r.URL.Path)
		method := r.Method
		start := time.Now()
		rw := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)
		status := strconv.Itoa(rw.status)
		m.requestTotal.WithLabelValues(method, path, status).Inc()
		m.requestLatency.WithLabelValues(method, path).Observe(time.Since(start).Seconds())
	})
}

func (m *Metrics) RecordError(code, operation string) {
	if code == "" {
		code = ErrorInternal
	}
	if operation == "" {
		operation = "unknown"
	}
	m.requestErrors.WithLabelValues(code, operation).Inc()
}

func (m *Metrics) IncActiveStream(streamType string) {
	m.activeStreams.WithLabelValues(streamType).Inc()
}

func (m *Metrics) DecActiveStream(streamType string) {
	m.activeStreams.WithLabelValues(streamType).Dec()
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (rw *statusRecorder) WriteHeader(status int) {
	rw.status = status
	rw.ResponseWriter.WriteHeader(status)
}

func (rw *statusRecorder) Flush() {
	if flusher, ok := rw.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (rw *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := rw.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("hijacker not supported")
	}
	return hijacker.Hijack()
}

func (rw *statusRecorder) Push(target string, opts *http.PushOptions) error {
	pusher, ok := rw.ResponseWriter.(http.Pusher)
	if !ok {
		return http.ErrNotSupported
	}
	return pusher.Push(target, opts)
}

func normalizeMetricPath(path string) string {
	if strings.HasPrefix(path, "/mig/v0.1/invoke/") {
		return "/mig/v0.1/invoke/{capability}"
	}
	if strings.HasPrefix(path, "/mig/v0.1/publish/") {
		return "/mig/v0.1/publish/{topic}"
	}
	if strings.HasPrefix(path, "/mig/v0.1/subscribe/") {
		return "/mig/v0.1/subscribe/{topic}"
	}
	if strings.HasPrefix(path, "/mig/v0.1/cancel/") {
		return "/mig/v0.1/cancel/{message_id}"
	}
	return path
}
