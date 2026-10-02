package config

import (
	"encoding/json"
	"os"
	"time"

	"github.com/trialogue/thunderhead/internal/allowlist"
	"github.com/trialogue/thunderhead/internal/blocklist"
)

type Action string

const (
	ActionLog    Action = "log"
	ActionTarpit Action = "tarpit"
	ActionBlock  Action = "block"
)

type Thresholds struct {
	Tarpit float64 `json:"tarpit"` // score >= this -> tarpit
	Block  float64 `json:"block"`  // score >= this -> block
}

type TarpitConfig struct {
	Delay time.Duration `json:"delay"` // how long to delay tarpitted requests
}

type Config struct {
	ListenAddr      string           `json:"listen_addr"`
	UpstreamURL     string           `json:"upstream_url"`
	Thresholds      Thresholds       `json:"thresholds"`
	Tarpit          TarpitConfig     `json:"tarpit"`
	LogFile         string           `json:"log_file"`
	ExpiryDays      int              `json:"expiry_days"`
	Allowlist       allowlist.Config `json:"allowlist"`
	Blocklist       blocklist.Config `json:"blocklist"`
	TLSCert         string           `json:"tls_cert"`
	TLSKey          string           `json:"tls_key"`
	APIKey          string           `json:"api_key"`
	DisallowedPaths []string         `json:"disallowed_paths"`
}

func Default() *Config {
	return &Config{
		ListenAddr:  ":8080",
		UpstreamURL: "http://localhost:3000",
		Thresholds: Thresholds{
			Tarpit: 40.0,
			Block:  75.0,
		},
		Tarpit: TarpitConfig{
			Delay: 5 * time.Second,
		},
		LogFile:    "",
		ExpiryDays: 30,
		DisallowedPaths: []string{
			"/admin",
			"/admin/",
			"/.env",
			"/config",
			"/backup",
			"/wp-admin",
			"/phpmyadmin",
		},
		Allowlist: allowlist.Config{
			IPs:        []string{},
			UserAgents: []string{"Googlebot", "Bingbot", "archive.org_bot"},
		},
		Blocklist: blocklist.Config{
			CIDRs: []string{},
			IPs:   []string{},
		},
	}
}

func Load(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	cfg := Default()
	if err := json.NewDecoder(f).Decode(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
