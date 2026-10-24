// Package web holds the browser UI. It is plain HTML, CSS and JavaScript, embedded in
// the server binary.
package web

import (
	"embed"
	"io/fs"
)

//go:embed static
var files embed.FS // why: read-only embedded data, the allowed kind of package-level value

// FS returns the UI's files rooted at the static directory.
func FS() (fs.FS, error) { return fs.Sub(files, "static") }
