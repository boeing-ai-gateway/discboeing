package viewmodel

// ShellSnapshot is the read model for the initial full-page render.
type ShellSnapshot struct {
	Title   string
	App     AppSnapshot
	Greeting GreetingSnapshot
}

// AppSnapshot is app-level view state owned by the backend.
type AppSnapshot struct {
	Name        string
	Description string
}

// GreetingSnapshot is the data state displayed by the hello world component.
type GreetingSnapshot struct {
	Message string
	Subject string
	Count   int
}

// DefaultShell returns the initial backend-authored application state.
func DefaultShell() ShellSnapshot {
	return ShellSnapshot{
		Title: "Discobot UI Go 2",
		App: AppSnapshot{
			Name:        "ui-go2",
			Description: "A small Datastar + templ scaffold mirroring ui-go structure.",
		},
		Greeting: GreetingSnapshot{
			Message: "Hello",
			Subject: "Datastar",
			Count:   0,
		},
	}
}
