package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/thunderheadhq/thunderhead/internal/allowlist"
	"github.com/thunderheadhq/thunderhead/internal/analyzer"
	"github.com/thunderheadhq/thunderhead/internal/blocklist"
	"github.com/thunderheadhq/thunderhead/internal/config"
	"github.com/thunderheadhq/thunderhead/internal/logger"
	"github.com/thunderheadhq/thunderhead/internal/metrics"
	"github.com/thunderheadhq/thunderhead/internal/proxy"
	"github.com/thunderheadhq/thunderhead/internal/store"
)

// Styles

var (
	styleDim   = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	styleMuted = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	styleWhite = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	styleBold  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("255"))

	styleAllow  = lipgloss.NewStyle().Foreground(lipgloss.Color("34"))
	styleTarpit = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	styleBlock  = lipgloss.NewStyle().Foreground(lipgloss.Color("160"))
	styleWarn   = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))

	styleDivider = lipgloss.NewStyle().Foreground(lipgloss.Color("236"))

	styleColHeader = lipgloss.NewStyle().
			Foreground(lipgloss.Color("238")).
			Bold(true)
)

// Messages

type tickMsg time.Time
type logMsg logger.Entry

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func waitForLog(feed chan logger.Entry) tea.Cmd {
	return func() tea.Msg { return logMsg(<-feed) }
}

// Model

const (
	visibleRows   = 14
	maxLogEntries = 500
)

type model struct {
	az      *analyzer.Analyzer
	mx      *metrics.Counters
	cfg     *config.Config
	feed    chan logger.Entry
	startAt time.Time

	clients   map[string]analyzer.ClientStatus
	total     int64
	allowed   int64
	tarpit    int64
	blocked   int64
	uptime    time.Duration
	scroll    int
	logs      []logger.Entry
	logScroll int
	mode      string // "clients" or "log"
	quitting  bool
}

func newModel(az *analyzer.Analyzer, mx *metrics.Counters, cfg *config.Config, feed chan logger.Entry) model {
	return model{
		az:      az,
		mx:      mx,
		cfg:     cfg,
		feed:    feed,
		startAt: time.Now(),
		mode:    "clients",
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(tick(), waitForLog(m.feed))
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tickMsg:
		m.clients = m.az.Status()
		m.total = m.mx.Total.Load()
		m.allowed = m.mx.Allowed.Load()
		m.tarpit = m.mx.Tarpit.Load()
		m.blocked = m.mx.Blocked.Load()
		m.uptime = time.Since(m.startAt).Truncate(time.Second)
		return m, tick()

	case logMsg:
		e := logger.Entry(msg)
		m.logs = append(m.logs, e)
		if len(m.logs) > maxLogEntries {
			m.logs = m.logs[len(m.logs)-maxLogEntries:]
		}
		return m, waitForLog(m.feed)

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		case "tab":
			if m.mode == "clients" {
				m.mode = "log"
			} else {
				m.mode = "clients"
			}
		case "j", "down":
			if m.mode == "clients" {
				max := len(m.clients) - visibleRows
				if max < 0 {
					max = 0
				}
				if m.scroll < max {
					m.scroll++
				}
			} else {
				max := len(m.logs) - visibleRows
				if max < 0 {
					max = 0
				}
				if m.logScroll < max {
					m.logScroll++
				}
			}
		case "k", "up":
			if m.mode == "clients" {
				if m.scroll > 0 {
					m.scroll--
				}
			} else {
				if m.logScroll > 0 {
					m.logScroll--
				}
			}
		}
	}
	return m, nil
}

func (m model) View() string {
	if m.quitting {
		return styleDim.Render("thunderhead stopped.\n")
	}

	var b strings.Builder
	div := styleDivider.Render(strings.Repeat("─", 80))

	// Title
	title := styleBold.Render("⚡ thunderhead")
	version := styleDim.Render("v0.1.1")
	meta := styleDim.Render(fmt.Sprintf("  %s → %s   up %s", m.cfg.ListenAddr, m.cfg.UpstreamURL, m.uptime))
	b.WriteString("  ")
	b.WriteString(title)
	b.WriteString(" ")
	b.WriteString(version)
	b.WriteString(meta)
	b.WriteString("\n")

	// Stats
	b.WriteString("\n")

	stats := []struct {
		label, val string
		style      lipgloss.Style
	}{
		{"total", fmt.Sprintf("%d", m.total), styleWhite},
		{"allowed", fmt.Sprintf("%d", m.allowed), styleAllow},
		{"tarpitted", fmt.Sprintf("%d", m.tarpit), styleTarpit},
		{"blocked", fmt.Sprintf("%d", m.blocked), styleBlock},
		{"clients", fmt.Sprintf("%d", len(m.clients)), styleWhite},
		{"tarpit@", fmt.Sprintf("%.0f", m.cfg.Thresholds.Tarpit), styleTarpit},
		{"block@", fmt.Sprintf("%.0f", m.cfg.Thresholds.Block), styleBlock},
	}

	var statParts []string
	for _, s := range stats {
		statParts = append(statParts,
			styleDim.Render(s.label+" ")+s.style.Render(s.val),
		)
	}
	b.WriteString("  ")
	b.WriteString(strings.Join(statParts, styleDivider.Render("   ·   ")))
	b.WriteString("\n\n")
	b.WriteString("  ")
	b.WriteString(div)
	b.WriteString("\n\n")

	// Tab bar
	clientsTab := styleDim.Render("clients")
	logTab := styleDim.Render("log")
	if m.mode == "clients" {
		clientsTab = styleWhite.Render("clients")
	} else {
		logTab = styleWhite.Render("log")
	}
	b.WriteString("  ")
	b.WriteString(clientsTab)
	b.WriteString(styleDivider.Render("  /  "))
	b.WriteString(logTab)
	b.WriteString("\n\n")

	// Table
	if m.mode == "clients" {
		m.renderClients(&b)
	} else {
		m.renderLog(&b)
	}

	// Footer
	b.WriteString("  ")
	b.WriteString(styleDim.Render("tab"))
	b.WriteString(" ")
	b.WriteString(styleWhite.Render("switch view"))
	b.WriteString("   ")
	b.WriteString(styleDim.Render("↑↓"))
	b.WriteString(" ")
	b.WriteString(styleWhite.Render("scroll"))
	b.WriteString("   ")
	b.WriteString(styleDim.Render("q"))
	b.WriteString(" ")
	b.WriteString(styleWhite.Render("quit"))
	b.WriteString("\n")

	return b.String()
}

func (m *model) renderClients(b *strings.Builder) {
	colW := []int{24, 9, 10, 7, 8, 8}
	// indices:   0=ip 1=score 2=bar 3=reqs 4=robots 5=action

	b.WriteString("  " +
		styleColHeader.Render(pad("ip", colW[0])) +
		styleColHeader.Render(pad("score", colW[1])) +
		styleColHeader.Render(pad("bar", colW[2])) +
		styleColHeader.Render(pad("reqs", colW[3])) +
		styleColHeader.Render(pad("robots", colW[4])) +
		styleColHeader.Render(pad("action", colW[5])) +
		"\n\n")

	type row struct {
		ip     string
		status analyzer.ClientStatus
	}
	rows := make([]row, 0, len(m.clients))
	for ip, cs := range m.clients {
		rows = append(rows, row{ip, cs})
	}
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].status.Score > rows[j].status.Score
	})

	visible := rows
	if m.scroll < len(rows) {
		visible = rows[m.scroll:]
	}
	if len(visible) > visibleRows {
		visible = visible[:visibleRows]
	}

	if len(rows) == 0 {
		b.WriteString("  " + styleDim.Render("no clients yet") + "\n")
		return
	}

	for _, r := range visible {
		action, as := actionInfo(r.status.Score, m.cfg)

		var ipStyle lipgloss.Style
		switch action {
		case "block":
			ipStyle = styleBlock
		case "tarpit":
			ipStyle = styleTarpit
		default:
			ipStyle = styleMuted
		}

		robots := styleDim.Render(pad("no", colW[4]))
		if r.status.RobotsViolated {
			robots = styleWarn.Render(pad("yes", colW[4]))
		}

		barWidth := 8
		filled := int((r.status.Score / 100.0) * float64(barWidth))
		if filled > barWidth {
			filled = barWidth
		}
		bar := scoreStyle(r.status.Score).Render(strings.Repeat("█", filled)) +
			styleDim.Render(strings.Repeat("░", barWidth-filled))
		barPad := strings.Repeat(" ", colW[2]-barWidth)

		score := scoreStyle(r.status.Score).Render(fmt.Sprintf("%.1f", r.status.Score))
		scorePad := strings.Repeat(" ", colW[1]-len(fmt.Sprintf("%.1f", r.status.Score)))

		b.WriteString("  " +
			ipStyle.Render(pad(r.ip, colW[0])) +
			score + scorePad +
			bar + barPad +
			styleDim.Render(pad(fmt.Sprintf("%d", r.status.RequestCount), colW[3])) +
			robots +
			as.Render(pad(action, colW[5])) +
			"\n")
	}

	if len(rows) > visibleRows {
		b.WriteString("\n  " + styleDim.Render(fmt.Sprintf("%d-%d of %d",
			m.scroll+1, min(m.scroll+visibleRows, len(rows)), len(rows))) + "\n")
	}
}

func (m *model) renderLog(b *strings.Builder) {
	colW := []int{10, 22, 8, 32, 8, 8}

	b.WriteString("  ")
	b.WriteString(styleColHeader.Render(pad("time", colW[0])))
	b.WriteString(styleColHeader.Render(pad("ip", colW[1])))
	b.WriteString(styleColHeader.Render(pad("method", colW[2])))
	b.WriteString(styleColHeader.Render(pad("path", colW[3])))
	b.WriteString(styleColHeader.Render(pad("score", colW[4])))
	b.WriteString(styleColHeader.Render(pad("action", colW[5])))
	b.WriteString("\n\n")

	if len(m.logs) == 0 {
		b.WriteString("  ")
		b.WriteString(styleDim.Render("waiting for requests..."))
		b.WriteString("\n")
		return
	}

	// newest at bottom when logScroll == 0
	end := len(m.logs) - m.logScroll
	start := end - visibleRows
	if start < 0 {
		start = 0
	}
	if end < 0 {
		end = 0
	}

	for _, e := range m.logs[start:end] {
		_, as := actionInfo(e.Score, m.cfg)
		b.WriteString("  ")
		b.WriteString(styleDim.Render(pad(e.Time.Format("15:04:05"), colW[0])))
		b.WriteString(styleMuted.Render(pad(e.IP, colW[1])))
		b.WriteString(styleDim.Render(pad(e.Method, colW[2])))
		b.WriteString(styleWhite.Render(pad(e.Path, colW[3])))
		b.WriteString(scoreStyle(e.Score).Render(pad(fmt.Sprintf("%.1f", e.Score), colW[4])))
		b.WriteString(as.Render(pad(e.Action, colW[5])))
		b.WriteString("\n")
	}

	if len(m.logs) > visibleRows {
		b.WriteString("\n  ")
		b.WriteString(styleDim.Render(fmt.Sprintf("%d-%d of %d",
			start+1, end, len(m.logs))))
		b.WriteString("\n")
	}
}

// Helpers

func actionInfo(score float64, cfg *config.Config) (string, lipgloss.Style) {
	switch {
	case score >= cfg.Thresholds.Block:
		return "block", styleBlock
	case score >= cfg.Thresholds.Tarpit:
		return "tarpit", styleTarpit
	default:
		return "allow", styleAllow
	}
}

func scoreStyle(score float64) lipgloss.Style {
	switch {
	case score >= 75:
		return styleBlock
	case score >= 40:
		return styleTarpit
	default:
		return styleAllow
	}
}

func pad(s string, w int) string {
	if len(s) >= w {
		return s[:w]
	}
	return s + strings.Repeat(" ", w-len(s))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Main

func main() {
	cfgPath := flag.String("config", "", "path to config file (optional)")
	statePath := flag.String("state", "state.json", "path to state file")
	dryRun := flag.Bool("dry-run", false, "score requests but never tarpit or block")
	flag.Parse()

	log.SetOutput(os.Stderr)

	var cfg *config.Config
	if *cfgPath != "" {
		var err error
		cfg, err = config.Load(*cfgPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
			os.Exit(1)
		}
	} else {
		cfg = config.Default()
	}

	st := store.New(*statePath)
	state, err := st.Load()
	if err != nil {
		state = &store.State{Clients: make(map[string]store.ClientRecord)}
	}

	fetched := analyzer.FetchDisallowedPaths(cfg.UpstreamURL)
	disallowed := append(fetched, cfg.DisallowedPaths...)
	az := analyzer.New(disallowed, state.Clients)

	if cfg.LogFile == "" {
		cfg.LogFile = "thunderhead.log"
	}
	lg, err := logger.New(cfg.LogFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to init logger: %v\n", err)
		os.Exit(1)
	}

	feed := make(chan logger.Entry, 256)
	lg.Feed = feed

	mx := metrics.New()
	al := allowlist.New(cfg.Allowlist)
	bl := blocklist.New(cfg.Blocklist)

	p, err := proxy.NewWithMetrics(cfg, az, lg, al, bl, mx, *dryRun)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to init proxy: %v\n", err)
		os.Exit(1)
	}

	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			st.Save(&store.State{Clients: az.Snapshot()})
		}
	}()

	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			pruned := az.PruneExpired(cfg.ExpiryDays)
			if pruned > 0 {
				fmt.Fprintf(os.Stderr, "thunderhead: pruned %d expired clients\n", pruned)
			}
		}
	}()

	watchConfig(*cfgPath, cfg)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-quit
		st.Save(&store.State{Clients: az.Snapshot()})
		os.Exit(0)
	}()

	go func() {
		var err error
		if cfg.TLSCert != "" && cfg.TLSKey != "" {
			fmt.Fprintf(os.Stderr, "thunderhead: TLS enabled\n")
			err = http.ListenAndServeTLS(cfg.ListenAddr, cfg.TLSCert, cfg.TLSKey, p)
		} else {
			err = http.ListenAndServe(cfg.ListenAddr, p)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "server error: %v\n", err)
			os.Exit(1)
		}
	}()

	prog := tea.NewProgram(
		newModel(az, mx, cfg, feed),
		tea.WithAltScreen(),
	)
	if _, err := prog.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "tui error: %v\n", err)
		os.Exit(1)
	}
}

func watchConfig(path string, cfg *config.Config) {
	if path == "" {
		return
	}

	var lastMod time.Time

	go func() {
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			info, err := os.Stat(path)
			if err != nil {
				continue
			}
			if info.ModTime().After(lastMod) {
				lastMod = info.ModTime()
				newCfg, err := config.Load(path)
				if err != nil {
					fmt.Fprintf(os.Stderr, "thunderhead: config reload failed: %v\n", err)
					continue
				}
				cfg.Thresholds = newCfg.Thresholds
				cfg.Tarpit = newCfg.Tarpit
				cfg.Allowlist = newCfg.Allowlist
				cfg.Blocklist = newCfg.Blocklist
				cfg.ExpiryDays = newCfg.ExpiryDays
				fmt.Fprintf(os.Stderr, "thunderhead: config reloaded\n")
			}
		}
	}()
}
