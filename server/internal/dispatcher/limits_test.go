package dispatcher

import (
	"testing"

	"github.com/boeing-ai-gateway/discboeing/server/internal/jobs"
)

func TestGetConcurrencyLimit(t *testing.T) {
	if limit := GetConcurrencyLimit(jobs.JobTypeSessionCommit); limit != DefaultConcurrencyLimit {
		t.Fatalf("Expected commit concurrency limit %d, got %d", DefaultConcurrencyLimit, limit)
	}
}
