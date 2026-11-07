// Package webui embeds the management console assets.
package webui

import (
	"embed"
	"io/fs"
)

// Files contains the built React/Material UI bundle.
//
//go:embed dist
var Files embed.FS

// Root returns the embedded bundle rooted at the Vite dist directory.
func Root() fs.FS {
	sub, err := fs.Sub(Files, "dist")
	if err != nil {
		panic(err)
	}
	return sub
}
