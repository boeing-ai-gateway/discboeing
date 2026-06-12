//go:build e2e_mock_llm

package main

import (
	// Build-tagged side-effect import for deterministic e2e LLM responses.
	_ "github.com/obot-platform/discobot/agent-go/providers/e2emockllm"
)
