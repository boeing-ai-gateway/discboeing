package helpers

import "github.com/obot-platform/discobot/discobot/internal/state"

func SessionPanelState(view state.View) state.SessionPanelState {
	return view.GlobalPanelLayout.SessionSidebar.State
}

func EditorPanelState(view state.View) state.EditorPanelState {
	sessionID := SessionPanelState(view).SelectedSessionID
	return view.SessionPanelLayouts[sessionID].Editor.State
}

func ComposerPanelState(view state.View) state.ComposerPanelState {
	sessionID := SessionPanelState(view).SelectedSessionID
	return view.SessionPanelLayouts[sessionID].Conversation.State
}

func SelectedSession(data state.Data, view state.View) (state.Session, bool) {
	for _, session := range state.Sessions(data) {
		if session.ID == SessionPanelState(view).SelectedSessionID {
			return session, true
		}
	}
	return state.Session{}, false
}

func SessionByID(sessions []state.Session, sessionID string) (state.Session, bool) {
	for _, session := range sessions {
		if session.ID == sessionID {
			return session, true
		}
	}
	return state.Session{}, false
}

func EditorPanelStateFor(panel state.Panel[state.EditorPanelState]) state.EditorPanelState {
	return panel.State
}

func ComposerPanelStateFor(panel state.Panel[state.ComposerPanelState]) state.ComposerPanelState {
	return panel.State
}
