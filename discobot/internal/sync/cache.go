package sync

import (
	"context"
	"fmt"

	serverapi "github.com/obot-platform/discobot/server/api"
	serviceclient "github.com/obot-platform/discobot/server/client"

	"github.com/obot-platform/discobot/discobot/internal/state"
)

func (m *Manager) buildProjectCache(ctx context.Context, project serverapi.Project) (state.ProjectData, error) {
	cache := state.ProjectData{
		Project:   project,
		Workspace: map[string]serverapi.Workspace{},
		Session:   map[string]state.SessionData{},
	}

	workspaces, err := m.listWorkspaces(ctx, project.ID)
	if err != nil {
		return cache, err
	}
	cache.Workspaces = workspaces
	for _, workspace := range workspaces {
		cache.Workspace[workspace.ID] = workspace
	}

	models, err := m.listModels(ctx, project.ID)
	if err != nil {
		return cache, err
	}
	cache.Models = models

	sessions, err := m.listSessions(ctx, project.ID)
	if err != nil {
		return cache, err
	}

	cache.Sessions = sessions
	for _, session := range sessions {
		sessionData := state.SessionData{Session: session, Thread: map[string]state.ThreadData{}}
		if !sessionSandboxRunning(session) {
			cache.Session[session.ID] = sessionData
			continue
		}
		threads, err := m.listThreads(ctx, project.ID, session.ID)
		if err != nil {
			return cache, err
		}
		sessionData.Threads = threads
		for _, thread := range threads {
			sessionData.Thread[thread.ID] = state.ThreadData{Thread: thread}
		}
		cache.Session[session.ID] = sessionData
	}
	return cache, nil
}

func (m *Manager) refreshWorkspaces(ctx context.Context, cache *state.ProjectData) (bool, error) {
	workspaces, err := m.listWorkspaces(ctx, cache.Project.ID)
	if err != nil {
		return false, err
	}
	cache.Workspaces = workspaces
	cache.Workspace = map[string]serverapi.Workspace{}
	for _, workspace := range workspaces {
		cache.Workspace[workspace.ID] = workspace
	}
	return true, nil
}

func (m *Manager) refreshThreads(ctx context.Context, cache *state.ProjectData, sessionID string) (bool, error) {
	sessionData := ensureSessionData(cache, sessionID)
	if !sessionSandboxRunning(sessionData.Session) {
		clearSessionThreads(cache, sessionID)
		return true, nil
	}

	threads, err := m.listThreads(ctx, cache.Project.ID, sessionID)
	if err != nil {
		return false, err
	}
	sessionData.Threads = threads
	if sessionData.Thread == nil {
		sessionData.Thread = map[string]state.ThreadData{}
	}
	seen := map[string]bool{}
	for _, thread := range threads {
		seen[thread.ID] = true
		threadData := sessionData.Thread[thread.ID]
		threadData.Thread = thread
		sessionData.Thread[thread.ID] = threadData
	}
	for threadID := range sessionData.Thread {
		if !seen[threadID] {
			delete(sessionData.Thread, threadID)
		}
	}
	cache.Session[sessionID] = sessionData
	return true, nil
}

func (m *Manager) subscribeProjectThreads(ctx context.Context, runtime *projectRuntime) error {
	desired := map[serviceclient.ProjectThreadSubscription]bool{}
	for sessionID, sessionData := range runtime.cache.Session {
		if !sessionSandboxRunning(sessionData.Session) {
			continue
		}
		for threadID := range sessionData.Thread {
			thread := serviceclient.ProjectThreadSubscription{SessionID: sessionID, ThreadID: threadID}
			desired[thread] = true
			if runtime.liveThreads[thread] {
				continue
			}
			if err := runtime.subscription.SubscribeThread(ctx, thread.SessionID, thread.ThreadID); err != nil {
				return fmt.Errorf("subscribe thread stream %s/%s/%s: %w", runtime.project.ID, thread.SessionID, thread.ThreadID, err)
			}
			runtime.liveThreads[thread] = true
		}
	}
	for thread := range runtime.liveThreads {
		if desired[thread] {
			continue
		}
		if err := runtime.subscription.UnsubscribeThread(ctx, thread.SessionID, thread.ThreadID); err != nil {
			return fmt.Errorf("unsubscribe thread stream %s/%s/%s: %w", runtime.project.ID, thread.SessionID, thread.ThreadID, err)
		}
		delete(runtime.liveThreads, thread)
	}
	return nil
}

func sessionSandboxRunning(session serverapi.Session) bool {
	switch session.SandboxStatus {
	case "ready", "running", "runtime":
		return true
	default:
		return false
	}
}

func clearSessionThreads(cache *state.ProjectData, sessionID string) {
	sessionData := cache.Session[sessionID]
	sessionData.Threads = nil
	sessionData.Thread = nil
	cache.Session[sessionID] = sessionData
}

func upsertSession(cache *state.ProjectData, session serverapi.Session) {
	if cache.Session == nil {
		cache.Session = map[string]state.SessionData{}
	}
	updated := false
	for i := range cache.Sessions {
		if cache.Sessions[i].ID == session.ID {
			cache.Sessions[i] = session
			updated = true
			break
		}
	}
	if !updated {
		cache.Sessions = append(cache.Sessions, session)
	}

	sessionData := cache.Session[session.ID]
	sessionData.Session = session
	cache.Session[session.ID] = sessionData
}

func ensureSessionData(cache *state.ProjectData, sessionID string) state.SessionData {
	if cache.Session == nil {
		cache.Session = map[string]state.SessionData{}
	}
	sessionData := cache.Session[sessionID]
	if sessionData.Thread == nil {
		sessionData.Thread = map[string]state.ThreadData{}
	}
	return sessionData
}
