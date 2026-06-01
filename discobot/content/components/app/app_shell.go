package app

import "github.com/obot-platform/discobot/discobot/internal/state"

func sessionPanelState(view state.View) state.SessionPanelState {
	return view.GlobalPanelLayout.SessionSidebar.State
}

func editorPanelState(view state.View) state.EditorPanelState {
	sessionID := sessionPanelState(view).SelectedSessionID
	return view.SessionPanelLayouts[sessionID].Editor.State
}

func composerPanelState(view state.View) state.ComposerPanelState {
	sessionID := sessionPanelState(view).SelectedSessionID
	return view.SessionPanelLayouts[sessionID].Conversation.State
}

func selectedSession(data state.Data, view state.View) (state.Session, bool) {
	for _, session := range state.Sessions(data) {
		if session.ID == sessionPanelState(view).SelectedSessionID {
			return session, true
		}
	}
	return state.Session{}, false
}

func sessionByID(sessions []state.Session, sessionID string) (state.Session, bool) {
	for _, session := range sessions {
		if session.ID == sessionID {
			return session, true
		}
	}
	return state.Session{}, false
}

func editorPanelStateFor(panel state.Panel[state.EditorPanelState]) state.EditorPanelState {
	return panel.State
}

func composerPanelStateFor(panel state.Panel[state.ComposerPanelState]) state.ComposerPanelState {
	return panel.State
}

func boolString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}
