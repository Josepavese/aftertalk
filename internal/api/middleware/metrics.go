package middleware

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/Josepavese/aftertalk/internal/metrics"
)

func PrometheusMetrics() http.Handler {
	return promhttp.Handler()
}

func MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)

		duration := time.Since(start).Seconds()
		status := strconv.Itoa(ww.Status())

		metrics.RequestDuration.WithLabelValues(
			r.Method,
			r.URL.Path,
			status,
		).Observe(duration)
	})
}

func RateLimit(requestsPerMinute int) func(http.Handler) http.Handler {
	type client struct {
		lastCheck time.Time
		count     int
	}

	var mu sync.Mutex
	clients := make(map[string]*client)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := r.RemoteAddr

			mu.Lock()
			c, exists := clients[ip]
			if !exists {
				clients[ip] = &client{
					count:     1,
					lastCheck: time.Now(),
				}
			} else {
				if time.Since(c.lastCheck) > time.Minute {
					c.count = 1
					c.lastCheck = time.Now()
				} else {
					c.count++
					if c.count > requestsPerMinute {
						mu.Unlock()
						http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
						return
					}
				}
			}
			mu.Unlock()

			next.ServeHTTP(w, r)
		})
	}
}
