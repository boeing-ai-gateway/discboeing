//go:build e2e_mock_llm

// Package llmresponses embeds deterministic mock LLM fixture JSON files for
// e2e-tagged agent-go builds only.
package llmresponses

import "embed"

// FS contains all *.json files in this directory.
//
//go:embed *.json
var FS embed.FS
