package dispatcher

import (
	"testing"

	"github.com/obot-platform/discobot/server/internal/jobs"
)

func TestGetConcurrencyLimit(t *testing.T) {
	if limit := GetConcurrencyLimit(jobs.JobTypeSessionCommit); limit != DefaultConcurrencyLimit {
		t.Fatalf("Expected commit concurrency limit %d, got %d", DefaultConcurrencyLimit, limit)
	}
}
