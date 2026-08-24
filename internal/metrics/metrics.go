package metrics

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	ActiveStreams = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "livestreaming_active_streams",
		Help: "Number of currently live channels",
	})
	Viewers = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "livestreaming_viewers",
		Help: "Concurrent viewers per channel",
	}, []string{"channel"})
	RequestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total HTTP requests",
	}, []string{"method", "path", "status"})
	RequestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request duration",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "path"})
)

func init() {
	prometheus.MustRegister(ActiveStreams, Viewers, RequestsTotal, RequestDuration)
}

// Middleware records http_requests_total and duration.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)
		duration := time.Since(start).Seconds()
		path := routePattern(r)
		RequestsTotal.WithLabelValues(r.Method, path, strconv.Itoa(rw.status)).Inc()
		RequestDuration.WithLabelValues(r.Method, path).Observe(duration)
	})
}

// routePattern returns the matched route pattern (e.g. "/api/channels/{username}")
// instead of the raw URL path, keeping label cardinality bounded.
func routePattern(r *http.Request) string {
	if rctx := chi.RouteContext(r.Context()); rctx != nil {
		if p := rctx.RoutePattern(); p != "" {
			return p
		}
	}
	return "unmatched"
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hj, ok := r.ResponseWriter.(http.Hijacker); ok {
		return hj.Hijack()
	}
	return nil, nil, fmt.Errorf("response does not implement http.Hijacker")
}

func (r *statusRecorder) Flush() {
	if fl, ok := r.ResponseWriter.(http.Flusher); ok {
		fl.Flush()
	}
}

// Handler returns the prometheus handler.
func Handler() http.Handler {
	return promhttp.Handler()
}

// SetActiveStreams updates the gauge.
func SetActiveStreams(n int) {
	ActiveStreams.Set(float64(n))
}

// SetViewers updates per-channel viewer gauge.
func SetViewers(channel string, n int) {
	Viewers.WithLabelValues(channel).Set(float64(n))
}

// ResetViewers clears all viewer gauges (useful for tests).
func ResetViewers() {
	Viewers.Reset()
}
