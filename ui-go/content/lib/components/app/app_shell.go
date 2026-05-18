package app

import "github.com/obot-platform/discobot/ui-go/content/lib/viewmodel"

func appShellSidebarColumnClass(sidebar viewmodel.AppSidebarSnapshot) string {
	if sidebar.Collapsed {
		return "pointer-events-none absolute inset-y-0 left-0 z-20"
	}
	return "min-h-0 min-w-[300px] w-[16%] max-w-[50%]"
}

func appShellSidebarInnerClass(sidebar viewmodel.AppSidebarSnapshot) string {
	base := "box-border h-full min-h-0 pb-3 pl-3 pr-2 pt-1"
	if sidebar.Collapsed {
		return base + " pointer-events-none"
	}
	return base
}

func appShellDividerClass(sidebar viewmodel.AppSidebarSnapshot) string {
	if sidebar.Collapsed {
		return "hidden"
	}
	return "bg-transparent w-px after:w-3"
}
