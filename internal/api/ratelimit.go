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

func (l *ipRateLimiter) wrap(next http.Handler) http.Handler {
	if l == nil {
		return next
	}
	limitHeader := strconv.Itoa(int(l.burst))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Never throttle orchestrator health probes or non-API static assets
		if r.URL.Path == "/healthz" || !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}

		ip := extractClientIP(r)
		ok, remaining, retryAfter := l.allow(ip, time.Now())
		w.Header().Set("X-RateLimit-Limit", limitHeader)
		w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))

		if !ok {
			w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
			writeJSON(w, http.StatusTooManyRequests, map[string]any{
				"error":   "rate_limited",
				"message": "Příliš mnoho požadavků – zkuste to prosím za okamžik.",
			})
			return
		}

		next.ServeHTTP(w, r)
	})
}

// extractClientIP safely resolves the client IP, trusting proxy headers ONLY when
// the immediate peer (RemoteAddr) is a loopback or private/internal proxy network.
func extractClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err != nil {
		host = strings.TrimSpace(r.RemoteAddr)
	}

	if peerAddr, err := netip.ParseAddr(host); err == nil {
		if peerAddr.IsLoopback() || peerAddr.IsPrivate() || peerAddr.IsLinkLocalUnicast() {
			if cf := strings.TrimSpace(r.Header.Get("CF-Connecting-IP")); cf != "" {
				if ip, err := netip.ParseAddr(cf); err == nil {
					return ip.String()
				}
			}
			if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
				for _, part := range strings.Split(xff, ",") {
					cand := strings.TrimSpace(part)
					if ip, err := netip.ParseAddr(cand); err == nil {
						return ip.String()
					}
				}
			}
			if xri := strings.TrimSpace(r.Header.Get("X-Real-IP")); xri != "" {
				if ip, err := netip.ParseAddr(xri); err == nil {
					return ip.String()
				}
			}
		}
		return peerAddr.String()
	}

	if host != "" {
		return host
	}
	return "unknown"
}
