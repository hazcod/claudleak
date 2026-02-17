# claudleak

Scans public GitHub repositories for leaked credentials in AI coding tool configuration files (`.claude/`, `.cursor/`, `.continue/`, `.codex/`, `CLAUDE.md`, `AGENTS.md`, etc.).

Uses [TruffleHog](https://github.com/trufflesecurity/trufflehog) for secret detection.

## Install

```bash
go install github.com/niels/claudleak/cmd/claudleak@latest
```

Or build from source:

```bash
git clone https://github.com/niels/claudleak.git
cd claudleak
go build -o claudleak ./cmd/claudleak/
```

## Usage

```bash
GITHUB_TOKEN="ghp_..." ./claudleak
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--token` | `$GITHUB_TOKEN` | GitHub personal access token |
| `--max-repos` | `100` | Maximum repositories to scan |
| `--workers` | CPU count | Concurrent clone/scan workers |
| `--json` | `false` | Output results as JSON |
| `--output` | stdout | Write results to file |
| `--verified-only` | `false` | Only show verified credentials |
| `--org` / `--user` | | Only scan repos owned by this GitHub user or org |
| `--verbose` | `false` | Show progress/debug info |

### Examples

```bash
# Scan up to 50 repos, output JSON to file
claudleak --max-repos 50 --json --output results.json

# Verbose scan with 4 workers
claudleak --workers 4 --verbose

# Scan a specific org, only verified secrets
claudleak --org microsoft --verified-only
```

## How It Works

1. **Discovery** — Searches GitHub Code Search for repositories containing AI coding tool config files
2. **Clone** — Clones matching repositories to a temp directory
3. **Scan** — Runs TruffleHog against each clone, filtering findings to AI config paths
4. **Report** — Outputs a table (or JSON) of detected secrets

## Project Structure

```
cmd/claudleak/main.go    CLI entrypoint
pkg/config/              Config parsing
pkg/discovery/           GitHub repo discovery
pkg/cloner/              Git clone operations
pkg/scanner/             TruffleHog secret scanning
pkg/reporter/            Table/JSON output
```

## Exit Codes

- `0` — No secrets found
- `1` — Secrets found (or runtime error)
- `2` — Configuration error
