package parts

import "github.com/obot-platform/discobot/ui-go/content/lib/viewmodel"

type DockWindowChromeSnapshot struct {
	DockMaximized                 bool
	ShiftWindowControlsForSidebar bool
	CloseLabel                    string
	MinimizeLabel                 string
	MaximizeTitle                 string
	ShellClass                    string
	HeaderClass                   string
	ContentClass                  string
	MaximizeRingOffsetClass       string
}

func DockWindowChromeFromDock(snapshot viewmodel.DockPanelSnapshot, closeLabel string, minimizeLabel string, maximizeTitle string) DockWindowChromeSnapshot {
	return DockWindowChromeSnapshot{
		DockMaximized:           snapshot.DockMaximized,
		CloseLabel:              dockWindowChromeLabel(closeLabel, "Close panel"),
		MinimizeLabel:           dockWindowChromeLabel(minimizeLabel, "Minimize panel"),
		MaximizeTitle:           dockWindowChromeLabel(maximizeTitle, "Toggle panel size"),
		MaximizeRingOffsetClass: dockWindowChromeRingOffsetClass(""),
	}
}

func dockWindowChromeLabel(label string, fallback string) string {
	if label == "" {
		return fallback
	}

	return label
}

func dockWindowChromeShellClass(className string) string {
	base := "flex h-full flex-col overflow-hidden rounded-md border border-sidebar-border bg-sidebar text-sidebar-foreground"
	if className != "" {
		base += " " + className
	}
	return base
}

func dockWindowChromeHeaderClass(className string) string {
	base := "flex h-10 shrink-0 items-center justify-between gap-3 border-b border-sidebar-border px-3"
	if className != "" {
		base += " " + className
	}
	return base
}

func dockWindowChromeContentClass(className string) string {
	base := "min-h-0 flex-1"
	if className != "" {
		base += " " + className
	}
	return base
}

func dockWindowChromeControlsClass(shift bool) string {
	base := "flex shrink-0 gap-1.5"
	if shift {
		base += " ml-36"
	}
	return base
}

func dockWindowChromeMaximizeClass(snapshot DockWindowChromeSnapshot) string {
	base := "size-3 rounded-full bg-green-500 transition-opacity hover:opacity-80"
	if snapshot.DockMaximized {
		base += " ring-2 ring-white/60 ring-offset-2 " + dockWindowChromeRingOffsetClass(snapshot.MaximizeRingOffsetClass)
	}
	return base
}

func dockWindowChromeRingOffsetClass(className string) string {
	if className == "" {
		return "ring-offset-sidebar"
	}
	return className
}
