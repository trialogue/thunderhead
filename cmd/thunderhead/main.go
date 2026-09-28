package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/thunderheadhq/thunderhead/internal/allowlist"
	"github.com/thunderheadhq/thunderhead/internal/analyzer"
	"github.com/thunderheadhq/thunderhead/internal/blocklist"
	"github.com/thunderheadhq/thunderhead/internal/config"
	"github.com/thunderheadhq/thunderhead/internal/logger"
	"github.com/thunderheadhq/thunderhead/internal/metrics"
	"github.com/thunderheadhq/thunderhead/internal/proxy"
	"github.com/thunderheadhq/thunderhead/internal/store"
)

func main() {
	cfgPath := flag.String("config", "", "path to config file (optional)")
	statePath := flag.String("state", "state.json", "path to state file")
	flag.Parse()

	var cfg *config.Config
	if *cfgPath != "" {
		var err error
		cfg, err = config.Load(*cfgPath)
		if err != nil {
			log.Fatalf("failed to load config: %v", err)
		}
	} else {
		cfg = config.Default()
	}

	log.Printf("thunderhead starting on %s -> %s", cfg.ListenAddr, cfg.UpstreamURL)
	log.Printf("thresholds: tarpit=%.0f block=%.0f", cfg.Thresholds.Tarpit, cfg.Thresholds.Block)

	// Load persisted state
	st := store.New(*statePath)
	state, err := st.Load()
	if err != nil {
		log.Printf("warning: could not load state: %v", err)
		state = &store.State{Clients: make(map[string]store.ClientRecord)}
	}
	log.Printf("thunderhead: loaded %d known clients from state", len(state.Clients))

	fetched := analyzer.FetchDisallowedPaths(cfg.UpstreamURL)
	disallowed := append(fetched, cfg.DisallowedPaths...)
	az := analyzer.New(disallowed, state.Clients)

	lg, err := logger.New(cfg.LogFile)
	if err != nil {
		log.Fatalf("failed to init logger: %v", err)
	}

	al := allowlist.New(cfg.Allowlist)
	bl := blocklist.New(cfg.Blocklist)
	mx := metrics.New()
	p, err := proxy.NewWithMetrics(cfg, az, lg, al, bl, mx, false)
	if err != nil {
		log.Fatalf("failed to init proxy: %v", err)
	}

	// Periodic state flush every 60 seconds
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			if err := st.Save(&store.State{Clients: az.Snapshot()}); err != nil {
				log.Printf("warning: could not save state: %v", err)
			} else {
				log.Printf("thunderhead: state flushed to disk")
			}
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

	// Graceful shutdown on SIGINT/SIGTERM
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-quit
		log.Printf("thunderhead: shutting down, saving state...")
		if err := st.Save(&store.State{Clients: az.Snapshot()}); err != nil {
			log.Printf("warning: final state save failed: %v", err)
		} else {
			log.Printf("thunderhead: state saved, goodbye")
		}
		os.Exit(0)
	}()

	if err := http.ListenAndServe(cfg.ListenAddr, p); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
