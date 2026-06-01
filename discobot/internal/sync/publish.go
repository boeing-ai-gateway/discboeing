package sync

import (
	serverapi "github.com/obot-platform/discobot/server/api"

	"github.com/obot-platform/discobot/discobot/internal/state"
)

func (m *Manager) publishProjectList(projects []serverapi.Project) {
	m.store.SaveData(func(current *state.Data) {
		current.Projects = append([]serverapi.Project(nil), projects...)

		projectData := make(map[string]state.ProjectData, len(current.Project))
		for projectID, cache := range current.Project {
			projectData[projectID] = state.CloneProjectData(cache)
		}

		seen := make(map[string]bool, len(projects))
		for _, project := range projects {
			seen[project.ID] = true
		}
		for projectID := range projectData {
			if !seen[projectID] {
				delete(projectData, projectID)
			}
		}
		current.Project = projectData
	})
}

func (m *Manager) publishProject(project serverapi.Project, cache state.ProjectData) {
	m.store.SaveData(func(data *state.Data) {
		projects := make(map[string]state.ProjectData, len(data.Project)+1)
		for projectID, projectData := range data.Project {
			projects[projectID] = state.CloneProjectData(projectData)
		}
		projects[project.ID] = state.CloneProjectData(cache)
		data.Project = projects
	})
}

func (m *Manager) publishProjectThread(project serverapi.Project, cache state.ProjectData, sessionID string, threadID string) {
	m.store.SaveData(func(data *state.Data) {
		projects := make(map[string]state.ProjectData, len(data.Project)+1)
		for projectID, projectData := range data.Project {
			projects[projectID] = projectData
		}

		projectData, ok := projects[project.ID]
		if !ok {
			projects[project.ID] = state.CloneProjectData(cache)
			data.Project = projects
			return
		}
		projectData.Project = cache.Project
		if updated, ok := clonePublishedThreadMessages(projectData, cache, sessionID, threadID); ok {
			projectData = updated
		} else {
			projectData = state.CloneProjectData(cache)
		}

		projects[project.ID] = projectData
		data.Project = projects
	})
}

func clonePublishedThreadMessages(projectData state.ProjectData, cache state.ProjectData, sessionID string, threadID string) (state.ProjectData, bool) {
	sourceSessionData, ok := cache.Session[sessionID]
	if !ok || sourceSessionData.Thread == nil {
		return projectData, false
	}
	sourceThreadData, ok := sourceSessionData.Thread[threadID]
	if !ok || sourceThreadData.Thread.ID == "" {
		return projectData, false
	}

	sessionData, ok := projectData.Session[sessionID]
	if !ok || sessionData.Thread == nil {
		return projectData, false
	}
	threadData, ok := sessionData.Thread[threadID]
	if !ok || threadData.Thread.ID == "" {
		return projectData, false
	}

	sessions := make(map[string]state.SessionData, len(projectData.Session))
	for id, data := range projectData.Session {
		sessions[id] = data
	}
	projectData.Session = sessions

	threads := make(map[string]state.ThreadData, len(sessionData.Thread))
	for id, data := range sessionData.Thread {
		threads[id] = data
	}
	sessionData.Thread = threads

	threadData.Thread = sourceThreadData.Thread
	threadData.PendingHistory = sourceThreadData.PendingHistory
	threadData.Messages = cloneMessagesForUpdate(sourceThreadData.Messages)
	threadData.PendingMessages = cloneMessagesForUpdate(sourceThreadData.PendingMessages)
	sessionData.Thread[threadID] = threadData
	projectData.Session[sessionID] = sessionData
	return projectData, true
}
