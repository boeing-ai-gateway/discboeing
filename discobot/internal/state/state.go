// Package state owns Discobot's server-side application and view state.
package state

// Shell is the render model for the full-page shell.
type Shell struct {
	Data Data
	View View
}

// NewShell packages server app and view state for templ rendering.
func NewShell(data Data, view View) Shell {
	data = cloneData(data)
	view = normalizeShellView(data, NormalizeView(view))
	return Shell{
		Data: data,
		View: view,
	}
}
