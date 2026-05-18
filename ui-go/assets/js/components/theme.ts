const THEME_KEY = "theme";
const COLOR_SCHEME_KEY = "theme.colorScheme";
const colorSchemeKey = (mode: ResolvedTheme) => `${COLOR_SCHEME_KEY}.${mode}`;
const THEME_MODES = ["light", "dark", "system"] as const;
const COLOR_SCHEMES = [
	"default",
	"flexoki",
	"nord",
	"tokyo-night",
	"solarized",
	"dracula",
	"catppuccin-mocha",
	"catppuccin-macchiato",
	"catppuccin-frappe",
	"alucard",
	"catppuccin-latte",
] as const;
const COLOR_SCHEME_NAMES: Record<ThemeColorScheme, string> = {
	default: "Default",
	flexoki: "Flexoki",
	nord: "Nord",
	"tokyo-night": "Tokyo Night",
	solarized: "Solarized",
	dracula: "Dracula",
	"catppuccin-mocha": "Catppuccin Mocha",
	"catppuccin-macchiato": "Catppuccin Macchiato",
	"catppuccin-frappe": "Catppuccin Frappé",
	alucard: "Alucard",
	"catppuccin-latte": "Catppuccin Latte",
};
const COLOR_SCHEME_MODES: Record<ThemeColorScheme, ResolvedTheme | "system"> = {
	default: "system",
	flexoki: "system",
	nord: "dark",
	"tokyo-night": "dark",
	solarized: "light",
	dracula: "dark",
	"catppuccin-mocha": "dark",
	"catppuccin-macchiato": "dark",
	"catppuccin-frappe": "dark",
	alucard: "light",
	"catppuccin-latte": "light",
};

type ResolvedTheme = "light" | "dark";
type ThemeMode = ResolvedTheme | "system";
type ThemeColorScheme = (typeof COLOR_SCHEMES)[number];

type ThemeWindow = typeof window & {
	uiGoThemeApply: typeof applyColorSchemeFromMode;
	uiGoThemeColorScheme: typeof storedColorScheme;
	uiGoThemeColorSchemeForMode: typeof colorSchemeForResolvedMode;
	uiGoThemeColorSchemeName: typeof colorSchemeName;
	uiGoThemeMode: typeof storedThemeMode;
	uiGoThemeOptionAvailable: typeof themeOptionAvailable;
	uiGoThemeResolve: typeof resolveThemeMode;
};

function isThemeMode(value: string | null): value is ThemeMode {
	return THEME_MODES.includes(value as ThemeMode);
}

function isColorScheme(value: string | null): value is ThemeColorScheme {
	return COLOR_SCHEMES.includes(value as ThemeColorScheme);
}

function preferredTheme(): ResolvedTheme {
	return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
}

function storedThemeMode(): ThemeMode {
	const stored = window.localStorage.getItem(THEME_KEY);
	return isThemeMode(stored) ? stored : "system";
}

function storedColorScheme(mode = resolvedTheme()): ThemeColorScheme {
	const storedForMode = window.localStorage.getItem(colorSchemeKey(mode));
	if (isColorScheme(storedForMode) && colorSchemeAvailableForMode(storedForMode, mode)) {
		return storedForMode;
	}
	return "default";
}

function resolveThemeMode(mode: ThemeMode): ResolvedTheme {
	return mode === "system" ? preferredTheme() : mode;
}

function colorSchemeAvailableForMode(
	scheme: ThemeColorScheme,
	mode: ResolvedTheme,
): boolean {
	const schemeMode = COLOR_SCHEME_MODES[scheme];
	return schemeMode === "system" || schemeMode === mode;
}

function colorSchemeForResolvedMode(
	scheme: ThemeColorScheme,
	mode: ResolvedTheme,
): ThemeColorScheme {
	return colorSchemeAvailableForMode(scheme, mode) ? scheme : storedColorScheme(mode);
}

function resolvedTheme(): ResolvedTheme {
	return document.documentElement.classList.contains("dark") ? "dark" : "light";
}

function applyTheme(mode: ThemeMode) {
	window.localStorage.setItem(THEME_KEY, mode);
	document.documentElement.classList.toggle("dark", resolveThemeMode(mode) === "dark");
}

function applyColorScheme(scheme: ThemeColorScheme, mode = resolvedTheme()) {
	window.localStorage.setItem(colorSchemeKey(mode), scheme);
	document.documentElement.setAttribute("data-theme", scheme);
}

function applyStoredTheme() {
	const mode = storedThemeMode();
	const resolved = resolveThemeMode(mode);
	applyTheme(mode);
	applyColorScheme(storedColorScheme(resolved), resolved);
}

function colorSchemeName(scheme: ThemeColorScheme): string {
	return COLOR_SCHEME_NAMES[scheme] ?? COLOR_SCHEME_NAMES.default;
}

function themeOptionAvailable(
	mode: ResolvedTheme | "system" | undefined,
	resolved: ResolvedTheme,
): boolean {
	return mode === "system" || mode === resolved;
}

function applyColorSchemeFromMode(mode: ThemeMode, scheme: ThemeColorScheme) {
	const resolved = resolveThemeMode(mode);
	applyTheme(mode);
	applyColorScheme(colorSchemeForResolvedMode(scheme, resolved), resolved);
}

function registerLegacyGlobals() {
	const themeWindow = window as ThemeWindow;
	themeWindow.uiGoThemeApply = applyColorSchemeFromMode;
	themeWindow.uiGoThemeColorScheme = storedColorScheme;
	themeWindow.uiGoThemeColorSchemeForMode = colorSchemeForResolvedMode;
	themeWindow.uiGoThemeColorSchemeName = colorSchemeName;
	themeWindow.uiGoThemeMode = storedThemeMode;
	themeWindow.uiGoThemeOptionAvailable = themeOptionAvailable;
	themeWindow.uiGoThemeResolve = resolveThemeMode;
}

export const theme = {
	applyColorSchemeFromMode,
	applyStored: applyStoredTheme,
	colorSchemeForResolvedMode,
	colorSchemeName,
	optionAvailable: themeOptionAvailable,
	registerLegacyGlobals,
	resolve: resolveThemeMode,
	storedColorScheme,
	storedMode: storedThemeMode,
	watchSystem(onChange?: () => void) {
		window.matchMedia("(prefers-color-scheme: dark)").addEventListener("change", () => {
			if (storedThemeMode() !== "system") {
				return;
			}
			applyStoredTheme();
			window.dispatchEvent(new CustomEvent("ui-go-theme-system-change"));
			onChange?.();
		});
	},
};
