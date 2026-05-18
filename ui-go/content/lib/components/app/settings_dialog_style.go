package app

func settingsTabsGridClass(showUpdate bool) string {
	if showUpdate {
		return "grid-cols-5"
	}
	return "grid-cols-4"
}

func settingsPillClass(active bool) string {
	base := "rounded-full border px-3 py-1 text-sm capitalize"
	if active {
		return base + " border-primary bg-primary text-primary-foreground shadow-sm"
	}
	return base + " border-transparent bg-transparent text-muted-foreground"
}

func settingsSwitchClass(checked bool) string {
	base := "inline-flex h-5 w-9 items-center rounded-full border transition-colors"
	if checked {
		return base + " border-primary bg-primary"
	}
	return base + " border-border bg-muted"
}

func settingsSwitchThumbClass(checked bool) string {
	base := "block size-4 rounded-full bg-background shadow transition-transform"
	if checked {
		return base + " translate-x-4"
	}
	return base + " translate-x-0.5"
}
