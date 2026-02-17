package cloner

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/niels/claudleak/pkg/discovery"
)

func CloneRepo(ctx context.Context, repo discovery.RepoInfo, baseDir string, verbose bool) (string, error) {
	dest := filepath.Join(baseDir, repo.Owner, repo.Name)
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return "", fmt.Errorf("mkdir: %w", err)
	}

	args := []string{"clone", "--quiet", repo.CloneURL, dest}
	if verbose {
		fmt.Printf("[clone] cloning %s\n", repo.FullName)
	}

	cmd := exec.CommandContext(ctx, "git", args...)
	stderr, err := cmd.CombinedOutput()
	if err != nil {
		msg := string(stderr)
		if msg != "" {
			return "", fmt.Errorf("git clone %s: %s", repo.FullName, msg)
		}
		return "", fmt.Errorf("git clone %s: %w", repo.FullName, err)
	}

	return dest, nil
}
