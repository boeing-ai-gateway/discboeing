package collapsible

func collapsibleState(open bool) string {
	if open {
		return "open"
	}
	return "closed"
}
