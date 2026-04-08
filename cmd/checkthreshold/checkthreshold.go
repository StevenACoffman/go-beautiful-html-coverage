// Package checkthreshold implements the "check-threshold" CLI command.
package checkthreshold

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/peterbourgon/ff/v4"
	"github.com/StevenACoffman/go-beautiful-html-coverage/cmd/normalizepath"
	"github.com/StevenACoffman/go-beautiful-html-coverage/cmd/root"
)

// Config holds the configuration for the check-threshold command.
type Config struct {
	*root.Config

	path      string
	revision  string
	threshold float64
}

// New creates and registers the check-threshold command with the given parent config.
func New(parent *root.Config) *Config {
	cfg := &Config{Config: parent}
	fset := ff.NewFlagSet("check-threshold").SetParent(parent.Flags)
	fset.StringVar(&cfg.path, 0, "path", "", "normalized sub-path (default: repo root)")
	fset.StringVar(&cfg.revision, 0, "revision", "", "git revision SHA (required)")
	fset.Float64Var(&cfg.threshold, 0, "threshold", 0, "minimum coverage percentage required")
	cmd := &ff.Command{
		Name:      "check-threshold",
		Usage:     "go-beautiful-html-coverage check-threshold --revision <sha> [--threshold <pct>] [--path <dir>]",
		ShortHelp: "fail if coverage is below the given threshold",
		LongHelp: `Reads go-cover/{path}/revisions/{revision}.txt, parses the total
coverage percentage from the last line, and exits non-zero if it falls
below --threshold.`,
		Flags: fset,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cmd)
	return cfg
}

func (cfg *Config) exec(_ context.Context, _ []string) error {
	if cfg.revision == "" {
		return fmt.Errorf("--revision is required")
	}

	path := normalizepath.Normalize(cfg.path)
	coverage, summary, err := readCoverage(path, cfg.revision)
	if err != nil {
		return err
	}

	if coverage < cfg.threshold {
		_, _ = fmt.Fprintf(cfg.Stdout, "\x1b[91m✘ coverage %s < %.1f%%\n", summary, cfg.threshold)
		return fmt.Errorf("coverage %.1f%% is below threshold %.1f%%", coverage, cfg.threshold)
	}

	_, _ = fmt.Fprintf(cfg.Stdout, "\x1b[92m✔ coverage %s >= %.1f%%\n", summary, cfg.threshold)
	return nil
}

// readCoverage reads the total coverage percentage from go-cover/{path}/revisions/{revision}.txt.
// It returns the parsed float64, the raw summary string (e.g. "87.3%"), and any error.
func readCoverage(path, revision string) (float64, string, error) {
	txtPath := filepath.Join("go-cover", path, "revisions", revision+".txt")
	data, err := os.ReadFile(txtPath)
	if err != nil {
		return 0, "", fmt.Errorf("read coverage file %s: %w", txtPath, err)
	}

	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) == 0 {
		return 0, "", fmt.Errorf("coverage file %s is empty", txtPath)
	}

	last := lines[len(lines)-1]
	fields := strings.Split(last, "\t")
	summary := strings.TrimSpace(fields[len(fields)-1])

	pctStr := strings.TrimSuffix(summary, "%")
	coverage, err := strconv.ParseFloat(pctStr, 64)
	if err != nil {
		return 0, "", fmt.Errorf("parse coverage %q: %w", summary, err)
	}

	return coverage, summary, nil
}
