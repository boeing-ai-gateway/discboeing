package checkbox

import "strings"

func checkboxClass(className string) string {
	base := "border-input dark:bg-input/30 data-[state=checked]:bg-primary data-[state=checked]:text-primary-foreground dark:data-[state=checked]:bg-primary data-[state=checked]:border-primary focus-visible:border-ring focus-visible:ring-ring/50 aria-invalid:ring-destructive/20 dark:aria-invalid:ring-destructive/40 aria-invalid:border-destructive peer flex size-4 shrink-0 items-center justify-center rounded-[4px] border shadow-xs transition-shadow outline-none focus-visible:ring-[3px] disabled:cursor-not-allowed disabled:opacity-50"
	if strings.TrimSpace(className) != "" {
		base += " " + className
	}
	return base
}

func checkboxState(checked bool, indeterminate bool) string {
	if checked {
		return "checked"
	}
	if indeterminate {
		return "indeterminate"
	}
	return "unchecked"
}

func checkboxAriaChecked(checked bool, indeterminate bool) string {
	if indeterminate && !checked {
		return "mixed"
	}
	if checked {
		return "true"
	}
	return "false"
}
