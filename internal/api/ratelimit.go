package api

import (
	"math"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	maxTrackedIPs   = 10000
	staleIPDuration = 5 * time.Minute
	cleanupInterval = 2 * time.Minute
)

// RateLimitConfig configures per-IP token-bucket rate limiting.
// When RPS <= 0, rate limiting is disabled.
type RateLimitConfig struct {
	RPS   float64
	Burst int
}

type ipBucket struct {
	tokens   float64
	lastSeen time.Time
}

type ipRateLimiter struct {
	rps         float64
	burst       float64
	mu          sync.Mutex
	buckets     map[string]*ipBucket
	lastCleanup time.Time
}

func newIPRateLimiter(cfg RateLimitConfig) *ipRateLimiter {
	if cfg.RPS <= 0 {
		return nil
	}
	burst := cfg.Burst
	if burst < 1 {
		burst = int(math.Max(1, math.Ceil(cfg.RPS*2)))
	}
	return &ipRateLimiter{
		rps:         cfg.RPS,
		burst:       float64(burst),
		buckets:     make(map[string]*ipBucket, 256),
		lastCleanup: time.Now(),
	}
}

func (l *ipRateLimiter) allow(ip string, now time.Time) (allowed bool, remaining int, retryAfterSec int) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if now.Sub(l.lastCleanup) >= cleanupInterval || len(l.buckets) >= maxTrackedIPs {
		l.evictStaleLocked(now)
	}

	b, ok := l.buckets[ip]
	if !ok {
		b = &ipBucket{
			tokens:   l.burst - 1,
			lastSeen: now,
		}
		l.buckets[ip] = b
		return true, int(b.tokens), 0
	}

	elapsed := now.Sub(b.lastSeen).Seconds()
	if elapsed > 0 {
		b.tokens = math.Min(l.burst, b.tokens+elapsed*l.rps)
	}
	b.lastSeen = now

	if b.tokens >= 1.0 {
		b.tokens -= 1.0
		return true, int(b.tokens), 0
	}

	needed := 1.0 - b.tokens
	waitSec := int(math.Ceil(needed / l.rps))
	if waitSec < 1 {
		waitSec = 1
	}
	return false, 0, waitSec
}

func (l *ipRateLimiter) evictStaleLocked(now time.Time) {
	l.lastCleanup = now
	cutoff := now.Add(-staleIPDuration)
	for ip, b := range l.buckets {
		if b.lastSeen.Before(cutoff) {
			delete(l.buckets, ip)
		}
	}
	// Hard cap protection against spoofed/distributed IP floods
	if len(l.buckets) >= maxTrackedIPs {
		clear(l.buckets)
	}
}

// limit charges one request to the bucket `id`. It sets the rate-limit headers and, when the
// bucket is empty, writes the 429 response and returns false. A nil limiter allows everything.
func (l *ipRateLimiter) limit(w http.ResponseWriter, id string) bool {
	if l == nil {
		return true
	}
	ok, remaining, retryAfter := l.allow(id, time.Now())
	w.Header().Set("X-RateLimit-Limit", strconv.Itoa(int(l.burst)))
	w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
	if !ok {
		w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
		writeJSON(w, http.StatusTooManyRequests, map[string]any{
			"error":   "rate_limited",
			"message": "Příliš mnoho požadavků – zkuste to prosím za okamžik.",
		})
	}
	return ok
}

// rateLimited applies the per-IP limit to anonymous API calls and the higher per-key limit to
// calls with a paid API key.
func (s *Server) rateLimited(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Never throttle orchestrator health probes, non-API static assets or Stripe's webhook
		// (it authenticates by signature).
		if r.URL.Path == "/healthz" || !strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == webhookPath {
			next.ServeHTTP(w, r)
			return
		}

		if key := bearerKey(r); key != "" && s.billing.enabled() {
			if id, ok := s.billing.Keys.Lookup(r.Context(), key); ok {
				if s.paidLimiter.limit(w, "key:"+id) {
					next.ServeHTTP(w, r)
				}
				return
			}
			// A wrong or lapsed key still costs the caller's IP budget, so guessing keys is throttled.
			if s.limiter.limit(w, extractClientIP(r)) {
				writeJSON(w, http.StatusUnauthorized, map[string]any{
					"error":   "invalid_api_key",
					"message": "Neplatný nebo neaktivní API klíč. Bez hlavičky Authorization funguje bezplatné API.",
				})
			}
			return
		}

		if s.limiter.limit(w, extractClientIP(r)) {
			next.ServeHTTP(w, r)
		}
	})
}

// extractClientIP resolves the visitor's IP. The bundled nginx (web/nginx.conf) is the single place
// that decides who the visitor is (from Cloudflare's CF-Connecting-IP) and hands it over in
// X-Real-IP, overwriting whatever the client sent. So X-Real-IP is trusted only when the immediate
// peer is that internal proxy; every other forwarding header is client-controlled and ignored.
func extractClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err != nil {
		host = strings.TrimSpace(r.RemoteAddr)
	}

	if peerAddr, err := netip.ParseAddr(host); err == nil {
		if peerAddr.IsLoopback() || peerAddr.IsPrivate() || peerAddr.IsLinkLocalUnicast() {
			if ip, err := netip.ParseAddr(strings.TrimSpace(r.Header.Get("X-Real-IP"))); err == nil {
				return ip.String()
			}
		}
		return peerAddr.String()
	}

	if host != "" {
		return host
	}
	return "unknown"
}
