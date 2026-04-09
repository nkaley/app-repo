package metrics

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Metrics struct {
	mu            sync.RWMutex
	requestsTotal map[string]uint64
	requestErrors uint64
	latencySum    map[string]float64
	latencyCount  map[string]uint64
	latencyBucket map[string]map[string]uint64
	dbErrors      uint64
	redisErrors   uint64
	cacheHits     uint64
	cacheMisses   uint64
}

func New() *Metrics {
	return &Metrics{
		requestsTotal: make(map[string]uint64),
		latencySum:    make(map[string]float64),
		latencyCount:  make(map[string]uint64),
		latencyBucket: make(map[string]map[string]uint64),
	}
}

func (m *Metrics) Instrument(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)

		path := r.URL.Path
		method := r.Method
		status := strconv.Itoa(rec.status)
		key := method + "|" + path + "|" + status
		latencyKey := method + "|" + path
		latencySeconds := time.Since(started).Seconds()

		m.mu.Lock()
		m.requestsTotal[key]++
		if rec.status >= http.StatusInternalServerError {
			m.requestErrors++
		}
		m.latencySum[latencyKey] += latencySeconds
		m.latencyCount[latencyKey]++
		m.observeLatencyBucket(latencyKey, latencySeconds)
		m.mu.Unlock()
	})
}

func (m *Metrics) IncDBErrors() {
	m.mu.Lock()
	m.dbErrors++
	m.mu.Unlock()
}

func (m *Metrics) IncRedisErrors() {
	m.mu.Lock()
	m.redisErrors++
	m.mu.Unlock()
}

func (m *Metrics) IncCacheHit() {
	m.mu.Lock()
	m.cacheHits++
	m.mu.Unlock()
}

func (m *Metrics) IncCacheMiss() {
	m.mu.Lock()
	m.cacheMisses++
	m.mu.Unlock()
}

func (m *Metrics) Handler(w http.ResponseWriter, _ *http.Request) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	w.Header().Set("Content-Type", "text/plain; version=0.0.4")

	var b strings.Builder
	b.WriteString("# HELP http_requests_total Total HTTP requests.\n")
	b.WriteString("# TYPE http_requests_total counter\n")
	for key, value := range m.requestsTotal {
		parts := strings.Split(key, "|")
		if len(parts) != 3 {
			continue
		}
		fmt.Fprintf(&b, "http_requests_total{method=%q,path=%q,status=%q} %d\n", parts[0], parts[1], parts[2], value)
	}

	b.WriteString("# HELP http_request_latency_seconds HTTP request latency histogram.\n")
	b.WriteString("# TYPE http_request_latency_seconds histogram\n")
	for key, bucketMap := range m.latencyBucket {
		parts := strings.Split(key, "|")
		if len(parts) != 2 {
			continue
		}
		for _, le := range []string{"0.01", "0.05", "0.1", "0.25", "0.5", "1", "2.5", "5", "+Inf"} {
			fmt.Fprintf(&b, "http_request_latency_seconds_bucket{method=%q,path=%q,le=%q} %d\n", parts[0], parts[1], le, bucketMap[le])
		}
		fmt.Fprintf(&b, "http_request_latency_seconds_sum{method=%q,path=%q} %f\n", parts[0], parts[1], m.latencySum[key])
		fmt.Fprintf(&b, "http_request_latency_seconds_count{method=%q,path=%q} %d\n", parts[0], parts[1], m.latencyCount[key])
	}

	fmt.Fprintf(&b, "# HELP http_request_errors_total HTTP 5xx responses.\n# TYPE http_request_errors_total counter\nhttp_request_errors_total %d\n", m.requestErrors)
	fmt.Fprintf(&b, "# HELP db_errors_total Database errors.\n# TYPE db_errors_total counter\ndb_errors_total %d\n", m.dbErrors)
	fmt.Fprintf(&b, "# HELP redis_errors_total Redis errors.\n# TYPE redis_errors_total counter\nredis_errors_total %d\n", m.redisErrors)
	fmt.Fprintf(&b, "# HELP cache_hits_total Cache hits.\n# TYPE cache_hits_total counter\ncache_hits_total %d\n", m.cacheHits)
	fmt.Fprintf(&b, "# HELP cache_misses_total Cache misses.\n# TYPE cache_misses_total counter\ncache_misses_total %d\n", m.cacheMisses)

	_, _ = w.Write([]byte(b.String()))
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(statusCode int) {
	r.status = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

func (m *Metrics) observeLatencyBucket(key string, latencySeconds float64) {
	if _, ok := m.latencyBucket[key]; !ok {
		m.latencyBucket[key] = make(map[string]uint64)
	}

	bounds := []struct {
		le    string
		value float64
	}{
		{"0.01", 0.01},
		{"0.05", 0.05},
		{"0.1", 0.1},
		{"0.25", 0.25},
		{"0.5", 0.5},
		{"1", 1},
		{"2.5", 2.5},
		{"5", 5},
	}

	for _, b := range bounds {
		if latencySeconds <= b.value {
			m.latencyBucket[key][b.le]++
		}
	}
	m.latencyBucket[key]["+Inf"]++
}
