package discovery

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/go-github/v68/github"
)

var searchQueries = []string{
	// Claude Code
	"path:.claude/ filename:settings.json",
	"path:.claude/ filename:settings.local.json",
	"path:.claude/commands",
	"path:.claude filename:CLAUDE.md",

	// Cursor
	"path:.cursor filename:mcp.json",

	// Continue.dev
	"path:.continue filename:config.yaml",
	"path:.continue filename:config.json",

	// Aider
	"filename:.aider.conf.yml",

	// OpenAI Codex CLI
	"path:.codex filename:config.toml",
	"filename:AGENTS.md",

	// Sourcegraph Cody
	"path:.sourcegraph filename:cody.json",

	// Amazon Q
	"path:.amazonq/rules",
}

type RepoInfo struct {
	Owner    string
	Name     string
	FullName string
	CloneURL string
	HTMLURL  string
}

func Discover(ctx context.Context, token string, maxRepos int, org string, verbose bool) ([]RepoInfo, error) {
	client := github.NewClient(nil).WithAuthToken(token)

	seen := make(map[string]bool)
	var repos []RepoInfo

	for _, query := range searchQueries {
		if len(repos) >= maxRepos {
			break
		}

		if org != "" {
			query = query + " user:" + org
		}

		if verbose {
			fmt.Printf("[discovery] searching: %s\n", query)
		}

		opts := &github.SearchOptions{
			ListOptions: github.ListOptions{PerPage: 100},
		}

		retries := 0
		for {
			result, resp, err := client.Search.Code(ctx, query, opts)
			if err != nil {
				if rlErr, ok := err.(*github.RateLimitError); ok {
					wait := time.Until(rlErr.Rate.Reset.Time) + time.Second
					if wait < 0 {
						wait = 30 * time.Second
					}
					if verbose {
						fmt.Printf("[discovery] primary rate limited, waiting %s...\n", wait.Round(time.Second))
					}
					select {
					case <-time.After(wait):
						continue
					case <-ctx.Done():
						return repos, ctx.Err()
					}
				}
				if abuseErr, ok := err.(*github.AbuseRateLimitError); ok {
					wait := abuseErr.GetRetryAfter()
					if wait == 0 {
						wait = 30 * time.Second
					}
					if verbose {
						fmt.Printf("[discovery] secondary rate limited, waiting %s...\n", wait.Round(time.Second))
					}
					select {
					case <-time.After(wait):
						continue
					case <-ctx.Done():
						return repos, ctx.Err()
					}
				}
				// Server errors (5xx) — retry up to 3 times then skip query
				if ghErr, ok := err.(*github.ErrorResponse); ok && ghErr.Response != nil && ghErr.Response.StatusCode >= 500 {
					retries++
					if retries > 3 {
						if verbose {
							fmt.Printf("[discovery] query %q: too many server errors, skipping\n", query)
						}
						break
					}
					wait := time.Duration(retries*5) * time.Second
					if verbose {
						fmt.Printf("[discovery] server error %d, retrying in %s...\n", ghErr.Response.StatusCode, wait)
					}
					select {
					case <-time.After(wait):
						continue
					case <-ctx.Done():
						return repos, ctx.Err()
					}
				}
				return nil, fmt.Errorf("search %q: %w", query, err)
			}

			for _, cr := range result.CodeResults {
				repo := cr.GetRepository()
				fullName := repo.GetFullName()
				if seen[fullName] || len(repos) >= maxRepos {
					continue
				}
				seen[fullName] = true

				parts := strings.SplitN(fullName, "/", 2)
				if len(parts) != 2 {
					continue
				}
				cloneURL := repo.GetCloneURL()
				if cloneURL == "" {
					cloneURL = fmt.Sprintf("https://github.com/%s.git", fullName)
				}
				htmlURL := repo.GetHTMLURL()
				if htmlURL == "" {
					htmlURL = fmt.Sprintf("https://github.com/%s", fullName)
				}
				repos = append(repos, RepoInfo{
					Owner:    parts[0],
					Name:     parts[1],
					FullName: fullName,
					CloneURL: cloneURL,
					HTMLURL:  htmlURL,
				})

				if verbose {
					fmt.Printf("[discovery] found: %s\n", fullName)
				}
			}

			if resp.NextPage == 0 || len(repos) >= maxRepos {
				break
			}
			opts.Page = resp.NextPage

			// Small delay between pages to avoid secondary rate limits
			time.Sleep(2 * time.Second)
		}

		// Delay between queries to avoid secondary rate limits
		time.Sleep(3 * time.Second)
	}

	return repos, nil
}
