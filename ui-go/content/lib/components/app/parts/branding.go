package parts

import "github.com/a-h/templ"

const (
	defaultDiscobotBrandHeightClass = "h-5"
	defaultDiscobotBrandTitle       = "Discobot brand"
	defaultDiscobotLogoTitle        = "Discobot logo"
	defaultDiscobotLogoSize         = 24
	defaultDiscobotWordmarkTitle    = "Discobot wordmark"
)

// brandImageClass builds the Tailwind class string for brand/wordmark <img> and
// <svg> elements. The base always includes "block w-auto shrink-0"; heightClass
// and className are appended when non-empty.
func brandImageClass(className string, heightClass string) string {
	base := "block w-auto shrink-0"
	if heightClass != "" {
		base += " " + heightClass
	}
	if className != "" {
		base += " " + className
	}
	return base
}

// logoImageClass builds the Tailwind class string for the logo <img> element.
// The base always includes "block shrink-0"; className is appended when non-empty.
func logoImageClass(className string) string {
	base := "block shrink-0"
	if className != "" {
		base += " " + className
	}
	return base
}

func discobotBrandHeightClass(heightClass string) string {
	if heightClass == "" {
		return defaultDiscobotBrandHeightClass
	}
	return heightClass
}

func discobotTitle(title string, fallback string) string {
	if title == "" {
		return fallback
	}
	return title
}

func discobotLogoSize(size int) int {
	if size <= 0 {
		return defaultDiscobotLogoSize
	}
	return size
}

// DefaultDiscobotBrand returns DiscobotBrand with the defaults that match the
// Svelte component's $props() defaults: heightClass="h-5", title="Discobot brand".
func DefaultDiscobotBrand() templ.Component {
	return DiscobotBrand(
		"",
		discobotBrandHeightClass(""),
		discobotTitle("", defaultDiscobotBrandTitle),
	)
}

// DefaultDiscobotLogo returns DiscobotLogo with the defaults that match the
// Svelte component's $props() defaults: size=24, title="Discobot logo".
func DefaultDiscobotLogo() templ.Component {
	return DiscobotLogo(
		"",
		discobotTitle("", defaultDiscobotLogoTitle),
		discobotLogoSize(0),
	)
}

// DefaultDiscobotWordmark returns DiscobotWordmark with the defaults that match
// the Svelte component's $props() defaults: title="Discobot wordmark".
func DefaultDiscobotWordmark() templ.Component {
	return DiscobotWordmark("", discobotTitle("", defaultDiscobotWordmarkTitle))
}
