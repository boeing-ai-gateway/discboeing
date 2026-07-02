package server

import (
	"context"
	"encoding/json"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/boeing-ai-gateway/discboeing/agent-go/message"
	"github.com/boeing-ai-gateway/discboeing/agent-go/portwatcher"
)

const workspacePortsDataType = "workspace-ports"

type workspacePortEvent struct {
	Reason    string              `json:"reason"`
	Ports     []portwatcher.Entry `json:"ports"`
	Added     []portwatcher.Entry `json:"added,omitempty"`
	Removed   []portwatcher.Entry `json:"removed,omitempty"`
	Resync    bool                `json:"resync,omitempty"`
	Error     *workspacePortError `json:"error,omitempty"`
	ScannedAt time.Time           `json:"scannedAt"`
}

type workspacePortError struct {
	Message string `json:"message"`
}

type workspacePortWatcher struct {
	emit func(message.MessageChunk)
	scan func(context.Context) ([]portwatcher.Entry, error)

	scanMu sync.Mutex
	mu     sync.Mutex
	ports  map[string]portwatcher.Entry

	stop chan struct{}
	done chan struct{}
}

func startWorkspacePortWatcher(emit func(message.MessageChunk)) *workspacePortWatcher {
	w := &workspacePortWatcher{
		emit:  emit,
		scan:  portwatcher.Scan,
		ports: make(map[string]portwatcher.Entry),
		stop:  make(chan struct{}),
		done:  make(chan struct{}),
	}
	go w.run()
	w.scanAndPublish("initial", true)
	return w
}

func (w *workspacePortWatcher) Close() {
	select {
	case <-w.done:
		return
	default:
	}
	select {
	case <-w.stop:
	default:
		close(w.stop)
	}
	<-w.done
}

func (w *workspacePortWatcher) OnTurnStart(string) {
	w.scanAndPublish("turn-start", false)
}

func (w *workspacePortWatcher) OnTurnComplete(string, error) {
	w.scanAndPublish("turn-complete", false)
}

func (w *workspacePortWatcher) run() {
	defer close(w.done)
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			w.scanAndPublish("interval", true)
		case <-w.stop:
			return
		}
	}
}

func (w *workspacePortWatcher) scanAndPublish(reason string, resync bool) {
	w.scanMu.Lock()
	defer w.scanMu.Unlock()

	scan := w.scan
	if scan == nil {
		scan = portwatcher.Scan
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	ports, err := scan(ctx)
	cancel()

	w.mu.Lock()
	previous := w.ports
	if err == nil {
		w.ports = portMap(ports)
	}
	w.mu.Unlock()

	if err != nil {
		log.Printf("workspace port watcher: %v", err)
		emitWorkspacePortEvent(w.emit, workspacePortEvent{
			Reason:    reason,
			Error:     &workspacePortError{Message: err.Error()},
			ScannedAt: time.Now().UTC(),
		})
		return
	}

	current := portMap(ports)
	added, removed := diffPorts(previous, current)
	if len(added) == 0 && len(removed) == 0 && !resync {
		return
	}
	emitWorkspacePortEvent(w.emit, workspacePortEvent{
		Reason:    reason,
		Ports:     ports,
		Added:     added,
		Removed:   removed,
		Resync:    resync,
		ScannedAt: time.Now().UTC(),
	})
}

func emitWorkspacePortEvent(emit func(message.MessageChunk), event workspacePortEvent) {
	if emit == nil {
		return
	}
	data, err := json.Marshal(event)
	if err != nil {
		log.Printf("workspace port watcher: marshal event: %v", err)
		return
	}
	emit(message.DataChunk{
		DataType: workspacePortsDataType,
		Data:     data,
	})
}

func portMap(ports []portwatcher.Entry) map[string]portwatcher.Entry {
	mapped := make(map[string]portwatcher.Entry, len(ports))
	for _, port := range ports {
		mapped[portKey(port)] = port
	}
	return mapped
}

func diffPorts(previous, current map[string]portwatcher.Entry) ([]portwatcher.Entry, []portwatcher.Entry) {
	var added, removed []portwatcher.Entry
	for key, entry := range current {
		if _, ok := previous[key]; !ok {
			added = append(added, entry)
		}
	}
	for key, entry := range previous {
		if _, ok := current[key]; !ok {
			removed = append(removed, entry)
		}
	}
	return added, removed
}

func portKey(entry portwatcher.Entry) string {
	return entry.LocalAddress + "|" + entry.Process + "|" + strconv.Itoa(entry.PID) + "|" + strconv.Itoa(entry.FD)
}
