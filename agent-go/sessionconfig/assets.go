package sessionconfig

import "embed"

//go:embed SYSTEM.md tool-*.yaml agent-*.md
var embeddedConfigFiles embed.FS
