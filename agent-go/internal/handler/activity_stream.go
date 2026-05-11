package handler

import "sync"

type activityNotifier struct {
	mu          sync.Mutex
	nextID      int
	subscribers map[int]chan struct{}
}

func newActivityNotifier() *activityNotifier {
	return &activityNotifier{
		subscribers: make(map[int]chan struct{}),
	}
}

func (n *activityNotifier) Subscribe() (<-chan struct{}, func()) {
	ch := make(chan struct{}, 1)

	n.mu.Lock()
	id := n.nextID
	n.nextID++
	n.subscribers[id] = ch
	n.mu.Unlock()

	return ch, func() {
		n.mu.Lock()
		delete(n.subscribers, id)
		n.mu.Unlock()
	}
}

func (n *activityNotifier) Notify() {
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
