package sync

import (
	"context"

	serverapi "github.com/obot-platform/discobot/server/api"
)

func (m *Manager) listProjects(ctx context.Context) ([]serverapi.Project, error) {
	var projects []serverapi.Project
	for project, err := range m.client.Projects.List(ctx) {
		if err != nil {
			return nil, err
		}
		projects = append(projects, project)
	}
	return projects, nil
}

func (m *Manager) listWorkspaces(ctx context.Context, projectID string) ([]serverapi.Workspace, error) {
	var workspaces []serverapi.Workspace
	for workspace, err := range m.client.Workspaces.List(ctx, projectID) {
		if err != nil {
			return nil, err
		}
		workspaces = append(workspaces, workspace)
	}
	return workspaces, nil
}

func (m *Manager) listSessions(ctx context.Context, projectID string) ([]serverapi.Session, error) {
	var sessions []serverapi.Session
	for session, err := range m.client.Project(projectID).Sessions.List(ctx) {
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, session)
	}
	return sessions, nil
}

func (m *Manager) getSession(ctx context.Context, projectID, sessionID string) (serverapi.Session, error) {
	session, err := m.client.Project(projectID).Sessions.Get(ctx, sessionID)
	if err != nil {
		return serverapi.Session{}, err
	}
	return *session, nil
}

func (m *Manager) listThreads(ctx context.Context, projectID, sessionID string) ([]serverapi.Thread, error) {
	var threads []serverapi.Thread
	for thread, err := range m.client.Project(projectID).Session(sessionID).Threads.List(ctx) {
		if err != nil {
			return nil, err
		}
		threads = append(threads, thread)
	}
	return threads, nil
}
