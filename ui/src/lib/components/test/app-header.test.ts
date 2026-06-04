import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import test from "node:test";

const APP_HEADER_COMPONENT = path.resolve(
	import.meta.dirname,
	"../app/AppHeader.svelte",
);
const APP_MAC_WINDOW_SPACER_COMPONENT = path.resolve(
	import.meta.dirname,
	"../app/AppMacWindowSpacer.svelte",
);

function readAppHeaderSource() {
	return readFileSync(APP_HEADER_COMPONENT, "utf-8");
}

function readAppMacWindowSpacerSource() {
	return readFileSync(APP_MAC_WINDOW_SPACER_COMPONENT, "utf-8");
}

test("app header preserves the toolbar grid slot even when the session toolbar is hidden", () => {
	const source = readAppHeaderSource();

	assert.match(
		source,
		/<div class="relative z-20 min-w-0 px-2">\s*\{#if showSessionToolbar\}[\s\S]*<SessionToolbarStack \/>[\s\S]*\{\/if\}\s*<\/div>/,
	);
	assert.doesNotMatch(source, /class="desktop-no-drag min-w-0"/);
});

test("app header keeps window controls in a dedicated rightmost grid column on desktop", () => {
	const source = readAppHeaderSource();

	assert.match(source, /grid-cols-\[auto_minmax\(0,1fr\)_auto_auto\]/);
	assert.match(source, /grid-cols-\[auto_minmax\(0,1fr\)_auto\]/);
	assert.match(source, /class=\{`desktop-drag-region relative grid h-10/);
	assert.match(source, /data-desktop-drag-region/);
	assert.doesNotMatch(source, /data-tauri-drag-region/);
	assert.match(
		source,
		/\{#if !appEnvironment\.isMobile\}[\s\S]*class="desktop-no-drag relative z-20 flex h-full min-w-0 items-stretch justify-self-end pr-0"[\s\S]*<RightWindowControls \/>[\s\S]*\{\/if\}/,
	);
	assert.ok(source.includes("<SessionToolbarStack />"));
	assert.ok(
		source.includes('{appEnvironment.isMobile ? "New" : "New Session"}'),
	);
	assert.match(source, /class="absolute inset-0 pointer-events-auto"/);
	assert.match(
		source,
		/const appEnvironment = context\.view\.app\.environment;/,
	);
	assert.doesNotMatch(source, /new IsMobile/);
});

test("app header shows the mobile Sessions toggle to the right of the logo", () => {
	const source = readAppHeaderSource();

	assert.match(
		source,
		/\{#if appEnvironment\.isMobile\}[\s\S]*<DiscobotLogo size=\{24\} \/>/,
	);
	assert.match(
		source,
		/\{#if onToggleSidebar\}[\s\S]*class="desktop-no-drag gap-1 px-1\.5 text-xs font-medium uppercase tracking-\[0\.16em\] text-muted-foreground"[\s\S]*<PanelLeftIcon class="size-3\.5" \/>[\s\S]*<span>Sessions<\/span>[\s\S]*\{\/if\}/,
	);
	assert.doesNotMatch(source, /onclick=\{\(\) => onToggleSidebar\?\.\(\)\}/);
});

test("app header does not show desktop sessions controls next to the brand", () => {
	const source = readAppHeaderSource();

	assert.match(
		source,
		/<AppMacWindowSpacer \/>[\s\S]*<DiscobotBrand heightClass="h-6" \/>/,
	);
	assert.doesNotMatch(source, /\w*Desktop\w*Sidebar\w*Toggle\w*\?: boolean/);
	assert.doesNotMatch(
		source,
		/\w*Desktop\w*Sidebar\w*Toggle\w* && onToggleSidebar/,
	);
	assert.doesNotMatch(
		source,
		/class="desktop-no-drag inline-flex shrink-0 translate-y-1/,
	);
	assert.doesNotMatch(source, /desktopSessionsPopoverOpen/);
	assert.doesNotMatch(source, /closeDesktopSessionsPopover/);
	assert.doesNotMatch(source, /<Popover/);
	assert.doesNotMatch(source, /<AppSidebar/);
	assert.doesNotMatch(source, /Open sessions menu/);
	assert.doesNotMatch(source, /document\.addEventListener\("pointerdown"/);
});

test("app header delegates macOS spacer rendering to a dedicated component", () => {
	const source = readAppHeaderSource();

	assert.ok(source.includes("<AppMacWindowSpacer />"));
	assert.doesNotMatch(source, /LeftWindowControls/);
	assert.doesNotMatch(source, /isMacFullscreen/);
});

test("app header uses native window control capability flags instead of shell-specific checks", () => {
	const source = readAppHeaderSource();

	assert.match(source, /environment\.supportsNativeWindowControls &&/);
	assert.doesNotMatch(source, /environment\.runtime === "tauri"/);
});

test("app header keeps the settings button outside the drag region", () => {
	const source = readAppHeaderSource();

	assert.match(
		source,
		/import \{ useContext \} from "\$lib\/context\/context\.svelte";/,
	);
	assert.match(source, /const context = useContext\(\);/);
	assert.match(source, /const environment = context\.data\.environment;/);
	assert.match(
		source,
		/const updates = \$derived\(context\.view\.app\.updates\);/,
	);
	assert.match(
		source,
		/const preferences = \$derived\(context\.view\.app\.preferences\);/,
	);
	assert.doesNotMatch(source, /useAppContext/);
	assert.doesNotMatch(source, /const app = context\.actions\.app!/);
	assert.doesNotMatch(source, /const environment = app\.environment/);
	assert.doesNotMatch(source, /const ui = app\.ui/);
	assert.doesNotMatch(source, /const updates = app\.updates/);
	assert.doesNotMatch(source, /const preferences = app\.preferences/);
	assert.match(
		source,
		/onclick=\{\(\) => openSettingsDialog\(\)\}[\s\S]*class="desktop-no-drag relative"/,
	);
});

test("app header starts new sessions through root commands", () => {
	const source = readAppHeaderSource();

	assert.match(
		source,
		/import \{[\s\S]*openSettingsDialog,[\s\S]*startNewSession,[\s\S]*\} from "\$lib\/context\/commands\/app-view";/,
	);
	assert.match(source, /onclick=\{\(\) => startNewSession\(\)\}/);
	assert.doesNotMatch(source, /sessions\.startNew/);
});

test("app mac window spacer skips the spacer while native fullscreen is active", () => {
	const source = readAppMacWindowSpacerSource();

	assert.match(
		source,
		/import \{ useContext \} from "\$lib\/context\/context\.svelte";/,
	);
	assert.match(source, /const context = useContext\(\);/);
	assert.match(source, /const environment = context\.data\.environment;/);
	assert.doesNotMatch(source, /useAppContext/);
	assert.match(
		source,
		/import \{ withCurrentDesktopWindow \} from "\$lib\/shell";/,
	);
	assert.match(source, /let isMacFullscreen = \$state\(false\);/);
	assert.match(source, /await window\.isFullscreen\(\)/);
	assert.match(source, /window\.onResized\(\(\) => \{/);
	assert.doesNotMatch(source, /@tauri-apps\/api\/window/);
	assert.match(
		source,
		/environment\.supportsNativeWindowControls &&[\s\S]*environment\.windowControlsSide === "left" &&[\s\S]*!isMacFullscreen/,
	);
	assert.ok(
		source.includes('<div aria-hidden="true" class="w-14 shrink-0"></div>'),
	);
});
