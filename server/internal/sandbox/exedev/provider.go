package exedev

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/obot-platform/discobot/server/internal/sandbox"
)

type commandClient interface {
	Exec(ctx context.Context, command string) ([]byte, error)
}

// Provider implements sandbox.Provider using exe.dev's HTTPS command API.
type Provider struct {
	cfg     Config
	timings timings

	client    commandClient
	httpCache *sandbox.HTTPClientCache

	listMu       sync.Mutex
	listCache    listCacheEntry
	listInFlight *listCall
}

// NewProvider creates an exe.dev provider backed by POST /exec.
func NewProvider(cfg Config) (*Provider, error) {
	cfg = cfg.withDefaults()
	if err := requireConfig(cfg); err != nil {
		return nil, err
	}
	return NewProviderWithClient(cfg, &httpCommandClient{endpoint: cfg.Endpoint, client: &http.Client{Timeout: 2 * time.Minute}, token: cfg.Token, timings: defaultTimings()})
}

func NewProviderWithClient(cfg Config, client commandClient) (*Provider, error) {
	if client == nil {
		return nil, fmt.Errorf("exe.dev command client is required")
	}
	t := defaultTimings()
	if hc, ok := client.(*httpCommandClient); ok {
		hc.timings = t
	}
	return &Provider{cfg: cfg.withDefaults(), timings: t, client: client, httpCache: sandbox.NewHTTPClientCache()}, nil
}

func (p *Provider) ImageExists(context.Context) bool       { return true }
func (p *Provider) Image() string                          { return p.cfg.SandboxImage }
func (p *Provider) IsLocal() bool                          { return false }
func (p *Provider) Definition() sandbox.ProviderDefinition { return Definition() }
func (p *Provider) Status() sandbox.ProviderStatus {
	status := sandbox.ProviderStatus{Available: true, State: "ready"}
	if p.cfg.Token == "" {
		status.Available = false
		status.State = "not_available"
		status.Message = "exe.dev token is not configured"
	}
	return status
}

func (p *Provider) Reconcile(context.Context) error             { return nil }
func (p *Provider) RemoveProject(context.Context, string) error { return nil }

var _ sandbox.Provider = (*Provider)(nil)
var _ sandbox.StatusProvider = (*Provider)(nil)
