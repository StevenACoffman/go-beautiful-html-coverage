// Package root defines the root configuration for the CLI.
package root

import (
	"io"

	"github.com/peterbourgon/ff/v4"
)

// Config holds shared I/O writers and the root ff.Command.
// All subcommand configs embed *Config to inherit these.
type Config struct {
	Stdin   io.Reader
	Stdout  io.Writer
	Stderr  io.Writer
	Flags   *ff.FlagSet
	Command *ff.Command
}

// New returns a new root Config with the given I/O writers.
func New(stdin io.Reader, stdout, stderr io.Writer) *Config {
	var cfg Config
	cfg.Stdin = stdin
	cfg.Stdout = stdout
	cfg.Stderr = stderr
	// No shared flags — cfg.Flags is nil; ff provides --help automatically.
	// To add shared flags, uncomment and bind before constructing the command:
	// cfg.Flags = ff.NewFlagSet("go-beautiful-html-coverage")
	// cfg.Flags.BoolVar(&cfg.MyFlag, 0, "my-flag", "", "description")
	cfg.Command = &ff.Command{
		Name:      "go-beautiful-html-coverage",
		Usage:     "go-beautiful-html-coverage <SUBCOMMAND> ...",
		ShortHelp: "TODO: describe go-beautiful-html-coverage here",
	}
	return &cfg
}
