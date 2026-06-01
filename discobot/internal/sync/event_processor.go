package sync

import (
	"context"
	"fmt"

	serviceclient "github.com/obot-platform/discobot/server/client"
)

type projectEventProcessor struct {
	manager *Manager
	runtime *projectRuntime
}

func newProjectEventProcessor(manager *Manager, runtime *projectRuntime) projectEventProcessor {
	return projectEventProcessor{manager: manager, runtime: runtime}
}

func (p projectEventProcessor) Process(ctx context.Context, event serviceclient.ProjectStreamEvent) error {
	handled, err := p.processSubscriptionEvent(ctx, event)
	if handled || err != nil {
		return err
	}
	if p.processThreadEvent(ctx, event) {
		return nil
	}
	return p.processProjectEvent(ctx, event)
}

func (p projectEventProcessor) processSubscriptionEvent(ctx context.Context, event serviceclient.ProjectStreamEvent) (bool, error) {
	switch event.(type) {
	case serviceclient.ProjectSubscriptionStartedEvent:
		return true, p.rebuildProjectCache(ctx)
	case serviceclient.ProjectThreadSubscriptionStartedEvent:
		return true, nil
	default:
		return false, nil
	}
}

func (p projectEventProcessor) rebuildProjectCache(ctx context.Context) error {
	cache, err := p.manager.buildProjectCache(ctx, p.runtime.project)
	if err != nil {
		return fmt.Errorf("build project %s cache: %w", p.runtime.project.ID, err)
	}
	p.runtime.cache = cache
	p.manager.publishProject(ctx, p.runtime.project, p.runtime.cache)
	return p.manager.subscribeProjectThreads(ctx, p.runtime)
}
