package sandbox

import (
	"context"
	"fmt"
	"net/http"
	"sync"
)

// HTTPClientLease holds a sandbox HTTP client until Release is called.
// Callers must always release the lease after they finish using the client.
type HTTPClientLease struct {
	Client  *http.Client
	release func()
	once    sync.Once
}

// Release returns the leased client to its cache.
func (l *HTTPClientLease) Release() {
	if l == nil {
		return
	}
	l.once.Do(func() {
		if l.release != nil {
			l.release()
		}
	})
}

func newHTTPClientLease(client *http.Client, release func()) *HTTPClientLease {
	return &HTTPClientLease{Client: client, release: release}
}

// AcquireHTTPClient acquires a leased HTTP client from the provider.
func AcquireHTTPClient(ctx context.Context, provider Provider, state []byte, sessionID string) (*HTTPClientLease, error) {
	return provider.AcquireHTTPClient(ctx, state, sessionID)
}

// HTTPClientCache caches leased HTTP clients keyed by a unique session identifier.
// Entries can be removed while still in use; they are only closed after the last
// lease is released.
type HTTPClientCache struct {
	mu      sync.Mutex
	entries map[string]*httpClientCacheEntry
}

type httpClientCacheEntry struct {
	client *http.Client
	target string
	refs   int
	stale  bool
	closer func(*http.Client)
}

// NewHTTPClientCache creates an empty HTTP client cache.
func NewHTTPClientCache() *HTTPClientCache {
	return &HTTPClientCache{entries: make(map[string]*httpClientCacheEntry)}
}

// Acquire returns a leased HTTP client for key and target, creating it when needed.
// If the target changes, the previous client is marked stale and will be closed once
// outstanding leases release it.
func (c *HTTPClientCache) Acquire(key, target string, factory func() (*http.Client, error)) (*HTTPClientLease, error) {
	if c == nil {
		return nil, fmt.Errorf("http client cache is nil")
	}

	c.mu.Lock()
	if entry, ok := c.entries[key]; ok && !entry.stale && entry.target == target {
		entry.refs++
		client := entry.client
		c.mu.Unlock()
		return newHTTPClientLease(client, func() { c.release(entry) }), nil
	} else if ok {
		entry.stale = true
		if entry.refs == 0 {
			delete(c.entries, key)
			c.mu.Unlock()
			closeHTTPClient(entry)
		} else {
			delete(c.entries, key)
			c.mu.Unlock()
		}
	} else {
		c.mu.Unlock()
	}

	client, err := factory()
	if err != nil {
		return nil, err
	}

	entry := &httpClientCacheEntry{
		client: client,
		target: target,
		refs:   1,
		closer: func(client *http.Client) {
			if transport, ok := client.Transport.(*http.Transport); ok {
				transport.CloseIdleConnections()
			}
		},
	}

	c.mu.Lock()
	if existing, ok := c.entries[key]; ok && !existing.stale && existing.target == target {
		existing.refs++
		c.mu.Unlock()
		closeHTTPClient(entry)
		return newHTTPClientLease(existing.client, func() { c.release(existing) }), nil
	}
	c.entries[key] = entry
	c.mu.Unlock()

	return newHTTPClientLease(client, func() { c.release(entry) }), nil
}

// Remove marks the cached client for key stale and removes it from future acquisitions.
// If the client is idle, its idle connections are closed immediately.
func (c *HTTPClientCache) Remove(key string) {
	if c == nil {
		return
	}

	c.mu.Lock()
	entry, ok := c.entries[key]
	if !ok {
		c.mu.Unlock()
		return
	}
	entry.stale = true
	delete(c.entries, key)
	if entry.refs == 0 {
		c.mu.Unlock()
		closeHTTPClient(entry)
		return
	}
	c.mu.Unlock()
}

func (c *HTTPClientCache) release(entry *httpClientCacheEntry) {
	if c == nil || entry == nil {
		return
	}

	c.mu.Lock()
	if entry.refs > 0 {
		entry.refs--
	}
	shouldClose := entry.refs == 0 && entry.stale
	c.mu.Unlock()

	if shouldClose {
		closeHTTPClient(entry)
	}
}

func closeHTTPClient(entry *httpClientCacheEntry) {
	if entry == nil || entry.client == nil {
		return
	}
	if entry.closer != nil {
		entry.closer(entry.client)
		return
	}
	if transport, ok := entry.client.Transport.(*http.Transport); ok {
		transport.CloseIdleConnections()
	}
}
