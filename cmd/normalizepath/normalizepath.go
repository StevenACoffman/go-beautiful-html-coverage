// Package normalizepath implements the "normalize-path" CLI command and the
// Normalize helper used by other commands.
package normalizepath

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/peterbourgon/ff/v4"
	"github.com/StevenACoffman/go-beautiful-html-coverage/cmd/root"
)

// Config holds the configuration for the normalize-path command.
type Config struct {
	*root.Config
}

// New creates and registers the normalize-path command with the given parent config.
func New(parent *root.Config) *Config {
	cfg := &Config{Config: parent}
	fset := ff.NewFlagSet("normalize-path").SetParent(parent.Flags)
	cmd := &ff.Command{
		Name:      "normalize-path",
		Usage:     "go-beautiful-html-coverage normalize-path <path>",
		ShortHelp: "normalize a directory path for use as a coverage sub-path",
		LongHelp: `Normalizes a directory path by resolving . and .. components,
stripping leading ./ and /, and stripping trailing /.
Prints the result to stdout. Returns "" for root paths.`,
		Flags: fset,
		Exec:  cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cmd)
	return cfg
}

func (cfg *Config) exec(_ context.Context, args []string) error {
	dir := ""
	if len(args) > 0 {
		dir = args[0]
	}
	_, _ = fmt.Fprintln(cfg.Stdout, Normalize(dir))
	return nil
}

// Normalize cleans a directory path and strips leading/trailing separators so
// it can be used safely as a coverage sub-path.  Returns "" for root-equivalent
// inputs ("", ".", "/", "./").
func Normalize(dir string) string {
	dir = filepath.ToSlash(filepath.Clean(dir))
	if dir == "/" || dir == "." {
		return ""
	}
	dir = strings.TrimPrefix(dir, "/")
	dir = strings.TrimSuffix(dir, "/")
	return dir
}
