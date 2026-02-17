package reporter

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/fatih/color"
	"github.com/niels/claudleak/pkg/config"
	"github.com/niels/claudleak/pkg/scanner"
	"github.com/olekukonko/tablewriter"
)

type Reporter struct {
	findings  []scanner.Finding
	jsonMode  bool
	writer    io.Writer
	repoCount int
}

func NewReporter(cfg *config.Config, repoCount int) (*Reporter, io.Closer, error) {
	var w io.Writer = os.Stdout
	var closer io.Closer

	if cfg.Output != "" {
		f, err := os.Create(cfg.Output)
		if err != nil {
			return nil, nil, fmt.Errorf("open output file: %w", err)
		}
		w = f
		closer = f
	}

	return &Reporter{
		findings:  []scanner.Finding{},
		jsonMode:  cfg.JSON,
		writer:    w,
		repoCount: repoCount,
	}, closer, nil
}

func (r *Reporter) Add(f scanner.Finding) {
	r.findings = append(r.findings, f)
}

func (r *Reporter) Render() error {
	if r.jsonMode {
		return r.renderJSON()
	}
	return r.renderTable()
}

func (r *Reporter) renderJSON() error {
	enc := json.NewEncoder(r.writer)
	enc.SetIndent("", "  ")
	return enc.Encode(r.findings)
}

func (r *Reporter) renderTable() error {
	bold := color.New(color.Bold)
	if len(r.findings) == 0 {
		bold.Fprintf(r.writer, "\nScanned %d repos, no secrets found.\n", r.repoCount)
		return nil
	}

	bold.Fprintf(r.writer, "\nScanned %d repos, found %d secrets:\n\n", r.repoCount, len(r.findings))

	table := tablewriter.NewTable(r.writer)
	table.Header("Repository", "File", "Secret Type", "Verified", "Commit")

	for _, f := range r.findings {
		verified := "no"
		if f.Verified {
			verified = "YES"
		}
		short := f.Commit
		if len(short) > 7 {
			short = short[:7]
		}
		table.Append(f.Repository, f.File, f.SecretType, verified, short)
	}

	return table.Render()
}

func (r *Reporter) HasFindings() bool {
	return len(r.findings) > 0
}
