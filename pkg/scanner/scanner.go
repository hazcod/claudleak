package scanner

import (
	"context"
	"fmt"
	"net/url"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/niels/claudleak/pkg/discovery"

	trufflectx "github.com/trufflesecurity/trufflehog/v3/pkg/context"
	"github.com/trufflesecurity/trufflehog/v3/pkg/decoders"
	"github.com/trufflesecurity/trufflehog/v3/pkg/detectors"
	"github.com/trufflesecurity/trufflehog/v3/pkg/engine"
	"github.com/trufflesecurity/trufflehog/v3/pkg/engine/defaults"
	"github.com/trufflesecurity/trufflehog/v3/pkg/pb/source_metadatapb"
	"github.com/trufflesecurity/trufflehog/v3/pkg/sources"
)

type Finding struct {
	Repository string    `json:"repository"`
	File       string    `json:"file"`
	Commit     string    `json:"commit"`
	Date       time.Time `json:"date"`
	SecretType string    `json:"secret_type"`
	Match      string    `json:"match"`
	Verified   bool      `json:"verified"`
	URL        string    `json:"url"`
}

// collectingDispatcher receives results from TruffleHog's engine.
type collectingDispatcher struct {
	mu      sync.Mutex
	results []detectors.ResultWithMetadata
}

func (d *collectingDispatcher) Dispatch(_ trufflectx.Context, result detectors.ResultWithMetadata) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.results = append(d.results, result)
	return nil
}

// ScanRepo scans a cloned git repo for secrets using TruffleHog and returns
// findings filtered to AI coding tool config paths.
func ScanRepo(parentCtx context.Context, repoPath string, repo discovery.RepoInfo, verbose bool) ([]Finding, error) {
	ctx := trufflectx.WithLogger(parentCtx, logr.Discard())

	absPath, err := filepath.Abs(repoPath)
	if err != nil {
		return nil, fmt.Errorf("abs path: %w", err)
	}
	repoURI := (&url.URL{Scheme: "file", Path: filepath.ToSlash(absPath)}).String()

	dispatcher := &collectingDispatcher{}

	sourceManager := sources.NewManager(
		sources.WithSourceUnits(),
		sources.WithBufferedOutput(64),
		sources.WithConcurrentSources(runtime.NumCPU()),
	)

	conf := engine.Config{
		Concurrency:   runtime.NumCPU(),
		Decoders:      decoders.DefaultDecoders(),
		Detectors:     defaults.DefaultDetectors(),
		Verify:        true,
		SourceManager: sourceManager,
		Dispatcher:    dispatcher,
		Results: map[string]struct{}{
			"verified":   {},
			"unverified": {},
			"unknown":    {},
		},
	}

	eng, err := engine.NewEngine(ctx, &conf)
	if err != nil {
		return nil, fmt.Errorf("engine init: %w", err)
	}
	eng.Start(ctx)

	gitCfg := sources.GitConfig{
		URI: repoURI,
	}

	if _, err := eng.ScanGit(ctx, gitCfg); err != nil {
		return nil, fmt.Errorf("scan git: %w", err)
	}

	if err := eng.Finish(ctx); err != nil {
		return nil, fmt.Errorf("engine finish: %w", err)
	}

	// Filter to .claude/ files and CLAUDE.md, convert to Finding
	var findings []Finding
	for _, r := range dispatcher.results {
		gitMeta, ok := r.SourceMetadata.GetData().(*source_metadatapb.MetaData_Git)
		if !ok {
			continue
		}
		g := gitMeta.Git
		file := g.GetFile()

		if !isTargetFile(file) {
			continue
		}

		commitDate, _ := time.Parse("2006-01-02 15:04:05 -0700 MST", g.GetTimestamp())

		commit := g.GetCommit()
		fileURL := fmt.Sprintf("%s/blob/%s/%s", repo.HTMLURL, commit, file)

		findings = append(findings, Finding{
			Repository: repo.FullName,
			File:       file,
			Commit:     commit,
			Date:       commitDate,
			SecretType: r.DetectorType.String(),
			Match:      r.Redacted,
			Verified:   r.Verified,
			URL:        fileURL,
		})
	}

	if verbose && len(findings) > 0 {
		fmt.Printf("[scan] %s: found %d secrets in AI config files\n", repo.FullName, len(findings))
	}

	return findings, nil
}

// targetDirs are directory prefixes for AI coding tool configs.
var targetDirs = []string{
	".claude/",      // Claude Code
	".cursor/",      // Cursor
	".continue/",    // Continue.dev
	".codex/",       // OpenAI Codex CLI
	".sourcegraph/", // Sourcegraph Cody
	".amazonq/",     // Amazon Q
}

// targetFiles are exact filenames (matched case-insensitively at any depth).
var targetFiles = []string{
	"claude.md",       // Claude Code
	"agents.md",       // OpenAI Codex CLI
	".aider.conf.yml", // Aider
	".cursorrules",    // Cursor (legacy)
	".windsurfrules",  // Windsurf
}

func isTargetFile(path string) bool {
	normalized := strings.ToLower(filepath.ToSlash(path))
	for _, dir := range targetDirs {
		if strings.Contains(normalized, dir) {
			return true
		}
	}
	base := filepath.Base(normalized)
	for _, name := range targetFiles {
		if base == name {
			return true
		}
	}
	return false
}
