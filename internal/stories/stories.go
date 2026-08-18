// Package stories embeds the bundled story JSON files.
package stories

import "embed"

// FS holds the embedded story JSON files. The path is relative to this file
// (internal/stories), so the assets live in internal/stories/stories/.
//
//go:embed all:stories
var FS embed.FS
