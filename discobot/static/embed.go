// Package static embeds the Datastar UI assets for bundled use.
package static

import "embed"

// Files contains the built-in static assets served by the new Discobot UI.
//
//go:embed *.css *.js branding/*.svg lib/*.js
var Files embed.FS
