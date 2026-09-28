package middleware

import (
	"net/http"
	"time"

	"github.com/thunderheadhq/thunderhead/internal/allowlist"
	"github.com/thunderheadhq/thunderhead/internal/analyzer"
	"github.com/thunderheadhq/thunderhead/internal/blocklist"
)

// Config holds middleware configuration.
type Config struct {
	// Score threshold to start tarpitting requests (default 40)
	TarpitThreshold float64
	// Score threshold to hard block requests (default 75)
	BlockThreshold float64
	// How long to delay tarpitted requests (default 5s)
	TarpitDelay time.Duration
	// IPs and user agents to always allow through
	Allowlist AllowlistConfig
	// IPs and CIDRs to always block
	Blocklist BlocklistConfig
	// Paths disallowed by robots.txt - violations add +30 to score
	DisallowedPaths []string
}

type AllowlistConfig struct {
	IPs        []string
	UserAgents []string
}

type BlocklistConfig struct {
	IPs   []string
	CIDRs []string
}

// Default returns a Config with sensible defaults.
func Default() Config {
	return Config{
		TarpitThreshold: 40,
		BlockThreshold:  75,
		TarpitDelay:     5 * time.Second,
		Allowlist: AllowlistConfig{
			UserAgents: []string{"Googlebot", "Bingbot", "archive.org_bot"},
		},
	}
}

// Thunderhead wraps an http.Handler with intent scoring middleware.
//
// Example:
//
//	mux := http.NewServeMux()
//	mux.HandleFunc("/", yourHandler)
//	protected := middleware.Thunderhead(mux, middleware.Default())
//	http.ListenAndServe(":8080", protected)
func Thunderhead(next http.Handler, cfg Config) http.Handler {
	if cfg.TarpitThreshold == 0 {
		cfg.TarpitThreshold = 40
	}
	if cfg.BlockThreshold == 0 {
		cfg.BlockThreshold = 75
	}
	if cfg.TarpitDelay == 0 {
		cfg.TarpitDelay = 5 * time.Second
	}

	az := analyzer.New(cfg.DisallowedPaths, nil)

	al := allowlist.New(allowlist.Config{
		IPs:        cfg.Allowlist.IPs,
		UserAgents: cfg.Allowlist.UserAgents,
	})

	bl := blocklist.New(blocklist.Config{
		IPs:   cfg.Blocklist.IPs,
		CIDRs: cfg.Blocklist.CIDRs,
	})

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := extractIP(r)

		// Allowlist check
		if al.IsAllowed(ip, r.Header.Get("User-Agent")) {
			next.ServeHTTP(w, r)
			return
		}

		// Blocklist check
		if bl.IsBlocked(ip) {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		// Score the request
		score := az.Score(r, ip)

		switch {
		case score >= cfg.BlockThreshold:
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		case score >= cfg.TarpitThreshold:
			time.Sleep(cfg.TarpitDelay)
		}

		next.ServeHTTP(w, r)
	})
}

func extractIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}
	ip := r.RemoteAddr
	for i := len(ip) - 1; i >= 0; i-- {
		if ip[i] == ':' {
			return ip[:i]
		}
	}
	return ip
}
