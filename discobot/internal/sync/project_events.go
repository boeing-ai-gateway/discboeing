package sync

import (
	"context"
	"errors"

	serverapi "github.com/obot-platform/discobot/server/api"
	serviceclient "github.com/obot-platform/discobot/server/client"

	"github.com/obot-platform/discobot/discobot/internal/state"
)

func (p projectEventProcessor) processProjectEvent(ctx context.Context, event serviceclient.ProjectStreamEvent) error {
	cache := state.CloneProjectData(p.runtime.cache)
	changed, err := p.applyProjectEventToCache(ctx, &cache, event)
	if err != nil || !changed {
		return err
	}
	p.runtime.cache = cache
	p.manager.publishProject(p.runtime.project, p.runtime.cache)
	return p.manager.subscribeProjectThreads(ctx, p.runtime)
}

func (p projectEventProcessor) applyProjectEventToCache(ctx context.Context, cache *state.ProjectData, event serviceclient.ProjectStreamEvent) (bool, error) {
	switch event := event.(type) {
	case serverapi.ProjectStreamSubscribedEvent, serverapi.ProjectConnectedEvent, serviceclient.ProjectSubscriptionStartedEvent, serviceclient.ProjectThreadSubscriptionStartedEvent:
		return false, nil
	case serverapi.ProjectStreamErrorEvent:
		return false, errors.New(event.Error)
	case serverapi.SessionUpdatedEvent:
		return p.applySessionUpdated(ctx, cache, event)
	case serverapi.ThreadUpdatedEvent:
		return p.applyThreadUpdated(ctx, cache, event)
	case serverapi.WorkspaceUpdatedEvent:
		return p.applyWorkspaceUpdated(ctx, cache)
	case serverapi.StartupTaskUpdatedEvent, serverapi.UnknownProjectEvent:
	}
	return false, nil
}

func (p projectEventProcessor) applyWorkspaceUpdated(ctx context.Context, cache *state.ProjectData) (bool, error) {
	return p.manager.refreshWorkspaces(ctx, cache)
}

func (p projectEventProcessor) applySessionUpdated(ctx context.Context, cache *state.ProjectData, event serverapi.SessionUpdatedEvent) (bool, error) {
	session, err := p.manager.getSession(ctx, cache.Project.ID, event.Data.SessionID)
	if err != nil {
		return false, err
	}
	upsertSession(cache, session)
	if !sessionSandboxRunning(session) {
		clearSessionThreads(cache, session.ID)
		return true, nil
	}
	return p.manager.refreshThreads(ctx, cache, session.ID)
}

func (p projectEventProcessor) applyThreadUpdated(ctx context.Context, cache *state.ProjectData, event serverapi.ThreadUpdatedEvent) (bool, error) {
	if event.Data.SessionID == "" {
		return false, nil
	}
	return p.manager.refreshThreads(ctx, cache, event.Data.SessionID)
}
