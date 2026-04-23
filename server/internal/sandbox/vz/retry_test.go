package vz

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestWaitForRetry_CompletesAfterDelay(t *testing.T) {
	ctx := context.Background()
	start := time.Now()

	if err := waitForRetry(ctx, 20*time.Millisecond); err != nil {
		t.Fatalf("waitForRetry returned error: %v", err)
	}

	if elapsed := time.Since(start); elapsed < 15*time.Millisecond {
		t.Fatalf("waitForRetry returned too early: %v", elapsed)
	}
}

func TestWaitForRetry_StopsOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	err := waitForRetry(ctx, time.Second)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("waitForRetry error = %v, want %v", err, context.Canceled)
	}

	if elapsed := time.Since(start); elapsed > 200*time.Millisecond {
		t.Fatalf("waitForRetry did not stop promptly: %v", elapsed)
	}
}
