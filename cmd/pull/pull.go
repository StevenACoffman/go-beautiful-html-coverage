// Package pull implements the "pull" CLI command.
// It checks out or creates the coverage branch in the go-cover directory.
package pull

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/peterbourgon/ff/v4"
	"github.com/StevenACoffman/go-beautiful-html-coverage/cmd/normalizepath"
	"github.com/StevenACoffman/go-beautiful-html-coverage/cmd/root"
)

// Config holds the configuration for the pull command.
type Config struct {
	*root.Config

	dir    string
	branch string
	path   string
}

// New creates and registers the pull command with the given parent config.
func New(parent *root.Config) *Config {
	cfg := &Config{Config: parent}
	fset := ff.NewFlagSet("pull").SetParent(parent.Flags)
	fset.StringVar(&cfg.dir, 0, "dir", "go-cover", "path to the coverage repository checkout")
	fset.StringVar(&cfg.branch, 0, "branch", "cover", "coverage branch name")
	fset.StringVar(&cfg.path, 0, "path", "", "sub-path for initial head files (used when creating a new branch)")
	cmd := &ff.Command{
		Name:      "pull",
		Usage:     "go-beautiful-html-coverage pull [--dir go-cover] [--branch cover] [--path <sub-path>]",
		ShortHelp: "checkout or create the coverage branch",
		LongHelp: `Fetches the coverage branch from origin.  If the branch already
exists it is checked out and pulled.  Otherwise an orphan branch is
created and the initial head placeholder files are written.`,
		Flags: fset,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cmd)
	return cfg
}

func (cfg *Config) exec(_ context.Context, _ []string) error {
	path := normalizepath.Normalize(cfg.path)
	return runPull(cfg.dir, cfg.branch, path)
}

func runPull(dir, branch, path string) error {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("resolve dir %q: %w", dir, err)
	}

	if err := gitRun(absDir, "fetch", "origin"); err != nil {
		return fmt.Errorf("git fetch: %w", err)
	}

	// Check whether the remote branch exists.
	cmd := exec.Command("git", "rev-parse", "--verify", "origin/"+branch)
	cmd.Dir = absDir
	if err := cmd.Run(); err == nil {
		// Branch exists — check it out and pull.
		if err := gitRun(absDir, "checkout", branch); err != nil {
			return fmt.Errorf("git checkout: %w", err)
		}
		if err := gitRun(absDir, "pull", "origin", branch); err != nil {
			return fmt.Errorf("git pull: %w", err)
		}
		return nil
	}

	// Branch does not exist — create an orphan branch with placeholder files.
	if err := gitRun(absDir, "checkout", "--orphan", branch); err != nil {
		return fmt.Errorf("git checkout --orphan: %w", err)
	}
	// Remove the index so git clean starts from a clean slate.
	if err := os.Remove(filepath.Join(absDir, ".git", "index")); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove git index: %w", err)
	}
	if err := gitRun(absDir, "clean", "-fdx"); err != nil {
		return fmt.Errorf("git clean: %w", err)
	}

	headDir := filepath.Join(absDir, path, "head")
	if err := os.MkdirAll(headDir, 0755); err != nil {
		return fmt.Errorf("create head dir: %w", err)
	}

	for _, name := range []string{"head.html", "head.txt"} {
		if err := os.WriteFile(filepath.Join(headDir, name), nil, 0644); err != nil {
			return fmt.Errorf("create %s: %w", name, err)
		}
	}
	if err := os.WriteFile(filepath.Join(headDir, "head.out"), []byte("mode: set\n"), 0644); err != nil {
		return fmt.Errorf("create head.out: %w", err)
	}

	return nil
}

func gitRun(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
