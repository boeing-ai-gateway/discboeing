package sandbox

import (
	"context"
	"sync"
	"testing"
	"time"
)

type testIdleRuntimeController struct {
	mu           sync.Mutex
	runtimeIDs   []string
	runningCount map[string]int
	stopped      []string
}

func (c *testIdleRuntimeController) ListRuntimeIDs(context.Context) ([]string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.runtimeIDs...), nil
}

func (c *testIdleRuntimeController) RunningSandboxCount(_ context.Context, runtimeID string) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.runningCount[runtimeID], nil
}

func (c *testIdleRuntimeController) StopRuntime(_ context.Context, runtimeID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stopped = append(c.stopped, runtimeID)
	return nil
}

func TestIdleRuntimeMonitorStopsRuntimeAfterTimeout(t *testing.T) {
	controller := &testIdleRuntimeController{
		runtimeIDs:   []string{"runtime-1"},
		runningCount: map[string]int{"runtime-1": 0},
	}

	monitor := NewIdleRuntimeMonitor(controller, "test", 20*time.Millisecond, 5*time.Millisecond)
	ctx := t.Context()

	monitor.Start(ctx)
	defer func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), time.Second)
		defer stopCancel()
		_ = monitor.Stop(stopCtx)
	}()

	deadline := time.Now().Add(time.Second)
	for {
		controller.mu.Lock()
		stopped := len(controller.stopped)
		controller.mu.Unlock()
		if stopped > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("runtime was not stopped before deadline")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestIdleRuntimeMonitorDoesNotStopActiveRuntime(t *testing.T) {
	controller := &testIdleRuntimeController{
		runtimeIDs:   []string{"runtime-1"},
		runningCount: map[string]int{"runtime-1": 1},
	}

	monitor := NewIdleRuntimeMonitor(controller, "test", 20*time.Millisecond, 5*time.Millisecond)
	ctx := t.Context()

	monitor.Start(ctx)
	defer func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), time.Second)
		defer stopCancel()
		_ = monitor.Stop(stopCtx)
	}()

	time.Sleep(60 * time.Millisecond)

	controller.mu.Lock()
	defer controller.mu.Unlock()
	if len(controller.stopped) != 0 {
		t.Fatalf("stopped runtimes = %v, want none", controller.stopped)
	}
}
