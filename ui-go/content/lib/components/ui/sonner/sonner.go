package sonner

func toasterTheme(theme string) string {
	if theme == "dark" || theme == "light" || theme == "system" {
		return theme
	}
	return "system"
}
