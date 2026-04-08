// Package comment implements the "comment" CLI command.
// It creates or updates a PR comment on GitHub with the coverage report.
package comment

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/peterbourgon/ff/v4"
	"github.com/StevenACoffman/go-beautiful-html-coverage/cmd/normalizepath"
	"github.com/StevenACoffman/go-beautiful-html-coverage/cmd/root"
)

// Config holds the configuration for the comment command.
type Config struct {
	*root.Config

	owner       string
	repo        string
	issueNumber int
	path        string
	revision    string
	threshold   float64
	token       string
}

// New creates and registers the comment command with the given parent config.
func New(parent *root.Config) *Config {
	cfg := &Config{Config: parent}
	fset := ff.NewFlagSet("comment").SetParent(parent.Flags)
	fset.StringVar(&cfg.owner, 0, "owner", "", "GitHub repository owner (required)")
	fset.StringVar(&cfg.repo, 0, "repo", "", "GitHub repository name (required)")
	fset.IntVar(&cfg.issueNumber, 0, "issue-number", 0, "pull request number (required)")
	fset.StringVar(&cfg.path, 0, "path", "", "normalized sub-path (default: repo root)")
	fset.StringVar(&cfg.revision, 0, "revision", "", "git revision SHA (required)")
	fset.Float64Var(&cfg.threshold, 0, "threshold", 0, "minimum coverage percentage")
	fset.StringVar(&cfg.token, 0, "token", "", "GitHub API token (required)")
	cmd := &ff.Command{
		Name:      "comment",
		Usage:     "go-beautiful-html-coverage comment --owner <owner> --repo <repo> --issue-number <n> --revision <sha> --token <tok>",
		ShortHelp: "create or update a PR coverage comment on GitHub",
		LongHelp: `Reads the coverage .txt file, builds a Markdown comment body, then
creates or updates the comment on the given pull request.  The comment
is identified by the marker <!-- coverage ({path})--> at the top.`,
		Flags: fset,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cmd)
	return cfg
}

func (cfg *Config) exec(_ context.Context, _ []string) error {
	if cfg.owner == "" {
		return fmt.Errorf("--owner is required")
	}
	if cfg.repo == "" {
		return fmt.Errorf("--repo is required")
	}
	if cfg.issueNumber == 0 {
		return fmt.Errorf("--issue-number is required")
	}
	if cfg.revision == "" {
		return fmt.Errorf("--revision is required")
	}
	if cfg.token == "" {
		return fmt.Errorf("--token is required")
	}

	path := normalizepath.Normalize(cfg.path)

	txtPath := filepath.Join("go-cover", path, "revisions", cfg.revision+".txt")
	data, err := os.ReadFile(txtPath)
	if err != nil {
		return fmt.Errorf("read coverage file %s: %w", txtPath, err)
	}

	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) == 0 {
		return fmt.Errorf("coverage file %s is empty", txtPath)
	}

	lastLine := lines[len(lines)-1]
	fields := strings.Split(lastLine, "\t")
	summary := strings.TrimSpace(fields[len(fields)-1])

	pctStr := strings.TrimSuffix(summary, "%")
	coverage, err := strconv.ParseFloat(pctStr, 64)
	if err != nil {
		return fmt.Errorf("parse coverage %q: %w", summary, err)
	}

	existingID, err := findComment(cfg.owner, cfg.repo, cfg.issueNumber, path, cfg.token)
	if err != nil {
		return fmt.Errorf("list comments: %w", err)
	}

	body := buildCommentBody(path, cfg.revision, cfg.owner, cfg.repo, summary, coverage, cfg.threshold, lines)

	if existingID != 0 {
		return updateComment(cfg.owner, cfg.repo, existingID, body, cfg.token)
	}
	return createComment(cfg.owner, cfg.repo, cfg.issueNumber, body, cfg.token)
}

type ghComment struct {
	ID   int    `json:"id"`
	Body string `json:"body"`
}

func findComment(owner, repo string, issueNumber int, path, token string) (int, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues/%d/comments?per_page=100",
		owner, repo, issueNumber)
	resp, err := ghRequest(http.MethodGet, url, token, nil)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("list comments: HTTP %d: %s", resp.StatusCode, body)
	}

	var comments []ghComment
	if err := json.NewDecoder(resp.Body).Decode(&comments); err != nil {
		return 0, fmt.Errorf("decode comments: %w", err)
	}

	marker := fmt.Sprintf("<!-- coverage (%s)-->", path)
	for _, c := range comments {
		if strings.HasPrefix(c.Body, marker) {
			return c.ID, nil
		}
	}
	return 0, nil
}

func createComment(owner, repo string, issueNumber int, body, token string) error {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues/%d/comments", owner, repo, issueNumber)
	payload, err := json.Marshal(map[string]string{"body": body})
	if err != nil {
		return fmt.Errorf("encode comment body: %w", err)
	}
	resp, err := ghRequest(http.MethodPost, url, token, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("create comment: HTTP %d: %s", resp.StatusCode, b)
	}
	return nil
}

func updateComment(owner, repo string, commentID int, body, token string) error {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues/comments/%d", owner, repo, commentID)
	payload, err := json.Marshal(map[string]string{"body": body})
	if err != nil {
		return fmt.Errorf("encode comment body: %w", err)
	}
	resp, err := ghRequest(http.MethodPatch, url, token, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("update comment: HTTP %d: %s", resp.StatusCode, b)
	}
	return nil
}

func ghRequest(method, url, token string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return http.DefaultClient.Do(req)
}

func buildCommentBody(path, revision, owner, repo, summary string, coverage, threshold float64, lines []string) string {
	marker := fmt.Sprintf("<!-- coverage (%s)-->", path)
	url := fmt.Sprintf("https://%s.github.io/%s/%s?hash=%s", owner, repo, path, revision)
	if path == "" {
		url = fmt.Sprintf("https://%s.github.io/%s?hash=%s", owner, repo, revision)
	}

	var emoji string
	if threshold > 0 && coverage < threshold {
		emoji = fmt.Sprintf("<kbd>🔻 %.1f%%</kbd> ", coverage-threshold)
	}

	pathText := ""
	if path != "" {
		pathText = fmt.Sprintf(" for <kbd>%s/</kbd>", path)
	}

	parts := []string{
		marker,
		fmt.Sprintf("##### %s<kbd>[🔗 Code Coverage Report](%s)</kbd>%s at <kbd>%s</kbd>", emoji, url, pathText, revision),
		"```",
		"📔 Total: " + summary,
	}

	if threshold > 0 {
		parts = append(parts, fmt.Sprintf("🎯 Threshold: %.1f%%", threshold))
		if coverage >= threshold {
			parts = append(parts, fmt.Sprintf("✅ %s >= %.1f%%", summary, threshold))
		} else {
			parts = append(parts, fmt.Sprintf("❌ %s < %.1f%%", summary, threshold))
		}
	}

	parts = append(parts,
		"```",
		"<details>",
		"<summary>Full coverage report</summary>",
		"",
		"```",
	)
	parts = append(parts, lines...)
	parts = append(parts,
		"```",
		"</details>",
		`<p align="right"><sup><a href="https://github.com/gha-common/go-beautiful-html-coverage">go-beautiful-html-coverage ↗</a></sup></p>`,
	)

	return strings.Join(parts, "\n")
}
