// Package version implements the "version" CLI command.
package version

import (
	"context"
	"fmt"

	"github.com/peterbourgon/ff/v4"
	"github.com/StevenACoffman/go-beautiful-html-coverage/cmd/root"
)

// Version is the application version string.
// Override at build time: go build -ldflags "-X 'github.com/StevenACoffman/go-beautiful-html-coverage/cmd/version.Version=1.2.3'"
var Version = "dev"

// Config holds the configuration for the version command.
type Config struct {
	*root.Config
}

// New creates and registers the version command with the given parent config.
func New(parent *root.Config) *Config {
	cfg := &Config{Config: parent}
	fset := ff.NewFlagSet("version").SetParent(parent.Flags)
	cmd := &ff.Command{
		Name:      "version",
		Usage:     "go-beautiful-html-coverage version",
		ShortHelp: "print version information",
		LongHelp:  "Prints version information for the application.",
		Flags:     fset,
		Exec:      cfg.exec,
	}
	parent.Command.Subcommands = append(parent.Command.Subcommands, cmd)
	return cfg
}

func (cfg *Config) exec(_ context.Context, _ []string) error {
	_, _ = fmt.Fprintln(cfg.Stdout, "version "+Version)
	return nil
}
