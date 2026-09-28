package analyzer

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/thunderheadhq/thunderhead/internal/store"
)

// Signal weights - all add up to produce a score 0–100
const (
	WeightRobotsViolation  = 30.0
	WeightSequentialCrawl  = 25.0
	WeightHighRate         = 20.0
	WeightSuspiciousHeader = 15.0
	WeightTextOnlyPattern  = 10.0
)

type requestRecord struct {
	path      string
	timestamp time.Time
}

type clientState struct {
	mu             sync.Mutex
	requests       []requestRecord
	robotsViolated bool
	lastScore      float64
	peakScore      float64
	lastSeen       time.Time
}

// Analyzer tracks per-IP state and scores intent
type Analyzer struct {
	mu      sync.RWMutex
	clients map[string]*clientState
	robots  map[string]struct{}
}

func New(disallowedPaths []string, initial map[string]store.ClientRecord) *Analyzer {
	robots := make(map[string]struct{}, len(disallowedPaths))
	for _, p := range disallowedPaths {
		robots[p] = struct{}{}
	}
	a := &Analyzer{
		clients: make(map[string]*clientState),
		robots:  robots,
	}
	for ip, rec := range initial {
		a.clients[ip] = &clientState{
			robotsViolated: rec.RobotsViolated,
			lastScore:      rec.LastScore,
			peakScore:      rec.PeakScore,
			lastSeen:       rec.LastSeen,
		}
	}
	return a
}

func (a *Analyzer) getClient(ip string) *clientState {
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, ok := a.clients[ip]; !ok {
		a.clients[ip] = &clientState{}
	}
	return a.clients[ip]
}

// Score returns an intent score 0–100 for the given request.
// Higher = more likely to be a bot.
func (a *Analyzer) Score(r *http.Request, ip string) float64 {
	client := a.getClient(ip)
	client.mu.Lock()
	defer client.mu.Unlock()

	now := time.Now()

	// Record this request
	client.requests = append(client.requests, requestRecord{
		path:      r.URL.Path,
		timestamp: now,
	})

	// Prune records older than 60s
	cutoff := now.Add(-60 * time.Second)
	pruned := client.requests[:0]
	for _, rec := range client.requests {
		if rec.timestamp.After(cutoff) {
			pruned = append(pruned, rec)
		}
	}
	client.requests = pruned

	var score float64

	// 1. robots.txt violation
	if _, disallowed := a.robots[r.URL.Path]; disallowed {
		client.robotsViolated = true
	}
	if client.robotsViolated {
		score += WeightRobotsViolation
	}

	// 2. Sequential crawling
	if isSequentialCrawl(client.requests) {
		score += WeightSequentialCrawl
	}

	// 3. High request rate - more than 30 requests in 60s
	if len(client.requests) > 30 {
		rate := float64(len(client.requests)) / 60.0
		score += WeightHighRate * min(rate/2.0, 1.0)
	}

	// 4. Suspicious headers
	score += WeightSuspiciousHeader * headerSuspicion(r)

	// 5. Text-only page pattern
	if isTextHeavyPattern(client.requests) {
		score += WeightTextOnlyPattern
	}

	if score > 100 {
		score = 100
	}
	client.lastScore = score
	if score > client.peakScore {
		client.peakScore = score
	}
	client.lastSeen = time.Now()
	return score
}

// isSequentialCrawl checks if recent paths look like a sequential crawl
func isSequentialCrawl(records []requestRecord) bool {
	if len(records) < 5 {
		return false
	}
	recent := records
	if len(recent) > 10 {
		recent = recent[len(recent)-10:]
	}
	ordered := 0
	for i := 1; i < len(recent); i++ {
		if recent[i].path > recent[i-1].path {
			ordered++
		}
	}
	return float64(ordered)/float64(len(recent)-1) >= 0.8
}

// headerSuspicion returns 0–1 based on how suspicious the headers look
func headerSuspicion(r *http.Request) float64 {
	suspicion := 0.0

	ua := r.Header.Get("User-Agent")
	if ua == "" {
		suspicion += 0.5
	} else {
		uaLower := strings.ToLower(ua)
		botKeywords := []string{"bot", "crawler", "spider", "scraper", "python", "curl", "wget", "httpclient", "go-http"}
		for _, kw := range botKeywords {
			if strings.Contains(uaLower, kw) {
				suspicion += 0.4
				break
			}
		}
	}

	if r.Header.Get("Accept") == "" {
		suspicion += 0.3
	}

	if r.Header.Get("Accept-Language") == "" {
		suspicion += 0.2
	}

	if suspicion > 1.0 {
		suspicion = 1.0
	}
	return suspicion
}

// isTextHeavyPattern checks if the client is hitting mostly text/content pages
func isTextHeavyPattern(records []requestRecord) bool {
	if len(records) < 5 {
		return false
	}
	textExtensions := []string{".html", ".htm", ".md", "/"}
	textHits := 0
	for _, rec := range records {
		for _, ext := range textExtensions {
			if strings.HasSuffix(rec.path, ext) || rec.path == "/" {
				textHits++
				break
			}
		}
	}
	return float64(textHits)/float64(len(records)) >= 0.8
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

type ClientStatus struct {
	Score          float64 `json:"score"`
	PeakScore      float64 `json:"peak_score"`
	RequestCount   int     `json:"request_count"`
	RobotsViolated bool    `json:"robots_violated"`
}

func (a *Analyzer) Status() map[string]ClientStatus {
	a.mu.RLock()
	defer a.mu.RUnlock()

	out := make(map[string]ClientStatus, len(a.clients))
	for ip, client := range a.clients {
		client.mu.Lock()
		out[ip] = ClientStatus{
			Score:          client.lastScore,
			PeakScore:      client.peakScore,
			RequestCount:   len(client.requests),
			RobotsViolated: client.robotsViolated,
		}
		client.mu.Unlock()
	}
	return out
}

func (a *Analyzer) Snapshot() map[string]store.ClientRecord {
	a.mu.RLock()
	defer a.mu.RUnlock()

	out := make(map[string]store.ClientRecord, len(a.clients))
	for ip, client := range a.clients {
		client.mu.Lock()
		out[ip] = store.ClientRecord{
			LastScore:      client.lastScore,
			PeakScore:      client.peakScore,
			RobotsViolated: client.robotsViolated,
			LastSeen:       client.lastSeen,
		}
		client.mu.Unlock()
	}
	return out
}

func (a *Analyzer) PruneExpired(days int) int {
	if days <= 0 {
		return 0
	}

	cutoff := time.Now().AddDate(0, 0, -days)
	a.mu.Lock()
	defer a.mu.Unlock()

	pruned := 0
	for ip, client := range a.clients {
		client.mu.Lock()
		lastSeen := client.lastSeen
		client.mu.Unlock()

		if !lastSeen.IsZero() && lastSeen.Before(cutoff) {
			delete(a.clients, ip)
			pruned++
		}
	}
	return pruned
}
