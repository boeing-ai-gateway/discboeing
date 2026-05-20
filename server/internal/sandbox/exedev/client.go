package exedev

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type httpCommandClient struct {
	endpoint string
	token    string
	client   *http.Client
	timings  timings
}

func (c *httpCommandClient) Exec(ctx context.Context, command string) ([]byte, error) {
	ctx, cancel := contextWithDefaultTimeout(ctx, c.timings.rateLimitRetryTimeout)
	defer cancel()

	sanitizedCommand := sanitizeCommandForLog(command)
	for {
		log.Printf("Running exe.dev command: %s", sanitizedCommand)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, strings.NewReader(command))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "text/plain")
		req.Header.Set("Authorization", "Bearer "+c.token)
		resp, err := c.client.Do(req)
		if err != nil {
			return nil, err
		}
		body, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			return nil, readErr
		}
		if resp.StatusCode == http.StatusTooManyRequests {
			delay := retryAfterDelay(resp.Header.Get("Retry-After"), c.timings.rateLimitRetryDelay)
			log.Printf("exe.dev command rate limited; retrying in %s: %s", delay, sanitizedCommand)
			if err := sleepContext(ctx, delay); err != nil {
				return nil, err
			}
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, commandError{
				statusCode: resp.StatusCode,
				body:       strings.TrimSpace(string(body)),
			}
		}
		return body, nil
	}
}

type commandError struct {
	statusCode int
	body       string
}

func (e commandError) Error() string {
	return fmt.Sprintf("exe.dev command failed with status %d: %s", e.statusCode, e.body)
}

func retryAfterDelay(value string, fallback time.Duration) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	if seconds, err := strconv.Atoi(value); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	if retryAt, err := http.ParseTime(value); err == nil {
		if delay := time.Until(retryAt); delay > 0 {
			return delay
		}
		return 0
	}
	return fallback
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
