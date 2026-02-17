package config

import (
	"flag"
	"fmt"
	"os"
	"runtime"
)

type Config struct {
	Token    string
	Workers  int
	MaxRepos int
	JSON     bool
	Output   string
	Verbose      bool
	VerifiedOnly bool
}

func ParseConfig() (*Config, error) {
	cfg := &Config{}

	flag.StringVar(&cfg.Token, "token", os.Getenv("GITHUB_TOKEN"), "GitHub token (default: $GITHUB_TOKEN)")
	flag.IntVar(&cfg.Workers, "workers", runtime.NumCPU(), "Concurrent clone/scan workers")
	flag.IntVar(&cfg.MaxRepos, "max-repos", 100, "Max repositories to scan")
	flag.BoolVar(&cfg.JSON, "json", false, "Output as JSON")
	flag.StringVar(&cfg.Output, "output", "", "Write results to file (default: stdout)")
	flag.BoolVar(&cfg.Verbose, "verbose", false, "Show progress/debug info")
	flag.BoolVar(&cfg.VerifiedOnly, "verified-only", false, "Only show verified credentials")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: claudleak [flags]\n\nScans public GitHub repos with .claude/ directories for leaked credentials.\n\nFlags:\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	if cfg.Token == "" {
		return nil, fmt.Errorf("GitHub token required: set GITHUB_TOKEN or use --token")
	}

	return cfg, nil
}
