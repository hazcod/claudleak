package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"

	"github.com/niels/claudleak/pkg/cloner"
	"github.com/niels/claudleak/pkg/config"
	"github.com/niels/claudleak/pkg/discovery"
	"github.com/niels/claudleak/pkg/reporter"
	"github.com/niels/claudleak/pkg/scanner"
)

func main() {
	cfg, err := config.ParseConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(2)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	// 1. Discover repos
	if cfg.Verbose {
		fmt.Println("[claudleak] discovering repos with .claude/ files...")
	}
	repos, err := discovery.Discover(ctx, cfg.Token, cfg.MaxRepos, cfg.Org, cfg.Verbose)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error discovering repos: %v\n", err)
		os.Exit(1)
	}
	if len(repos) == 0 {
		fmt.Println("No repositories found with .claude/ files.")
		os.Exit(0)
	}
	if cfg.Verbose {
		fmt.Printf("[claudleak] found %d repos, starting clone+scan...\n", len(repos))
	}

	// 2. Create temp dir for clones
	tmpDir, err := os.MkdirTemp("", "claudleak-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating temp dir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir)

	// 3. Clone + scan with worker pool
	rpt, closer, err := reporter.NewReporter(cfg, len(repos))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if closer != nil {
		defer closer.Close()
	}

	var (
		mu        sync.Mutex
		wg        sync.WaitGroup
		scanned   int
		semaphore = make(chan struct{}, cfg.Workers)
	)

loop:
	for _, repo := range repos {
		select {
		case <-ctx.Done():
			break loop
		default:
		}

		wg.Add(1)
		semaphore <- struct{}{}

		go func(repo discovery.RepoInfo) {
			defer wg.Done()
			defer func() { <-semaphore }()

			// Clone
			repoDir, err := cloner.CloneRepo(ctx, repo, tmpDir, cfg.Verbose)
			if err != nil {
				fmt.Fprintf(os.Stderr, "[error] clone %s: %v\n", repo.FullName, err)
				mu.Lock()
				scanned++
				mu.Unlock()
				return
			}

			// Scan
			findings, err := scanner.ScanRepo(ctx, repoDir, repo, cfg.Verbose)
			if err != nil {
				fmt.Fprintf(os.Stderr, "[error] scan %s: %v\n", repo.FullName, err)
				mu.Lock()
				scanned++
				mu.Unlock()
				return
			}

			_ = os.RemoveAll(repoDir)

			mu.Lock()
			for _, f := range findings {
				if cfg.VerifiedOnly && !f.Verified {
					continue
				}
				rpt.Add(f)
			}
			scanned++
			if !cfg.Verbose && !cfg.JSON {
				fmt.Fprintf(os.Stderr, "\r[claudleak] scanned %d/%d repos...", scanned, len(repos))
			}
			mu.Unlock()
		}(repo)
	}

	wg.Wait()

	if !cfg.Verbose && !cfg.JSON {
		fmt.Fprintln(os.Stderr)
	}

	// 4. Report
	if err := rpt.Render(); err != nil {
		fmt.Fprintf(os.Stderr, "Error rendering report: %v\n", err)
		os.Exit(1)
	}

	if rpt.HasFindings() {
		os.Exit(1)
	}
}
