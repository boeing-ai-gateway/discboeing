//go:build e2e_mock_llm

// Package e2emockllm is compiled only with -tags=e2e_mock_llm. It registers
// provider ID "e2e-mock-llm" and loads JSON response mappings from the embedded
// agent-go/llm-responses directory or DISCOBOT_E2E_MOCK_LLM_RESPONSES_DIR.
package e2emockllm
