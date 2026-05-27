import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const root = dirname(dirname(fileURLToPath(import.meta.url)));
const iconRoot = join(root, "node_modules", "@vscode", "codicons", "src", "icons");
const outputPath = join(root, "static", "vendor", "codicons.svg");

const icons = [
	"add",
	"arrow-down",
	"arrow-left",
	"arrow-right",
	"arrow-up",
	"attach",
	"chevron-down",
	"chevron-right",
	"chrome-close",
	"chrome-maximize",
	"chrome-minimize",
	"close",
	"collapse-all",
	"code",
	"expand-all",
	"file",
	"file-code",
	"file-media",
	"file-text",
	"folder",
	"gear",
	"git-branch",
	"json",
	"kebab-vertical",
	"layout-sidebar-left",
	"layout-sidebar-right",
	"markdown",
	"play",
	"refresh",
	"robot",
	"search",
	"settings",
	"shield",
	"symbol-key",
	"symbol-color",
	"terminal",
];

function symbolFor(icon) {
	const source = readFileSync(join(iconRoot, `${icon}.svg`), "utf8");
	const viewBox = source.match(/viewBox="([^"]+)"/)?.[1];
	const body = source
		.replace(/^<svg\b[^>]*>/, "")
		.replace(/<\/svg>\s*$/, "")
		.trim();

	if (!viewBox || !body) {
		throw new Error(`Could not read Codicon SVG: ${icon}`);
	}

	return `\t<symbol id="codicon-${icon}" viewBox="${viewBox}">${body}</symbol>`;
}

mkdirSync(dirname(outputPath), { recursive: true });
writeFileSync(
	outputPath,
	[
		"<!-- Generated from @vscode/codicons. Codicons are licensed CC BY 4.0. -->",
		'<svg xmlns="http://www.w3.org/2000/svg">',
		...icons.map(symbolFor),
		"</svg>",
		"",
	].join("\n"),
);
