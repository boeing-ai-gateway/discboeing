package server

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/boeing-ai-gateway/discboeing/agent-go/message"
	"github.com/boeing-ai-gateway/discboeing/agent-go/portwatcher"
)

func TestWorkspacePortWatcherSerializesScans(t *testing.T) {
	firstEntered := make(chan struct{})
	releaseFirst := make(chan struct{})
	secondEntered := make(chan struct{})
	var calls atomic.Int32
	var concurrent atomic.Int32
	var maxConcurrent atomic.Int32
	var chunks atomic.Int32

	watcher := &workspacePortWatcher{
		emit: func(message.MessageChunk) {
			chunks.Add(1)
		},
		ports: make(map[string]portwatcher.Entry),
		scan: func(ctx context.Context) ([]portwatcher.Entry, error) {
			active := concurrent.Add(1)
			defer concurrent.Add(-1)
			for {
				currentMax := maxConcurrent.Load()
				if active <= currentMax || maxConcurrent.CompareAndSwap(currentMax, active) {
					break
				}
			}

			switch calls.Add(1) {
			case 1:
				close(firstEntered)
				select {
				case <-releaseFirst:
				case <-ctx.Done():
					return nil, ctx.Err()
				}
				return []portwatcher.Entry{{LocalAddress: "127.0.0.1:3000", Port: 3000, Process: "first", PID: 1, FD: 3}}, nil
			case 2:
				close(secondEntered)
				return []portwatcher.Entry{{LocalAddress: "127.0.0.1:4000", Port: 4000, Process: "second", PID: 2, FD: 4}}, nil
			default:
				t.Fatal("unexpected extra scan")
				return nil, nil
			}
		},
	}

	firstDone := make(chan struct{})
	go func() {
		defer close(firstDone)
		watcher.scanAndPublish("first", false)
	}()

	select {
	case <-firstEntered:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for first scan to start")
	}

	secondDone := make(chan struct{})
	go func() {
		defer close(secondDone)
		watcher.scanAndPublish("second", false)
	}()

	select {
	case <-secondEntered:
		t.Fatal("second scan started before first scan completed")
	case <-time.After(50 * time.Millisecond):
	}

	close(releaseFirst)

	select {
	case <-firstDone:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for first scan to finish")
	}
	select {
	case <-secondDone:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for second scan to finish")
	}

	if got := maxConcurrent.Load(); got != 1 {
		t.Fatalf("expected serialized scans, max concurrent scans was %d", got)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("expected 2 scans, got %d", got)
	}
	if got := chunks.Load(); got != 2 {
		t.Fatalf("expected 2 emitted chunks, got %d", got)
	}

	watcher.mu.Lock()
	defer watcher.mu.Unlock()
	if _, ok := watcher.ports[portKey(portwatcher.Entry{LocalAddress: "127.0.0.1:4000", Process: "second", PID: 2, FD: 4})]; !ok {
		t.Fatalf("expected final port state from second scan, got %#v", watcher.ports)
	}
}
