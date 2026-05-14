export const categoryLabels = {
	layout: "Layout",
	form: "Forms",
	navigation: "Navigation",
	overlay: "Overlays",
	data: "Data display",
	feedback: "Feedback",
	utility: "Utilities",
} as const;

export type UiComponentCategory = keyof typeof categoryLabels;
export type UiComponentFilter = "all" | UiComponentCategory;

export type UiComponentCatalogEntry = {
	category: UiComponentCategory;
	description: string;
	exportName: string;
	featured: boolean;
	importPath: string;
	label: string;
	name: string;
};

const componentNames = [
	"alert-dialog",
	"alert",
	"badge",
	"button-group",
	"button",
	"card",
	"checkbox",
	"collapsible",
	"context-menu",
	"dialog",
	"dropdown-menu",
	"hover-card",
	"input-group",
	"input",
	"item",
	"kbd",
	"label",
	"native-select",
	"popover",
	"progress",
	"resizable",
	"select",
	"separator",
	"sheet",
	"skeleton",
	"sonner",
	"spinner",
	"split-dropdown-button",
	"switch",
	"tabs",
	"textarea",
	"toggle-group",
	"toggle",
	"tooltip",
] as const;

const categoryOrder: UiComponentCategory[] = [
	"layout",
	"form",
	"navigation",
	"overlay",
	"data",
	"feedback",
	"utility",
];

const featuredNames = new Set([
	"badge",
	"button",
	"card",
	"dialog",
	"dropdown-menu",
	"input",
	"select",
	"sheet",
	"switch",
	"tabs",
	"textarea",
	"tooltip",
]);

const descriptions: Partial<Record<string, string>> = {
	"alert-dialog":
		"Ask for confirmation before destructive or high-friction actions.",
	alert: "Present inline status, warnings, and contextual feedback.",
	badge: "Attach a compact label for state, tags, or counts.",
	button: "Trigger a primary, secondary, or contextual action.",
	"button-group": "Keep related actions visually and semantically grouped.",
	card: "Wrap related content and actions in a consistent surface.",
	checkbox: "Capture boolean choices or multi-select options.",
	collapsible: "Expand or collapse a region while staying in the same layout.",
	"context-menu":
		"Expose secondary actions at the cursor or long-press position.",
	dialog: "Open a centered modal workflow that blocks the rest of the app.",
	"dropdown-menu": "Offer a compact menu of contextual actions.",
	"hover-card": "Preview extra context when someone hovers a trigger.",
	input: "Capture short single-line text or search queries.",
	"input-group": "Combine inputs with icons, prefixes, or inline actions.",
	item: "Lay out dense list rows with media, text, and trailing actions.",
	kbd: "Show keyboard shortcuts in a compact visual token.",
	label: "Describe an associated form control accessibly.",
	"native-select": "Use the platform select element with consistent styling.",
	popover:
		"Anchor a floating panel to a trigger without taking over the screen.",
	progress: "Communicate completion progress for an ongoing task.",
	resizable: "Let the user drag panes to change layout proportions.",
	select: "Pick from a styled listbox-style set of options.",
	separator: "Visually divide related content or toolbar groups.",
	sheet: "Slide in supporting content from a screen edge.",
	skeleton: "Reserve space while content is loading.",
	sonner: "Display toast notifications and transient system messages.",
	spinner: "Show an indeterminate loading state.",
	switch: "Toggle a setting on and off.",
	tabs: "Switch between sibling views within the same surface.",
	textarea: "Capture longer multi-line text input.",
	toggle: "Turn a compact pressed state on or off.",
	"toggle-group": "Manage one-or-many pressed button selections.",
	tooltip: "Reveal short help text for controls and icons.",
};

function toLabel(name: string): string {
	return name
		.split("-")
		.map((part) => part.charAt(0).toUpperCase() + part.slice(1))
		.join(" ");
}

function toExportName(name: string): string {
	return toLabel(name).replaceAll(" ", "");
}

function categorizeComponent(name: string): UiComponentCategory {
	if (
		[
			"checkbox",
			"input",
			"input-group",
			"label",
			"native-select",
			"select",
			"switch",
			"textarea",
			"toggle",
			"toggle-group",
		].includes(name)
	) {
		return "form";
	}

	if (["context-menu", "dropdown-menu", "tabs"].includes(name)) {
		return "navigation";
	}

	if (
		[
			"alert-dialog",
			"dialog",
			"hover-card",
			"popover",
			"sheet",
			"tooltip",
		].includes(name)
	) {
		return "overlay";
	}

	if (["alert", "progress", "skeleton", "sonner", "spinner"].includes(name)) {
		return "feedback";
	}

	if (["badge", "kbd", "separator"].includes(name)) {
		return "utility";
	}

	return "layout";
}

function defaultDescription(category: UiComponentCategory): string {
	switch (category) {
		case "layout":
			return "Structure the shell and arrange content within a page or panel.";
		case "form":
			return "Capture user input and control interactive settings.";
		case "navigation":
			return "Move through views, menus, and grouped destinations.";
		case "overlay":
			return "Present transient UI above the current screen.";
		case "data":
			return "Show structured information, tables, and charts.";
		case "feedback":
			return "Communicate loading, status, and empty states.";
		case "utility":
			return "Support the rest of the system with compact helper UI.";
	}
}

export const uiComponentCatalog: UiComponentCatalogEntry[] = componentNames
	.map((name) => {
		if (!name) {
			throw new Error(`Unable to derive component name from path: ${name}`);
		}

		const category = categorizeComponent(name);
		return {
			category,
			description: descriptions[name] ?? defaultDescription(category),
			exportName: toExportName(name),
			featured: featuredNames.has(name),
			importPath: `$lib/components/ui/${name}`,
			label: toLabel(name),
			name,
		};
	})
	.sort((left, right) => {
		return (
			categoryOrder.indexOf(left.category) -
				categoryOrder.indexOf(right.category) ||
			left.label.localeCompare(right.label)
		);
	});

export const uiComponentFilters: UiComponentFilter[] = [
	"all",
	...categoryOrder,
];
