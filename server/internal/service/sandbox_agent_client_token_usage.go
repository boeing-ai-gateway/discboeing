package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/obot-platform/discobot/server/internal/sandbox/sandboxapi"
)

// GetThreadTokenUsage retrieves detailed token usage for a thread from the
// sandbox agent.
func (c *SandboxAgentClient) GetThreadTokenUsage(
	ctx context.Context,
	sessionID, threadID string,
) (*sandboxapi.ThreadTokenUsageDetails, error) {
	resp, err := retryWithBackoff(ctx, func() (*http.Response, int, error) {
		lease, err := c.acquireHTTPClient(ctx, sessionID)
		if err != nil {
			return nil, 0, err
		}
		defer lease.Release()
		client := lease.Client

		req, err := http.NewRequestWithContext(
			ctx,
			http.MethodGet,
			c.threadURL(threadID, "/token-usage"),
			nil,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to create request: %w", err)
		}
		if err := c.applyRequestAuth(ctx, req, sessionID, nil); err != nil {
			return nil, 0, err
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, 0, err
		}
		return resp, resp.StatusCode, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get thread token usage: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("sandbox returned status %d: %s", resp.StatusCode, string(body))
	}

	var result sandboxapi.ThreadTokenUsageDetails
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}
