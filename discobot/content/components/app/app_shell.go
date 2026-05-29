package app

import "github.com/obot-platform/discobot/discobot/internal/state"

func sessionPanelState(view state.View) state.SessionPanelState {
	return *view.PanelLayout.Panels["session"].Session
}

func editorPanelState(view state.View) state.EditorPanelState {
	return editorPanelStateFor(view.PanelLayout.Panels["editor"])
}

func composerPanelState(view state.View) state.ComposerPanelState {
	return composerPanelStateFor(view.PanelLayout.Panels["composer"])
}

func selectedSession(data state.Data, view state.View) (state.Session, bool) {
	for _, session := range data.Sessions {
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

func editorPanelStateFor(panel state.Panel) state.EditorPanelState {
	if panel.Editor == nil {
		return state.EditorPanelState{}
	}
	return *panel.Editor
}

func composerPanelStateFor(panel state.Panel) state.ComposerPanelState {
	if panel.Composer == nil {
		return state.ComposerPanelState{}
	}
	return *panel.Composer
}

func boolString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}
