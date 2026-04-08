// Package cmd is the dispatcher; it routes CLI arguments to the matching command.
package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffhelp"
	"github.com/StevenACoffman/go-beautiful-html-coverage/cmd/beautify"
	"github.com/StevenACoffman/go-beautiful-html-coverage/cmd/checkthreshold"
	"github.com/StevenACoffman/go-beautiful-html-coverage/cmd/comment"
	"github.com/StevenACoffman/go-beautiful-html-coverage/cmd/normalizepath"
	"github.com/StevenACoffman/go-beautiful-html-coverage/cmd/pull"
	"github.com/StevenACoffman/go-beautiful-html-coverage/cmd/push"
	"github.com/StevenACoffman/go-beautiful-html-coverage/cmd/root"
	"github.com/StevenACoffman/go-beautiful-html-coverage/cmd/version"
)

// Run parses args and dispatches to the matching command.
// args must not include the executable name (pass os.Args[1:]).
func Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	r := root.New(stdin, stdout, stderr)
	version.New(r)
	normalizepath.New(r)
	beautify.New(r)
	checkthreshold.New(r)
	comment.New(r)
	push.New(r)
	pull.New(r)

	if err := r.Command.Parse(args); err != nil {
		fmt.Fprintf(stderr, "\n%s\n", ffhelp.Command(r.Command))
		return fmt.Errorf("parse: %w", err)
	}

	if err := r.Command.Run(ctx); err != nil {
		if !errors.Is(err, ff.ErrNoExec) {
			fmt.Fprintf(stderr, "\n%s\n", ffhelp.Command(r.Command.GetSelected()))
		}
		return err
	}

	return nil
}
