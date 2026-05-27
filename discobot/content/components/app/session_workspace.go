package app

import "github.com/obot-platform/discobot/discobot/internal/state"

func selectedSession(data state.Data, view state.View) (state.Session, bool) {
	for _, session := range data.Sessions {
		if session.ID == view.SelectedSessionID {
			return session, true
		}
	}
	return state.Session{}, false
}
