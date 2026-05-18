package input

import "strings"

func inputClass(inputType string, className string) string {
	base := ""
	if inputType == "file" {
		base = "selection:bg-primary dark:bg-input/30 selection:text-primary-foreground border-input ring-offset-background placeholder:text-muted-foreground flex h-9 w-full min-w-0 rounded-md border bg-transparent px-3 pt-1.5 text-sm font-medium shadow-xs transition-[color,box-shadow] outline-none disabled:cursor-not-allowed disabled:opacity-50 focus-visible:border-ring focus-visible:ring-ring/50 focus-visible:ring-[3px] aria-invalid:ring-destructive/20 dark:aria-invalid:ring-destructive/40 aria-invalid:border-destructive"
	} else {
		base = "border-input bg-background selection:bg-primary dark:bg-input/30 selection:text-primary-foreground ring-offset-background placeholder:text-muted-foreground flex h-9 w-full min-w-0 rounded-md border px-3 py-1 text-base shadow-xs transition-[color,box-shadow] outline-none disabled:cursor-not-allowed disabled:opacity-50 md:text-sm focus-visible:border-ring focus-visible:ring-ring/50 focus-visible:ring-[3px] aria-invalid:ring-destructive/20 dark:aria-invalid:ring-destructive/40 aria-invalid:border-destructive"
	}
	if strings.TrimSpace(className) != "" {
		base += " " + className
	}
	return base
}

func inputType(inputType string) string {
	if strings.TrimSpace(inputType) == "" {
		return "text"
	}
	return inputType
}

func inputDataSlot(dataSlot string) string {
	if strings.TrimSpace(dataSlot) == "" {
		return "input"
	}
	return dataSlot
}
