// Package push implements the "push" CLI command.
// It generates coverage HTML/TXT files, injects the shared assets, and pushes
// the results to the coverage branch.
package push

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/peterbourgon/ff/v4"
	"github.com/StevenACoffman/go-beautiful-html-coverage/assets"
	"github.com/StevenACoffman/go-beautiful-html-coverage/cmd/beautify"
	"github.com/StevenACoffman/go-beautiful-html-coverage/cmd/normalizepath"
	"github.com/StevenACoffman/go-beautiful-html-coverage/cmd/root"
)

// Config holds the configuration for the push command.
type Config struct {
	*root.Config

	workspace string
	path      string
	revision  string
	branch    string
	refName   string
}

// New creates and registers the push command with the given parent config.
func New(parent *root.Config) *Config {
	cfg := &Config{Config: parent}
	fset := ff.NewFlagSet("push").SetParent(parent.Flags)
	fset.StringVar(&cfg.workspace, 0, "workspace", "", "GitHub workspace directory (default: $GITHUB_WORKSPACE or current dir)")
	fset.StringVar(&cfg.path, 0, "path", "", "sub-path within the workspace (e.g. for monorepos)")
	fset.StringVar(&cfg.revision, 0, "revision", "", "git revision SHA (required)")
	fset.StringVar(&cfg.branch, 0, "branch", "cover", "coverage branch name")
	fset.StringVar(&cfg.refName, 0, "ref-name", "", "current branch name (used to update head files on main)")
	cmd := &ff.Command{
		Name:      "push",
		Usage:     "go-beautiful-html-coverage push --revision <sha> [FLAGS]",
		ShortHelp: "generate coverage files and push them to the coverage branch",
		LongHelp: `Runs go tool cover to generate HTML and text coverage reports,
computes incremental coverage, injects shared assets, and pushes
everything to the coverage branch.

Expects cover.out to exist in the current directory (or in --path if
--path is a subdirectory relative to --workspace).`,
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

	workspace := cfg.workspace
	if workspace == "" {
		workspace = os.Getenv("GITHUB_WORKSPACE")
	}
	if workspace == "" {
		var err error
		workspace, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}
	}

	path := normalizepath.Normalize(cfg.path)

	// cover.out lives in {workspace}/{path} (or {workspace} if path is "").
	appDir := workspace
	if path != "" {
		appDir = filepath.Join(workspace, path)
	}

	coverDir := filepath.Join(workspace, "go-cover", path)

	return runPush(appDir, coverDir, cfg.revision, cfg.branch, cfg.refName)
}

func runPush(appDir, coverDir, revision, branch, refName string) error {
	revisionsDir := filepath.Join(coverDir, "revisions")
	if err := os.MkdirAll(revisionsDir, 0755); err != nil {
		return fmt.Errorf("create revisions dir: %w", err)
	}
	headDir := filepath.Join(coverDir, "head")
	if err := os.MkdirAll(headDir, 0755); err != nil {
		return fmt.Errorf("create head dir: %w", err)
	}

	coverOut := filepath.Join(appDir, "cover.out")

	// Generate HTML and text coverage reports.
	if err := goToolCover("-html="+coverOut, "-o", filepath.Join(revisionsDir, revision+".html")); err != nil {
		return fmt.Errorf("go tool cover html: %w", err)
	}
	if err := goToolCover("-func="+coverOut, "-o", filepath.Join(revisionsDir, revision+".txt")); err != nil {
		return fmt.Errorf("go tool cover func: %w", err)
	}
	if err := copyFile(coverOut, filepath.Join(revisionsDir, revision+".out")); err != nil {
		return fmt.Errorf("copy cover.out: %w", err)
	}

	// Compute incremental coverage (cover.out minus head.out).
	headOut := filepath.Join(headDir, "head.out")
	coverData, err := os.ReadFile(coverOut)
	if err != nil {
		return fmt.Errorf("read cover.out: %w", err)
	}
	headData, _ := os.ReadFile(headOut) // tolerate missing head.out on new branches
	incrementalData := computeIncremental(string(coverData), string(headData))

	incOut := filepath.Join(appDir, "incremental.out")
	if err := os.WriteFile(incOut, []byte(incrementalData), 0644); err != nil {
		return fmt.Errorf("write incremental.out: %w", err)
	}

	if err := goToolCover("-html="+incOut, "-o", filepath.Join(revisionsDir, revision+"-inc.html")); err != nil {
		return fmt.Errorf("go tool cover html (incremental): %w", err)
	}
	if err := goToolCover("-func="+incOut, "-o", filepath.Join(revisionsDir, revision+"-inc.txt")); err != nil {
		return fmt.Errorf("go tool cover func (incremental): %w", err)
	}
	if err := copyFile(incOut, filepath.Join(revisionsDir, revision+"-inc.out")); err != nil {
		return fmt.Errorf("copy incremental.out: %w", err)
	}

	// Write embedded static assets to the cover directory.
	if err := writeAssets(coverDir); err != nil {
		return fmt.Errorf("write assets: %w", err)
	}

	// Inject assets into the HTML files.
	if err := beautify.Run(coverDir, revision); err != nil {
		return fmt.Errorf("beautify: %w", err)
	}

	// If on the main branch, update the head files.
	if refName == "main" {
		for _, pair := range [][2]string{
			{filepath.Join(revisionsDir, revision+".html"), filepath.Join(headDir, "head.html")},
			{filepath.Join(revisionsDir, revision+".txt"), filepath.Join(headDir, "head.txt")},
			{filepath.Join(revisionsDir, revision+".out"), filepath.Join(headDir, "head.out")},
		} {
			if err := copyFile(pair[0], pair[1]); err != nil {
				return fmt.Errorf("update head file: %w", err)
			}
		}
	}

	// Git add / commit / push.
	if err := gitRun(coverDir, "add", "."); err != nil {
		return fmt.Errorf("git add: %w", err)
	}
	if err := gitRun(coverDir, "config", "user.email", "go-coverage-action@github.com"); err != nil {
		return fmt.Errorf("git config email: %w", err)
	}
	if err := gitRun(coverDir, "config", "user.name", "go-coverage-action"); err != nil {
		return fmt.Errorf("git config name: %w", err)
	}
	// Continue even if there is nothing new to commit.
	_ = gitRun(coverDir, "commit", "-m", "chore: add cover for "+revision)
	if err := gitRun(coverDir, "push", "origin", branch); err != nil {
		return fmt.Errorf("git push: %w", err)
	}

	return nil
}

// computeIncremental returns the set of lines present in coverOut but absent
// from headOut, prefixed with the "mode: set" header line.
func computeIncremental(coverOut, headOut string) string {
	headSet := make(map[string]struct{})
	for _, line := range strings.Split(strings.TrimRight(headOut, "\n"), "\n") {
		if line != "" {
			headSet[line] = struct{}{}
		}
	}

	var sb strings.Builder
	sb.WriteString("mode: set\n")
	for _, line := range strings.Split(strings.TrimRight(coverOut, "\n"), "\n") {
		if line == "" {
			continue
		}
		if _, inHead := headSet[line]; !inHead {
			sb.WriteString(line)
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}

func goToolCover(args ...string) error {
	all := append([]string{"tool", "cover"}, args...)
	cmd := exec.Command("go", all...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func gitRun(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

// writeAssets writes the embedded index.css, index.js, and index.html into dir.
func writeAssets(dir string) error {
	return fs.WalkDir(assets.FS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := assets.FS.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dir, path), data, 0644)
	})
}
