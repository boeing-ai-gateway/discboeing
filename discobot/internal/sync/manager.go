// Package sync mirrors live Discobot server state into renderable UI state.
package sync

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	serverapi "github.com/obot-platform/discobot/server/api"
	serviceclient "github.com/obot-platform/discobot/server/client"

	"github.com/obot-platform/discobot/discobot/internal/state"
)

const retryDelay = time.Second

// Store receives consistent cache swaps and incremental cache updates.
type Store interface {
	SaveData(func(*state.Data))
}

// Manager keeps state.Data synchronized with the running Discobot API server.
type Manager struct {
	client       *serviceclient.Client
	streamClient *serviceclient.Client
	store        Store
	logger       *slog.Logger
}

// NewManager creates a data sync manager for serverBaseURL.
func NewManager(serverBaseURL string, store Store, logger *slog.Logger) (*Manager, error) {
	client, err := serviceclient.NewClient(serverBaseURL)
	if err != nil {
		return nil, err
	}
	return &Manager{client: client, streamClient: client, store: store, logger: logger}, nil
}

// Run starts the sync loop and returns when ctx is canceled.
func (m *Manager) Run(ctx context.Context) {
	for ctx.Err() == nil {
		if err := m.syncOnce(ctx); err != nil && ctx.Err() == nil {
			m.logger.Warn("discobot data sync failed; retrying", "error", err)
			select {
			case <-ctx.Done():
			case <-time.After(retryDelay):
			}
		}
	}
}

type projectRuntime struct {
	project      serverapi.Project
	subscription *serviceclient.ProjectSubscription
	events       <-chan serviceclient.ProjectStreamEvent
	cache        state.ProjectData
	liveThreads  map[serviceclient.ProjectThreadSubscription]bool
}

func (m *Manager) syncOnce(ctx context.Context) error {
	projects, err := m.listProjects(ctx)
	if err != nil {
		return err
	}

	cycleCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	m.publishProjectList(projects)

	errs := make(chan error, len(projects))
	for _, project := range projects {
		go func() {
			errs <- m.syncProject(cycleCtx, project)
		}()
	}

	if len(projects) == 0 {
		<-cycleCtx.Done()
		return cycleCtx.Err()
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errs:
		return err
	}
}

func (m *Manager) syncProject(ctx context.Context, project serverapi.Project) error {
	runtime, err := m.startProjectRuntime(ctx, project)
	if err != nil {
		return err
	}
	defer func() { _ = runtime.subscription.Close() }()

	processor := newProjectEventProcessor(m, &runtime)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event, ok := <-runtime.events:
			if !ok {
				return errors.New("project event stream closed")
			}
			if err := processor.Process(ctx, event); err != nil {
				return err
			}
		}
	}
}

func (m *Manager) startProjectRuntime(ctx context.Context, project serverapi.Project) (projectRuntime, error) {
	subscription, err := m.streamClient.SubscribeProject(ctx, project.ID, serviceclient.ProjectSubscriptionOptions{
		ProjectEvents: serverapi.ProjectEventsSubscriptionOptions{},
	})
	if err != nil {
		return projectRuntime{}, fmt.Errorf("subscribe project %s: %w", project.ID, err)
	}

	return projectRuntime{
		project:      project,
		subscription: subscription,
		events:       subscription.Events(),
		cache: state.ProjectData{
			Project:   project,
			Workspace: map[string]serverapi.Workspace{},
			Session:   map[string]state.SessionData{},
		},
		liveThreads: map[serviceclient.ProjectThreadSubscription]bool{},
	}, nil
}
