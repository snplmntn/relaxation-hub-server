package middleware

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type GlobalRateLimiter struct {
	visitors map[string]*visitor
	mu       sync.Mutex
	r        rate.Limit
	b        int
}

func NewGlobalRateLimiter(rps float64, burst int) *GlobalRateLimiter {
	rl := &GlobalRateLimiter{
		visitors: make(map[string]*visitor),
		r:        rate.Limit(rps),
		b:        burst,
	}
	go rl.cleanupVistors()
	return rl
}

func (rl *GlobalRateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. Exclude static assets (uploads)
		if strings.Contains(r.URL.Path, "/uploads/") {
			next.ServeHTTP(w, r)
			return
		}

		// 2. Identify client by IP
		ip := r.Header.Get("X-Forwarded-For")
		if ip == "" {
			ip, _, _ = net.SplitHostPort(r.RemoteAddr)
		}

		// 3. Check Limit
		limiter := rl.getLimiter(ip)
		if !limiter.Allow() {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error":       "Too many requests. Please try again in 5 seconds.",
				"retry_after": 5,
			})
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (rl *GlobalRateLimiter) getLimiter(ip string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	v, exists := rl.visitors[ip]
	if !exists {
		limiter := rate.NewLimiter(rl.r, rl.b)
		rl.visitors[ip] = &visitor{limiter, time.Now()}
		return limiter
	}
	v.lastSeen = time.Now()
	return v.limiter
}

func (rl *GlobalRateLimiter) cleanupVistors() {
	for {
		time.Sleep(1 * time.Minute)
		rl.mu.Lock()
		for ip, v := range rl.visitors {
			if time.Since(v.lastSeen) > 3*time.Minute {
				delete(rl.visitors, ip)
			}
		}
		rl.mu.Unlock()
	}
}
