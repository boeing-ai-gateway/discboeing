package handler

import (
	"log"
	"sync"
	"time"

	"github.com/boeing-ai-gateway/discboeing/agent-go/filewatcher"
)

type workspaceFileNotifier struct {
	root string

	mu          sync.Mutex
	nextID      int
	subscribers map[int]chan struct{}
	watcher     *filewatcher.Watcher
	done        chan struct{}
}

func newWorkspaceFileNotifier(root string) *workspaceFileNotifier {
	return &workspaceFileNotifier{
		root:        root,
		subscribers: make(map[int]chan struct{}),
	}
}

func (n *workspaceFileNotifier) Subscribe() (<-chan struct{}, func()) {
	ch := make(chan struct{}, 1)

	n.mu.Lock()
	id := n.nextID
	n.nextID++
	n.subscribers[id] = ch
	if n.watcher == nil {
		n.startLocked()
	}
	n.mu.Unlock()

	return ch, func() {
		n.mu.Lock()
		delete(n.subscribers, id)
		shouldStop := len(n.subscribers) == 0
		watcher := n.watcher
		done := n.done
		if shouldStop {
			n.watcher = nil
			n.done = nil
		}
		n.mu.Unlock()

		if shouldStop && watcher != nil {
			if err := watcher.Close(); err != nil {
				log.Printf("session stream: workspace watcher shutdown: %v", err)
			}
			if done != nil {
				<-done
			}
		}
	}
}

func (n *workspaceFileNotifier) startLocked() {
	if n.root == "" {
		return
	}
	watcher, err := filewatcher.New(n.root, filewatcher.Options{
		IncludeInitial: false,
		ResyncInterval: 30 * time.Second,
	})
	if err != nil {
		log.Printf("session stream: failed to start workspace watcher: %v", err)
		return
	}

	done := make(chan struct{})
	n.watcher = watcher
	n.done = done
	go n.run(watcher, done)
}

func (n *workspaceFileNotifier) run(watcher *filewatcher.Watcher, done chan<- struct{}) {
	defer func() {
		n.mu.Lock()
		if n.watcher == watcher {
			n.watcher = nil
			n.done = nil
		}
		n.mu.Unlock()
		close(done)
	}()

	for {
		select {
		case _, ok := <-watcher.Events():
			if !ok {
				return
			}
			n.notify()
		case err, ok := <-watcher.Errors():
			if !ok {
				return
			}
			if err != nil {
				log.Printf("session stream: workspace watcher error: %v", err)
			}
			n.notify()
		}
	}
}

func (n *workspaceFileNotifier) notify() {
	n.mu.Lock()
	subscribers := make([]chan struct{}, 0, len(n.subscribers))
	for _, ch := range n.subscribers {
		subscribers = append(subscribers, ch)
	}
	n.mu.Unlock()

	for _, ch := range subscribers {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}
