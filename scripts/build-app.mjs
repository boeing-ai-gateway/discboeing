#!/usr/bin/env node
import { execFileSync } from "node:child_process";
import { readFileSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = dirname(fileURLToPath(import.meta.url));
const projectRoot = join(__dirname, "..");
const packageJSON = JSON.parse(
	readFileSync(join(projectRoot, "package.json"), "utf-8"),
);

function normalizeTauriVersion(version) {
	const match = /^(\d+)\.(\d+)\.(\d+)(?:-([0-9A-Za-z.-]+))?(?:\+.*)?$/.exec(version);
	if (!match) {
		throw new Error(
			`Version ${JSON.stringify(version)} is not semver-like. Set DISCOBOT_TAURI_VERSION to an MSI-safe value such as 0.0.0-0.`,
		);
	}

	const [, major, minor, patch, prerelease = ""] = match;
	const base = `${major}.${minor}.${patch}`;
	if (!prerelease) {
		return base;
	}

	if (/^\d+$/.test(prerelease)) {
		const value = Number(prerelease);
		return `${base}-${Math.min(value, 65535)}`;
	}

	const numericIdentifiers = prerelease.match(/\d+/g) || [];
	for (const identifier of numericIdentifiers) {
		const value = Number(identifier);
		if (value <= 65535) {
			return `${base}-${value}`;
		}
	}

	return `${base}-0`;
}

function validateExplicitTauriVersion(version) {
	const match = /^(\d+)\.(\d+)\.(\d+)(?:-(\d+))?$/.exec(version);
	if (!match) {
		throw new Error(
			`DISCOBOT_TAURI_VERSION must be major.minor.patch or major.minor.patch-number, got ${JSON.stringify(version)}.`,
		);
	}

	const prerelease = match[4];
	if (prerelease && Number(prerelease) > 65535) {
		throw new Error(
			`DISCOBOT_TAURI_VERSION prerelease must be 65535 or smaller, got ${JSON.stringify(version)}.`,
		);
	}

	return version;
}

function runNodeScript(scriptPath, args = []) {
	execFileSync(process.execPath, [scriptPath, ...args], {
		cwd: projectRoot,
		env: { ...process.env },
		stdio: "inherit",
	});
}

function runTauri(args) {
	runNodeScript(join(projectRoot, "node_modules", "@tauri-apps", "cli", "tauri.js"), args);
}

function main() {
	const discobotVersion = process.env.DISCOBOT_VERSION || packageJSON.version;
	const tauriVersion = process.env.DISCOBOT_TAURI_VERSION
		? validateExplicitTauriVersion(process.env.DISCOBOT_TAURI_VERSION)
		: normalizeTauriVersion(discobotVersion);
	const forwardedArgs = process.argv.slice(2);

	console.log(`[build-app] Discobot version: ${discobotVersion}`);
	console.log(`[build-app] Tauri bundle version: ${tauriVersion}`);
	console.log(
		`[build-app] WSL image ref: ${process.env.WSL_IMAGE_REF || "ghcr.io/obot-platform/discobot-wsl:main"}`,
	);

	runNodeScript(join(__dirname, "prepare-tauri-assets.mjs"));
	runTauri([
		"build",
		"--config",
		JSON.stringify({ version: tauriVersion }),
		...forwardedArgs,
	]);
}

export { normalizeTauriVersion, validateExplicitTauriVersion };

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
	main();
}
