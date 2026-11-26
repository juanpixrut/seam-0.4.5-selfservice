// Package web embeds the static web assets (CSS, JS, images, icons) into the
// compiled binary so that Seam AMS can be deployed as a single executable
// without an external web/static directory.
//
// Served at /static/ by the HTTP server via http.FS(Static).
package web

import "embed"

// Static holds the embedded web/static directory tree.
//
//go:embed static
var Static embed.FS
