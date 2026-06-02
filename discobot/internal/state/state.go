// Package state owns Discobot's server-side application state.
package state

// NewData returns an immutable snapshot clone of server application state.
func NewData(data Data) Data {
	return cloneData(data)
}
