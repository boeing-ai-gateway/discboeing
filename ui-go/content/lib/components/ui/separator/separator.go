package separator

import "strings"

func separatorClass(className string) string {
	base := "bg-border shrink-0 data-[orientation=horizontal]:h-px data-[orientation=horizontal]:w-full data-[orientation=vertical]:min-h-full data-[orientation=vertical]:w-px"
	if strings.TrimSpace(className) == "" {
		return base
	}
	return base + " " + strings.TrimSpace(className)
}

func separatorOrientation(orientation string) string {
	if orientation == "vertical" {
		return "vertical"
	}
	return "horizontal"
}

func separatorDataSlot(dataSlot string) string {
	if strings.TrimSpace(dataSlot) == "" {
		return "separator"
	}
	return dataSlot
}
