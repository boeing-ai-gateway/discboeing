package static

import "embed"

//go:embed api-ui.html scalar-ui.html all:ui
var Files embed.FS
