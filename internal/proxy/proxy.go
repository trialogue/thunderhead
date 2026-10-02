package proxy

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"text/template"
	"time"

	"github.com/trialogue/thunderhead/internal/allowlist"
	"github.com/trialogue/thunderhead/internal/analyzer"
	"github.com/trialogue/thunderhead/internal/blocklist"
	"github.com/trialogue/thunderhead/internal/config"
	"github.com/trialogue/thunderhead/internal/logger"
	"github.com/trialogue/thunderhead/internal/metrics"
)

type Proxy struct {
	cfg       *config.Config
	analyzer  *analyzer.Analyzer
	logger    *logger.Logger
	upstream  *httputil.ReverseProxy
	allowlist *allowlist.Allowlist
	blocklist *blocklist.Blocklist
	metrics   *metrics.Counters
	dryRun    bool
	startTime time.Time
}

//go:embed dashboard.html
var dashboardHTML string

type dashboardData struct {
	CSS             string
	ListenAddr      string
	UpstreamURL     string
	ClientCount     int
	TarpitThreshold float64
	BlockThreshold  float64
	Rows            string
}

func New(cfg *config.Config, az *analyzer.Analyzer, log *logger.Logger, al *allowlist.Allowlist, bl *blocklist.Blocklist) (*Proxy, error) {
	target, err := url.Parse(cfg.UpstreamURL)
	if err != nil {
		return nil, err
	}

	rp := httputil.NewSingleHostReverseProxy(target)
	rp.Director = func(req *http.Request) {
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.Host = target.Host
	}

	return &Proxy{
		cfg:       cfg,
		analyzer:  az,
		logger:    log,
		upstream:  rp,
		allowlist: al,
		blocklist: bl,
		startTime: time.Now(),
	}, nil
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	// API routes
	if strings.HasPrefix(r.URL.Path, "/api/v1/") {
		p.apiMux().ServeHTTP(w, r)
		return
	}

	if r.URL.Path == "/thunderhead/status" {
		p.handleStatus(w, r)
		return
	}

	ip := extractIP(r)
	if p.allowlist.IsAllowed(ip, r.Header.Get("User-Agent")) {
		p.upstream.ServeHTTP(w, r)
		return
	}

	if p.blocklist.IsBlocked(ip) {
		p.logger.Log(logger.Entry{
			IP:        ip,
			Method:    r.Method,
			Path:      r.URL.Path,
			Score:     100,
			Action:    "blocklist",
			UserAgent: r.Header.Get("User-Agent"),
		})
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	score := p.analyzer.Score(r, ip)

	action := "allow"
	switch {
	case score >= p.cfg.Thresholds.Block:
		action = "block"
	case score >= p.cfg.Thresholds.Tarpit:
		action = "tarpit"
	}

	p.logger.Log(logger.Entry{
		IP:        ip,
		Method:    r.Method,
		Path:      r.URL.Path,
		Score:     score,
		Action:    action,
		UserAgent: r.Header.Get("User-Agent"),
	})

	if p.metrics != nil {
		p.metrics.Total.Add(1)
		switch action {
		case "block":
			p.metrics.Blocked.Add(1)
		case "tarpit":
			p.metrics.Tarpit.Add(1)
		default:
			p.metrics.Allowed.Add(1)
		}
	}

	switch action {
	case "block":
		if !p.dryRun {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
	case "tarpit":
		if !p.dryRun {
			time.Sleep(p.cfg.Tarpit.Delay)
		}
	}

	p.upstream.ServeHTTP(w, r)
}

func (p *Proxy) handleStatus(w http.ResponseWriter, r *http.Request) {
	status := p.analyzer.Status()

	// Serve JSON if requested via API
	accept := r.Header.Get("Accept")
	if strings.Contains(accept, "application/json") {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"clients": status,
		})
		return
	}

	// Otherwise serve the HTML dashboard
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(renderDashboard(status, p.cfg)))
}

func renderDashboard(status map[string]analyzer.ClientStatus, cfg *config.Config) string {
	rows := ""
	for ip, client := range status {
		action := "allow"
		actionClass := "allow"
		if client.Score >= cfg.Thresholds.Block {
			action = "block"
			actionClass = "block"
		} else if client.Score >= cfg.Thresholds.Tarpit {
			action = "tarpit"
			actionClass = "tarpit"
		}

		robotsStr := "no"
		if client.RobotsViolated {
			robotsStr = "yes"
		}

		rows += fmt.Sprintf(`
		<tr class="%s">
			<td>%s</td>
			<td>%.1f</td>
			<td>%d</td>
			<td>%s</td>
			<td><span class="badge %s">%s</span></td>
		</tr>`, actionClass, ip, client.Score, client.RequestCount, robotsStr, actionClass, action)
	}

	if rows == "" {
		rows = `<tr><td colspan="5" class="empty">No clients tracked yet</td></tr>`
	}

	tmpl, err := template.New("dashboard").Parse(dashboardHTML)
	if err != nil {
		return "template error: " + err.Error()
	}

	data := dashboardData{
		ListenAddr:      cfg.ListenAddr,
		UpstreamURL:     cfg.UpstreamURL,
		ClientCount:     len(status),
		TarpitThreshold: cfg.Thresholds.Tarpit,
		BlockThreshold:  cfg.Thresholds.Block,
		Rows:            rows,
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return "render error: " + err.Error()
	}
	return buf.String()
}

func extractIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

func NewWithMetrics(cfg *config.Config, az *analyzer.Analyzer, log *logger.Logger, al *allowlist.Allowlist, bl *blocklist.Blocklist, mx *metrics.Counters, dryRun bool) (*Proxy, error) {
	p, err := New(cfg, az, log, al, bl)
	if err != nil {
		return nil, err
	}
	p.metrics = mx
	p.dryRun = dryRun
	return p, nil
}
