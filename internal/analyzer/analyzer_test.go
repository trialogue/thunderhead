package analyzer

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/trialogue/thunderhead/internal/store"
)

// ─── Helpers ──────────────────────────────────────────────────────────────────

func newReq(path string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, path, nil)
	r.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")
	r.Header.Set("Accept", "text/html,application/xhtml+xml")
	r.Header.Set("Accept-Language", "en-US,en;q=0.9")
	return r
}

func newBotReq(path string) *http.Request {
	return httptest.NewRequest(http.MethodGet, path, nil)
	// no User-Agent, Accept, or Accept-Language - maximally suspicious
}

func newAnalyzer(disallowed []string) *Analyzer {
	return New(disallowed, map[string]store.ClientRecord{})
}

// ─── Score: clean request ─────────────────────────────────────────────────────

func TestScore_CleanRequest_LowScore(t *testing.T) {
	az := newAnalyzer(nil)
	r := newReq("/about")
	score := az.Score(r, "1.2.3.4")
	if score >= 40 {
		t.Errorf("expected clean request score < 40, got %.1f", score)
	}
}

// ─── Score: robots.txt violation ─────────────────────────────────────────────

func TestScore_RobotsViolation_AddsWeight(t *testing.T) {
	az := newAnalyzer([]string{"/admin", "/private"})

	r := newReq("/admin")
	score := az.Score(r, "1.2.3.4")
	if score < WeightRobotsViolation {
		t.Errorf("expected score >= %.0f after robots violation, got %.1f", WeightRobotsViolation, score)
	}
}

func TestScore_RobotsViolation_Persists(t *testing.T) {
	az := newAnalyzer([]string{"/private"})

	az.Score(newReq("/private"), "1.2.3.4")

	// subsequent clean requests should still carry the violation weight
	score := az.Score(newReq("/about"), "1.2.3.4")
	if score < WeightRobotsViolation {
		t.Errorf("expected robots violation to persist, got %.1f", score)
	}
}

func TestScore_NoRobotsViolation_WhenPathAllowed(t *testing.T) {
	az := newAnalyzer([]string{"/private"})

	r := newReq("/about")
	score := az.Score(r, "1.2.3.4")
	if score >= WeightRobotsViolation {
		t.Errorf("expected no robots penalty for allowed path, got %.1f", score)
	}
}

// ─── Score: cap at 100 ───────────────────────────────────────────────────────

func TestScore_CapsAt100(t *testing.T) {
	az := newAnalyzer([]string{"/admin"})

	// hit robots violation + send with bot headers to stack signals
	for i := 0; i < 50; i++ {
		r := newBotReq("/admin")
		score := az.Score(r, "1.2.3.4")
		if score > 100 {
			t.Errorf("score exceeded 100: got %.1f on request %d", score, i)
		}
	}
}

// ─── Score: suspicious headers ───────────────────────────────────────────────

func TestHeaderSuspicion_MissingAll_MaxSuspicion(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	// no headers set at all
	s := headerSuspicion(r)
	if s < 0.9 {
		t.Errorf("expected suspicion near 1.0 for missing all headers, got %.2f", s)
	}
}

func TestHeaderSuspicion_BotUserAgent(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("User-Agent", "python-requests/2.28.0")
	r.Header.Set("Accept", "text/html")
	r.Header.Set("Accept-Language", "en")
	s := headerSuspicion(r)
	if s < 0.4 {
		t.Errorf("expected elevated suspicion for bot UA, got %.2f", s)
	}
}

func TestHeaderSuspicion_CleanBrowser_LowSuspicion(t *testing.T) {
	r := newReq("/")
	s := headerSuspicion(r)
	if s > 0.1 {
		t.Errorf("expected low suspicion for clean browser headers, got %.2f", s)
	}
}

func TestHeaderSuspicion_CapsAt1(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	// no headers - would normally exceed 1.0 if uncapped (0.5 + 0.3 + 0.2 = 1.0)
	s := headerSuspicion(r)
	if s > 1.0 {
		t.Errorf("headerSuspicion exceeded 1.0: got %.2f", s)
	}
}

// ─── Sequential crawl ────────────────────────────────────────────────────────

func TestIsSequentialCrawl_FewRequests_False(t *testing.T) {
	records := []requestRecord{
		{path: "/a"}, {path: "/b"}, {path: "/c"},
	}
	if isSequentialCrawl(records) {
		t.Error("expected false for < 5 records")
	}
}

func TestIsSequentialCrawl_OrderedPaths_True(t *testing.T) {
	records := []requestRecord{
		{path: "/apple"}, {path: "/banana"}, {path: "/cherry"},
		{path: "/date"}, {path: "/elderberry"}, {path: "/fig"},
	}
	if !isSequentialCrawl(records) {
		t.Error("expected true for strictly ascending paths")
	}
}

func TestIsSequentialCrawl_RandomPaths_False(t *testing.T) {
	records := []requestRecord{
		{path: "/z"}, {path: "/a"}, {path: "/m"},
		{path: "/b"}, {path: "/q"}, {path: "/c"},
	}
	if isSequentialCrawl(records) {
		t.Error("expected false for random/unordered paths")
	}
}

// ─── Text-heavy pattern ───────────────────────────────────────────────────────

func TestIsTextHeavyPattern_FewRequests_False(t *testing.T) {
	records := []requestRecord{
		{path: "/index.html"}, {path: "/about.html"},
	}
	if isTextHeavyPattern(records) {
		t.Error("expected false for < 5 records")
	}
}

func TestIsTextHeavyPattern_MostlyHTML_True(t *testing.T) {
	records := []requestRecord{
		{path: "/index.html"}, {path: "/about.html"}, {path: "/blog.html"},
		{path: "/contact.html"}, {path: "/faq.html"}, {path: "/"},
	}
	if !isTextHeavyPattern(records) {
		t.Error("expected true for mostly HTML paths")
	}
}

func TestIsTextHeavyPattern_MixedContent_False(t *testing.T) {
	records := []requestRecord{
		{path: "/index.html"}, {path: "/api/data"}, {path: "/image.png"},
		{path: "/api/users"}, {path: "/bundle.js"}, {path: "/style.css"},
	}
	if isTextHeavyPattern(records) {
		t.Error("expected false for mixed content paths")
	}
}

// ─── PruneExpired ─────────────────────────────────────────────────────────────

func TestPruneExpired_RemovesOldClients(t *testing.T) {
	az := newAnalyzer(nil)

	// inject an old client directly
	az.mu.Lock()
	az.clients["old.client"] = &clientState{
		lastSeen: time.Now().AddDate(0, 0, -10),
	}
	az.clients["new.client"] = &clientState{
		lastSeen: time.Now(),
	}
	az.mu.Unlock()

	pruned := az.PruneExpired(7)
	if pruned != 1 {
		t.Errorf("expected 1 pruned client, got %d", pruned)
	}

	az.mu.RLock()
	_, oldExists := az.clients["old.client"]
	_, newExists := az.clients["new.client"]
	az.mu.RUnlock()

	if oldExists {
		t.Error("expected old.client to be pruned")
	}
	if !newExists {
		t.Error("expected new.client to remain")
	}
}

func TestPruneExpired_ZeroDays_NoPrune(t *testing.T) {
	az := newAnalyzer(nil)
	az.mu.Lock()
	az.clients["old.client"] = &clientState{
		lastSeen: time.Now().AddDate(0, 0, -100),
	}
	az.mu.Unlock()

	pruned := az.PruneExpired(0)
	if pruned != 0 {
		t.Errorf("expected 0 pruned with days=0, got %d", pruned)
	}
}

func TestPruneExpired_ZeroLastSeen_NotPruned(t *testing.T) {
	az := newAnalyzer(nil)
	az.mu.Lock()
	az.clients["zero.client"] = &clientState{} // lastSeen is zero value
	az.mu.Unlock()

	pruned := az.PruneExpired(1)
	if pruned != 0 {
		t.Errorf("expected client with zero lastSeen to be skipped, got %d pruned", pruned)
	}
}

// ─── Snapshot / Status ────────────────────────────────────────────────────────

func TestSnapshot_ReturnsAllClients(t *testing.T) {
	az := newAnalyzer(nil)
	az.Score(newReq("/a"), "1.1.1.1")
	az.Score(newReq("/b"), "2.2.2.2")

	snap := az.Snapshot()
	if len(snap) != 2 {
		t.Errorf("expected 2 clients in snapshot, got %d", len(snap))
	}
}

func TestStatus_ReturnsClientStatus(t *testing.T) {
	az := newAnalyzer([]string{"/private"})
	az.Score(newReq("/private"), "1.1.1.1")

	status := az.Status()
	cs, ok := status["1.1.1.1"]
	if !ok {
		t.Fatal("expected 1.1.1.1 in status")
	}
	if !cs.RobotsViolated {
		t.Error("expected RobotsViolated to be true")
	}
	if cs.Score < WeightRobotsViolation {
		t.Errorf("expected score >= %.0f, got %.1f", WeightRobotsViolation, cs.Score)
	}
}

// ─── Concurrency ─────────────────────────────────────────────────────────────

func TestScore_ConcurrentAccess_NoRace(t *testing.T) {
	az := newAnalyzer(nil)
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ip := "1.2.3.4"
			if i%2 == 0 {
				ip = "5.6.7.8"
			}
			az.Score(newReq("/page"), ip)
		}(i)
	}

	wg.Wait()
	// if we got here without a panic, the race detector will catch any actual races
}

func TestSnapshot_ConcurrentWithScore_NoRace(t *testing.T) {
	az := newAnalyzer(nil)
	var wg sync.WaitGroup

	for i := 0; i < 20; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			az.Score(newReq("/page"), "1.2.3.4")
		}()
		go func() {
			defer wg.Done()
			az.Snapshot()
		}()
	}

	wg.Wait()
}

// ─── Initial state from store ─────────────────────────────────────────────────

func TestNew_LoadsInitialState(t *testing.T) {
	initial := map[string]store.ClientRecord{
		"9.9.9.9": {
			LastScore:      88.0,
			RobotsViolated: true,
			LastSeen:       time.Now(),
		},
	}
	az := New(nil, initial)

	status := az.Status()
	cs, ok := status["9.9.9.9"]
	if !ok {
		t.Fatal("expected 9.9.9.9 to be loaded from initial state")
	}
	if !cs.RobotsViolated {
		t.Error("expected RobotsViolated from initial state")
	}
}
