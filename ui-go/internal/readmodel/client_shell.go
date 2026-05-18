package readmodel

import (
	"context"
	"fmt"
	"sort"
	"strings"

	api "github.com/obot-platform/discobot/server/api"
	"github.com/obot-platform/discobot/ui-go/content/lib/viewmodel"
	"github.com/obot-platform/discobot/ui-go/internal/live"
	"github.com/obot-platform/discobot/ui-go/internal/state"
)

// LiveScopeFromView derives the backend data scope needed to render view.
func LiveScopeFromView(view state.ViewState) live.Scope {
	sessionID, threadID := currentSelection(view)
	return live.Scope{
		ProjectID: live.DefaultProjectID,
		SessionID: sessionID,
		ThreadID:  threadID,
	}
}

// BuildShellFromBackend builds the rendered shell from persisted UI view state
// plus the live backend cache.
func BuildShellFromBackend(view state.ViewState, backend live.Snapshot) viewmodel.ShellSnapshot {
	if !backend.Ready {
		view.Workspace.Visible = true
		view.Workspace.Title = "Loading Discobot"
		view.Workspace.State = "Loading"
		view.Workspace.Message = "Loading backend data…"
		if backend.Error != "" {
			view.Workspace.State = "Error"
			view.Workspace.Message = backend.Error
		}
		return view
	}

	selectedSessionID, selectedThreadID := currentSelection(view)
	grouped := view.Sidebar.GroupedByWorkspace
	pending := view.Workspace.IsPending && selectedSessionID == "" && selectedThreadID == ""
	var shell viewmodel.ShellSnapshot
	if pending || len(backend.Sessions) == 0 {
		shell = BuildPendingShellFromBackend(backend, grouped)
	} else {
		shell = buildShellFromBackendSelection(backend, selectedSessionID, selectedThreadID, grouped)
	}
	applyViewState(&shell, view)
	return shell
}

// BuildPendingShellFromBackend builds pending new-session UI from live backend data.
func BuildPendingShellFromBackend(backend live.Snapshot, grouped bool) viewmodel.ShellSnapshot {
	sessions := append([]api.Session(nil), backend.Sessions...)
	sort.SliceStable(sessions, func(i, j int) bool {
		return sessions[i].UpdatedAt.After(sessions[j].UpdatedAt)
	})
	sidebar := viewmodel.AppSidebarSnapshot{
		ShowRecentThreads:  len(sessions) > 0,
		RecentOpen:         true,
		ShowAllHeader:      true,
		AllOpen:            true,
		GroupedByWorkspace: grouped,
		StreamEvents:       "0",
		Commands:           "0",
	}
	if grouped {
		for _, workspace := range backend.Workspaces {
			group := viewmodel.SidebarSessionGroup{
				Key:         workspace.ID,
				WorkspaceID: workspace.ID,
				Label:       workspaceLabel(workspace),
				SourceType:  workspace.SourceType,
			}
			for _, session := range sessions {
				if session.WorkspaceID == workspace.ID {
					group.Sessions = append(group.Sessions, sidebarSession(session, backend.ThreadsBySession[session.ID], "", ""))
				}
			}
			sidebar.SessionGroups = append(sidebar.SessionGroups, group)
		}
	} else {
		group := viewmodel.SidebarSessionGroup{Key: "all", Label: "All sessions"}
		for _, session := range sessions {
			group.Sessions = append(group.Sessions, sidebarSession(session, backend.ThreadsBySession[session.ID], "", ""))
		}
		if len(group.Sessions) > 0 {
			sidebar.SessionGroups = append(sidebar.SessionGroups, group)
		}
	}
	for _, session := range sessions {
		if thread, ok := sidebarRecentThread(session, backend.ThreadsBySession[session.ID], "", ""); ok {
			sidebar.RecentThreads = append(sidebar.RecentThreads, thread)
		}
	}
	shell := pendingSessionShell(backend.Workspaces, grouped)
	shell.Sidebar = sidebar
	shell.Workspace.Composer.Models = modelOptions(backend.Models)
	return shell
}

func buildShellFromBackendSelection(backend live.Snapshot, selectedSessionID string, selectedThreadID string, grouped bool) viewmodel.ShellSnapshot {
	sessions := append([]api.Session(nil), backend.Sessions...)
	sort.SliceStable(sessions, func(i, j int) bool {
		return sessions[i].UpdatedAt.After(sessions[j].UpdatedAt)
	})
	if selectedSessionID == "" && len(sessions) > 0 {
		selectedSessionID = sessions[0].ID
	}
	if selectedThreadID == "" && selectedSessionID != "" {
		selectedThreadID = primaryThreadID(selectedSessionID)
	}

	workspaceByID := map[string]api.Workspace{}
	for _, workspace := range backend.Workspaces {
		workspaceByID[workspace.ID] = workspace
	}

	sidebar := viewmodel.AppSidebarSnapshot{
		ShowRecentThreads:  true,
		RecentOpen:         true,
		ShowAllHeader:      true,
		AllOpen:            true,
		GroupedByWorkspace: grouped,
		StreamEvents:       "0",
		Commands:           "0",
	}
	for _, session := range sessions {
		if thread, ok := sidebarRecentThread(session, backend.ThreadsBySession[session.ID], selectedSessionID, selectedThreadID); ok {
			sidebar.RecentThreads = append(sidebar.RecentThreads, thread)
		}
	}
	if grouped {
		for _, workspace := range backend.Workspaces {
			group := viewmodel.SidebarSessionGroup{Key: workspace.ID, WorkspaceID: workspace.ID, Label: workspaceLabel(workspace), SourceType: workspace.SourceType}
			for _, session := range sessions {
				if session.WorkspaceID == workspace.ID {
					group.Sessions = append(group.Sessions, sidebarSession(session, backend.ThreadsBySession[session.ID], selectedSessionID, selectedThreadID))
				}
			}
			if len(group.Sessions) > 0 {
				sidebar.SessionGroups = append(sidebar.SessionGroups, group)
			}
		}
	} else {
		group := viewmodel.SidebarSessionGroup{Key: "all", Label: "All sessions"}
		for _, session := range sessions {
			group.Sessions = append(group.Sessions, sidebarSession(session, backend.ThreadsBySession[session.ID], selectedSessionID, selectedThreadID))
		}
		if len(group.Sessions) > 0 {
			sidebar.SessionGroups = append(sidebar.SessionGroups, group)
		}
	}

	selected := selectedSession(sessions, selectedSessionID)
	thread := selectedThread(backend.ThreadsBySession[selected.ID], selectedThreadID)
	workspace := workspaceByID[selected.WorkspaceID]
	title := sessionTitle(selected)
	if title == "" {
		title = "New Session"
	}
	return viewmodel.ShellSnapshot{
		Header:  viewmodel.HeaderSnapshot{ShowSessionToolbar: true, SessionTitle: title, ShowRefreshButton: true},
		Sidebar: sidebar,
		Workspace: viewmodel.SessionWorkspaceSnapshot{
			Title:          title,
			State:          workspaceStatus(workspace, selected),
			ThreadState:    selected.ThreadStatus,
			Message:        fmt.Sprintf("Selected %s in %s.", title, workspaceLabel(workspace)),
			ReserveSidebar: false,
			Visible:        true,
			Composer: viewmodel.ConversationComposerSnapshot{
				Placeholder:      "Ask Discobot to continue the Go UI migration…",
				SubmitStatus:     "ready",
				ModelID:          thread.Model,
				ModelLabel:       "Model",
				Models:           modelOptions(backend.Models),
				ReasoningValue:   thread.Reasoning,
				ServiceTierValue: thread.ServiceTier,
			},
			Conversation: viewmodel.ConversationPaneSnapshot{Status: "ready", ShowComposer: true},
		},
	}
}

func applyViewState(shell *viewmodel.ShellSnapshot, view state.ViewState) {
	shell.Sidebar.StreamEvents = view.Sidebar.StreamEvents
	shell.Sidebar.Commands = view.Sidebar.Commands
	shell.Sidebar.Collapsed = view.Sidebar.Collapsed
	shell.Sidebar.FloatingOpen = view.Sidebar.FloatingOpen
	shell.Sidebar.RecentOpen = view.Sidebar.RecentOpen
	shell.Sidebar.AllOpen = view.Sidebar.AllOpen
	shell.Sidebar.OpenMenu = view.Sidebar.OpenMenu
	shell.Sidebar.RenameDialog = view.Sidebar.RenameDialog
	shell.Sidebar.DeleteDialog = view.Sidebar.DeleteDialog
	shell.Header.Settings = view.Header.Settings
	shell.Workspace.Conversation.Messages = view.Workspace.Conversation.Messages
	shell.Workspace.Conversation.SelectionComment = view.Workspace.Conversation.SelectionComment
	shell.Workspace.Composer.Draft = view.Workspace.Composer.Draft
	shell.Workspace.Composer.Attachments = view.Workspace.Composer.Attachments
	shell.Workspace.Composer.Error = view.Workspace.Composer.Error
	shell.Workspace.Composer.ScheduledRunAfter = view.Workspace.Composer.ScheduledRunAfter
	shell.Workspace.Composer.ScheduleOpen = view.Workspace.Composer.ScheduleOpen
	shell.Workspace.Composer.PromptQueue = view.Workspace.Composer.PromptQueue
	if view.Workspace.Composer.ModelSelectionSet {
		shell.Workspace.Composer.ModelID = view.Workspace.Composer.ModelID
		shell.Workspace.Composer.ModelSelectionSet = true
	}
	if view.Workspace.Composer.ReasoningSet {
		shell.Workspace.Composer.ReasoningValue = view.Workspace.Composer.ReasoningValue
		shell.Workspace.Composer.ReasoningSet = true
	}
	if view.Workspace.Composer.ServiceTierSet {
		shell.Workspace.Composer.ServiceTierValue = view.Workspace.Composer.ServiceTierValue
		shell.Workspace.Composer.ServiceTierSet = true
	}
	applyComposerSelectedModel(&shell.Workspace.Composer)
	if shell.Workspace.IsPending {
		shell.Workspace.Composer.WorkspaceSelector = mergeWorkspaceSelector(
			shell.Workspace.Composer.WorkspaceSelector,
			view.Workspace.Composer.WorkspaceSelector,
		)
		applyWorkspaceSelectorSetupStatus(&shell.Workspace.Composer)
	}
}

func mergeWorkspaceSelector(next viewmodel.ConversationWorkspaceSelectorSnapshot, saved viewmodel.ConversationWorkspaceSelectorSnapshot) viewmodel.ConversationWorkspaceSelectorSnapshot {
	if saved.SelectedOption == "" {
		return next
	}
	next.FullWidth = saved.FullWidth
	next.RequiresInput = saved.RequiresInput
	next.SourceType = saved.SourceType
	next.SourceInput = saved.SourceInput
	next.SelectedOption = saved.SelectedOption
	next.Validating = saved.Validating
	next.ValidationPath = saved.ValidationPath
	next.ValidationSourceType = saved.ValidationSourceType
	next.ValidationValid = saved.ValidationValid
	next.ValidationError = saved.ValidationError
	next.SetupMessage = saved.SetupMessage
	next.Suggestions = saved.Suggestions
	next.HasSuggestionSelection = saved.HasSuggestionSelection
	next.SelectedSuggestionIndex = saved.SelectedSuggestionIndex
	next.Branch = saved.Branch
	next.Branches = saved.Branches
	next.ShowBranchSelector = saved.ShowBranchSelector
	return next
}

func applyWorkspaceSelectorSetupStatus(composer *viewmodel.ConversationComposerSnapshot) {
	selector := composer.WorkspaceSelector
	composer.SetupStatus.SetupMessage = selector.SetupMessage
	composer.SetupStatus.ValidationMessage = ""
	composer.SetupStatus.ValidationIsValid = true
	if !selector.RequiresInput || strings.TrimSpace(selector.SourceInput) == "" {
		return
	}
	if selector.Validating {
		composer.SetupStatus.ValidationMessage = "Validating workspace..."
		return
	}
	if strings.TrimSpace(selector.ValidationError) != "" {
		composer.SetupStatus.ValidationMessage = selector.ValidationError
		composer.SetupStatus.ValidationIsValid = false
	}
}

func currentSelection(view viewmodel.ShellSnapshot) (string, string) {
	for _, group := range view.Sidebar.SessionGroups {
		for _, session := range group.Sessions {
			if session.Selected {
				for _, thread := range session.Threads {
					if selectedThreadID, ok := selectedThreadInTree(thread); ok {
						return session.ID, selectedThreadID
					}
				}
				return session.ID, ""
			}
		}
	}
	for _, thread := range view.Sidebar.RecentThreads {
		if thread.Selected {
			return thread.SessionID, thread.ID
		}
	}
	return "", ""
}

func selectedThreadInTree(thread viewmodel.SidebarThreadItem) (string, bool) {
	if thread.Selected {
		return thread.ID, true
	}
	for _, child := range thread.Children {
		if id, ok := selectedThreadInTree(child); ok {
			return id, true
		}
	}
	return "", false
}

// BuildShellFromClient builds the session-scoped frontend snapshot from the
// Discobot client read side for the selected sidebar session/thread.
func BuildShellFromClient(ctx context.Context, client *api.Client, selectedSessionID string, selectedThreadID string, grouped bool) (viewmodel.ShellSnapshot, error) {
	projects, err := client.Projects.List(ctx)
	if err != nil {
		return viewmodel.ShellSnapshot{}, err
	}
	if len(projects) == 0 {
		return pendingSessionShell(nil, grouped), nil
	}

	project := projects[0]
	workspaces, err := client.Workspaces.List(ctx, project.ID)
	if err != nil {
		return viewmodel.ShellSnapshot{}, err
	}
	sessions, err := client.Sessions.List(ctx, project.ID)
	if err != nil {
		return viewmodel.ShellSnapshot{}, err
	}
	if len(sessions) == 0 {
		return pendingSessionShell(workspaces, grouped), nil
	}
	sort.SliceStable(sessions, func(i, j int) bool {
		return sessions[i].UpdatedAt.After(sessions[j].UpdatedAt)
	})

	if selectedSessionID == "" && len(sessions) > 0 {
		selectedSessionID = sessions[0].ID
	}
	if selectedThreadID == "" && selectedSessionID != "" {
		selectedThreadID = primaryThreadID(selectedSessionID)
	}

	workspaceByID := map[string]api.Workspace{}
	for _, workspace := range workspaces {
		workspaceByID[workspace.ID] = workspace
	}

	sidebar := viewmodel.AppSidebarSnapshot{
		ShowRecentThreads:  true,
		RecentOpen:         true,
		ShowAllHeader:      true,
		AllOpen:            true,
		GroupedByWorkspace: grouped,
		StreamEvents:       "0",
		Commands:           "0",
	}
	threadsBySession := map[string][]api.Thread{}
	for _, session := range sessions {
		threads, err := client.Sessions.ListThreads(ctx, project.ID, session.ID)
		if err != nil {
			return viewmodel.ShellSnapshot{}, err
		}
		threadsBySession[session.ID] = threads
		if thread, ok := sidebarRecentThread(session, threads, selectedSessionID, selectedThreadID); ok {
			sidebar.RecentThreads = append(sidebar.RecentThreads, thread)
		}
	}

	if grouped {
		for _, workspace := range workspaces {
			group := viewmodel.SidebarSessionGroup{
				Key:         workspace.ID,
				WorkspaceID: workspace.ID,
				Label:       workspaceLabel(workspace),
				SourceType:  workspace.SourceType,
			}
			for _, session := range sessions {
				if session.WorkspaceID == workspace.ID {
					group.Sessions = append(group.Sessions, sidebarSession(session, threadsBySession[session.ID], selectedSessionID, selectedThreadID))
				}
			}
			if len(group.Sessions) > 0 {
				sidebar.SessionGroups = append(sidebar.SessionGroups, group)
			}
		}
	} else {
		group := viewmodel.SidebarSessionGroup{Key: "all", Label: "All sessions"}
		for _, session := range sessions {
			group.Sessions = append(group.Sessions, sidebarSession(session, threadsBySession[session.ID], selectedSessionID, selectedThreadID))
		}
		if len(group.Sessions) > 0 {
			sidebar.SessionGroups = append(sidebar.SessionGroups, group)
		}
	}

	selected := selectedSession(sessions, selectedSessionID)
	workspace := workspaceByID[selected.WorkspaceID]
	title := sessionTitle(selected)
	if title == "" {
		title = "New Session"
	}
	return viewmodel.ShellSnapshot{
		Header: viewmodel.HeaderSnapshot{
			ShowSessionToolbar: true,
			SessionTitle:       title,
			ShowRefreshButton:  true,
		},
		Sidebar: sidebar,
		Workspace: viewmodel.SessionWorkspaceSnapshot{
			Title:          title,
			State:          workspaceStatus(workspace, selected),
			ThreadState:    selected.ThreadStatus,
			Message:        fmt.Sprintf("Selected %s in %s.", title, workspaceLabel(workspace)),
			ReserveSidebar: false,
			Composer: viewmodel.ConversationComposerSnapshot{
				Placeholder:  "Ask Discobot to continue the Go UI migration…",
				SubmitStatus: "ready",
				ModelLabel:   "Model",
			},
			Conversation: viewmodel.ConversationPaneSnapshot{
				Status:       "ready",
				ShowComposer: true,
			},
		},
	}, nil
}

// BuildPendingShellFromClient builds the normal conversation workspace in its
// pending new-session state while keeping any existing sessions in the sidebar
// unselected.
func BuildPendingShellFromClient(ctx context.Context, client *api.Client, grouped bool) (viewmodel.ShellSnapshot, error) {
	projects, err := client.Projects.List(ctx)
	if err != nil {
		return viewmodel.ShellSnapshot{}, err
	}
	if len(projects) == 0 {
		return pendingSessionShell(nil, grouped), nil
	}

	project := projects[0]
	workspaces, err := client.Workspaces.List(ctx, project.ID)
	if err != nil {
		return viewmodel.ShellSnapshot{}, err
	}
	sessions, err := client.Sessions.List(ctx, project.ID)
	if err != nil {
		return viewmodel.ShellSnapshot{}, err
	}
	sort.SliceStable(sessions, func(i, j int) bool {
		return sessions[i].UpdatedAt.After(sessions[j].UpdatedAt)
	})

	sidebar := viewmodel.AppSidebarSnapshot{
		ShowRecentThreads:  len(sessions) > 0,
		RecentOpen:         true,
		ShowAllHeader:      true,
		AllOpen:            true,
		GroupedByWorkspace: grouped,
		StreamEvents:       "0",
		Commands:           "0",
	}
	threadsBySession := map[string][]api.Thread{}
	for _, session := range sessions {
		threads, err := client.Sessions.ListThreads(ctx, project.ID, session.ID)
		if err != nil {
			return viewmodel.ShellSnapshot{}, err
		}
		threadsBySession[session.ID] = threads
		if thread, ok := sidebarRecentThread(session, threads, "", ""); ok {
			sidebar.RecentThreads = append(sidebar.RecentThreads, thread)
		}
	}
	if grouped {
		for _, workspace := range workspaces {
			group := viewmodel.SidebarSessionGroup{
				Key:         workspace.ID,
				WorkspaceID: workspace.ID,
				Label:       workspaceLabel(workspace),
				SourceType:  workspace.SourceType,
			}
			for _, session := range sessions {
				if session.WorkspaceID == workspace.ID {
					group.Sessions = append(group.Sessions, sidebarSession(session, threadsBySession[session.ID], "", ""))
				}
			}
			sidebar.SessionGroups = append(sidebar.SessionGroups, group)
		}
	} else {
		group := viewmodel.SidebarSessionGroup{Key: "all", Label: "All sessions"}
		for _, session := range sessions {
			group.Sessions = append(group.Sessions, sidebarSession(session, threadsBySession[session.ID], "", ""))
		}
		if len(group.Sessions) > 0 {
			sidebar.SessionGroups = append(sidebar.SessionGroups, group)
		}
	}

	shell := pendingSessionShell(workspaces, grouped)
	shell.Sidebar = sidebar
	return shell, nil
}

func pendingSessionShell(workspaces []api.Workspace, grouped bool) viewmodel.ShellSnapshot {
	return viewmodel.ShellSnapshot{
		Header: viewmodel.HeaderSnapshot{
			ShowSessionToolbar: false,
			SessionTitle:       "Discobot",
			ShowRefreshButton:  true,
		},
		Sidebar: pendingSidebar(workspaces, grouped),
		Workspace: viewmodel.SessionWorkspaceSnapshot{
			Title:          "",
			State:          "",
			Message:        "",
			ReserveSidebar: false,
			Visible:        true,
			IsPending:      true,
			Composer: viewmodel.ConversationComposerSnapshot{
				Placeholder:  "Ask Discobot to make a change…",
				IsPending:    true,
				SubmitStatus: "ready",
				ModelLabel:   "Model",
				SetupStatus: viewmodel.ComposerSessionSetupStatusSnapshot{
					ValidationIsValid: true,
				},
				WorkspaceSelector: pendingWorkspaceSelector(workspaces),
			},
			Conversation: viewmodel.ConversationPaneSnapshot{
				Status:       "ready",
				ShowComposer: true,
			},
		},
	}
}

func pendingSidebar(workspaces []api.Workspace, grouped bool) viewmodel.AppSidebarSnapshot {
	sidebar := viewmodel.AppSidebarSnapshot{
		ShowRecentThreads:  false,
		RecentOpen:         true,
		ShowAllHeader:      true,
		AllOpen:            true,
		GroupedByWorkspace: grouped,
		StreamEvents:       "0",
		Commands:           "0",
	}
	if grouped {
		for _, workspace := range workspaces {
			sidebar.SessionGroups = append(sidebar.SessionGroups, viewmodel.SidebarSessionGroup{
				Key:         workspace.ID,
				WorkspaceID: workspace.ID,
				Label:       workspaceLabel(workspace),
				SourceType:  workspace.SourceType,
			})
		}
	}
	return sidebar
}

func pendingWorkspaceSelector(workspaces []api.Workspace) viewmodel.ConversationWorkspaceSelectorSnapshot {
	selector := viewmodel.ConversationWorkspaceSelectorSnapshot{
		SelectedOption: "new-workspace",
		Workspaces:     make([]viewmodel.WorkspaceOption, 0, len(workspaces)),
	}
	for _, workspace := range workspaces {
		option := viewmodel.WorkspaceOption{
			ID:         workspace.ID,
			Label:      workspaceLabel(workspace),
			SourceType: workspace.SourceType,
			GitHub:     isGitHubWorkspace(workspace),
		}
		selector.Workspaces = append(selector.Workspaces, option)
		if selector.SelectedOption == "new-workspace" {
			selector.SelectedOption = pendingWorkspaceOptionValue(option)
		}
	}
	return selector
}

func isGitHubWorkspace(workspace api.Workspace) bool {
	if workspace.SourceType != "git" {
		return false
	}
	displayName := ""
	if workspace.DisplayName != nil {
		displayName = *workspace.DisplayName
	}
	value := strings.ToLower(workspace.Path + " " + displayName)
	return strings.Contains(value, "github.com") || strings.Contains(value, "github")
}

func pendingWorkspaceOptionValue(workspace viewmodel.WorkspaceOption) string {
	return "existing:" + workspace.ID
}

func modelOptions(models []api.ModelInfo) []viewmodel.ModelOption {
	options := make([]viewmodel.ModelOption, 0, len(models))
	for _, model := range models {
		options = append(options, viewmodel.ModelOption{
			ID:               model.ID,
			Name:             model.Name,
			Provider:         model.Provider,
			Description:      model.Description,
			Reasoning:        model.Reasoning,
			ReasoningLevels:  append([]string(nil), model.ReasoningLevels...),
			DefaultReasoning: model.DefaultReasoning,
			ServiceTiers:     append([]string(nil), model.ServiceTiers...),
		})
	}
	return options
}

func applyComposerSelectedModel(composer *viewmodel.ConversationComposerSnapshot) {
	for _, model := range composer.Models {
		if model.ID != composer.ModelID {
			continue
		}
		if model.Reasoning {
			composer.DefaultReasoning = model.DefaultReasoning
			composer.ReasoningLevels = append([]string(nil), model.ReasoningLevels...)
		} else {
			composer.DefaultReasoning = ""
			composer.ReasoningLevels = nil
			composer.ReasoningValue = ""
		}
		composer.ServiceTiers = append([]string(nil), model.ServiceTiers...)
		if len(model.ServiceTiers) == 0 {
			composer.ServiceTierValue = ""
		}
		return
	}
	composer.DefaultReasoning = ""
	composer.ReasoningLevels = nil
	composer.ServiceTiers = nil
}

func selectedSession(sessions []api.Session, selectedSessionID string) api.Session {
	for _, session := range sessions {
		if session.ID == selectedSessionID {
			return session
		}
	}
	if len(sessions) > 0 {
		return sessions[0]
	}
	return api.Session{}
}

func selectedThread(threads []api.Thread, selectedThreadID string) api.Thread {
	for _, thread := range threads {
		if thread.ID == selectedThreadID {
			return thread
		}
	}
	if len(threads) > 0 {
		return threads[0]
	}
	return api.Thread{}
}

func sidebarSession(session api.Session, threads []api.Thread, selectedSessionID string, selectedThreadID string) viewmodel.SidebarSessionItem {
	return viewmodel.SidebarSessionItem{
		ID:          session.ID,
		Name:        session.Name,
		DisplayName: displayName(session.DisplayName, session.Name),
		Selected:    session.ID == selectedSessionID,
		Status:      sessionStatus(session),
		Threads:     sidebarThreads(session.ID, threads, selectedSessionID, selectedThreadID),
	}
}

func sidebarRecentThread(session api.Session, threads []api.Thread, selectedSessionID string, selectedThreadID string) (viewmodel.SidebarThreadItem, bool) {
	for _, thread := range threads {
		if thread.ID == primaryThreadID(session.ID) {
			return sidebarThread(session.ID, thread, selectedSessionID, selectedThreadID), true
		}
	}
	if len(threads) > 0 {
		return sidebarThread(session.ID, threads[0], selectedSessionID, selectedThreadID), true
	}
	id := primaryThreadID(session.ID)
	return viewmodel.SidebarThreadItem{
		SessionID:   session.ID,
		ID:          id,
		Name:        primaryThreadName(session),
		DisplayName: primaryThreadName(session),
		Selected:    session.ID == selectedSessionID && selectedThreadID == id,
		Status:      sessionStatus(session),
		State:       threadState(session),
		Primary:     true,
	}, true
}

func sidebarThreads(sessionID string, threads []api.Thread, selectedSessionID string, selectedThreadID string) []viewmodel.SidebarThreadItem {
	var roots []viewmodel.SidebarThreadItem
	for _, thread := range threads {
		roots = append(roots, sidebarThread(sessionID, thread, selectedSessionID, selectedThreadID))
	}
	return roots
}

func sidebarThread(sessionID string, thread api.Thread, selectedSessionID string, selectedThreadID string) viewmodel.SidebarThreadItem {
	item := viewmodel.SidebarThreadItem{
		SessionID:   sessionID,
		ID:          thread.ID,
		Name:        thread.Name,
		DisplayName: thread.Name,
		Selected:    sessionID == selectedSessionID && selectedThreadID == thread.ID,
		Status:      threadStatus(thread),
		State:       thread.State,
		Primary:     thread.ID == primaryThreadID(sessionID),
	}
	return item
}

func primaryThreadID(sessionID string) string {
	return sessionID
}

func primaryThreadName(session api.Session) string {
	name := sessionTitle(session)
	if name == "" {
		return "New Thread"
	}
	return name
}

func sessionTitle(session api.Session) string {
	return displayName(session.DisplayName, session.Name)
}

func displayName(displayName *string, name string) string {
	if displayName != nil && *displayName != "" {
		return *displayName
	}
	return name
}

func workspaceLabel(workspace api.Workspace) string {
	if workspace.DisplayName != nil && *workspace.DisplayName != "" {
		return *workspace.DisplayName
	}
	if workspace.Path != "" {
		parts := strings.Split(strings.Trim(workspace.Path, "/"), "/")
		return parts[len(parts)-1]
	}
	return "Workspace"
}

func workspaceStatus(workspace api.Workspace, session api.Session) string {
	if session.SandboxStatus != "" {
		return session.SandboxStatus
	}
	if workspace.Status != "" {
		return workspace.Status
	}
	return "ready"
}

func sessionStatus(session api.Session) string {
	if session.SandboxStatus != "" {
		return session.SandboxStatus
	}
	return "ready"
}

func threadStatus(thread api.Thread) string {
	if thread.ActivityStatus != nil && thread.ActivityStatus.Status != "" {
		return thread.ActivityStatus.Status
	}
	if thread.Pending {
		return "pending"
	}
	if thread.State != "" {
		return thread.State
	}
	return "ready"
}

func threadState(session api.Session) string {
	if session.ThreadStatus != "" {
		return session.ThreadStatus
	}
	return "ready"
}
