// Package frontend embeds the prebuilt, offline UI. Building Go never needs Node.
package frontend

import (
	"embed"
	"io/fs"
)

//go:embed dist
var content embed.FS

func Files() fs.FS {
	f, e := fs.Sub(content, "dist")
	if e != nil {
		panic(e)
	}
	return f
}
