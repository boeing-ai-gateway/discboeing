package static

import "embed"

// Files contains Meta static assets.
//
// api-ui.html is intentionally copied from server/static/api-ui.html so the Meta
// service uses the same route explorer UI as the main server.
//
//go:embed api-ui.html
var Files embed.FS
