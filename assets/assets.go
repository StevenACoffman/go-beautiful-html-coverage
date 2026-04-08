// Package assets embeds the static web assets served alongside coverage reports.
package assets

import "embed"

//go:embed index.css index.js index.html
var FS embed.FS
