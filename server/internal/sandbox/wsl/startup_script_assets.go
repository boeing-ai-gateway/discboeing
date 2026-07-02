//go:build windows

package wsl

import _ "embed"

//go:embed assets/discboeing-wsl-startup.ps1
var embeddedWSLStartupScript []byte
