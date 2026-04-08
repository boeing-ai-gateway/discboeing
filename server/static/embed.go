package static

import "embed"

//go:embed api-ui.html all:ui
var Files embed.FS
