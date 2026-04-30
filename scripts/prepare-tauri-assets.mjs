#!/usr/bin/env node
import { execFileSync } from "node:child_process";
import os from "node:os";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = dirname(fileURLToPath(import.meta.url));
const projectRoot = join(__dirname, "..");

if (process.env.DISCOBOT_TAURI_SKIP_ASSET_PREP === "1") {
	console.log("[prepare-tauri-assets] Skipping bundled runtime asset preparation");
	process.exit(0);
}

const assetConfigs = {
	darwin: {
		arch: process.env.DISCOBOT_TAURI_VZ_ARCH || (process.arch === "arm64" ? "arm64" : "amd64"),
		imageRef: process.env.VZ_IMAGE_REF || "ghcr.io/obot-platform/discobot-vz:main",
		label: "VZ",
		script: "extract-vz-image.mjs",
	},
	win32: {
		arch: process.env.DISCOBOT_TAURI_WSL_ARCH || (process.arch === "arm64" ? "arm64" : "amd64"),
		imageRef: process.env.WSL_IMAGE_REF || "ghcr.io/obot-platform/discobot-wsl:main",
		label: "WSL",
		script: "extract-wsl-image.mjs",
	},
};

const assetConfig = assetConfigs[os.platform()];

if (!assetConfig) {
	console.log(
		`[prepare-tauri-assets] No bundled runtime asset preparation for ${os.platform()}`,
	);
	process.exit(0);
}

const shouldFailOnMissingAssets = process.env.DISCOBOT_TAURI_REQUIRE_ASSETS === "1";

console.log(
	`[prepare-tauri-assets] Preparing bundled ${assetConfig.label} runtime assets from ${assetConfig.imageRef} (${assetConfig.arch})`,
);

try {
	execFileSync(
		process.execPath,
		[join(__dirname, assetConfig.script), assetConfig.imageRef, assetConfig.arch],
		{
			cwd: projectRoot,
			env: { ...process.env },
			stdio: "inherit",
		},
	);
} catch (error) {
	if (shouldFailOnMissingAssets) {
		throw error;
	}

	console.warn(
		`[prepare-tauri-assets] Continuing without bundled ${assetConfig.label} runtime assets. Set DISCOBOT_TAURI_REQUIRE_ASSETS=1 to make this fatal.`,
	);
}
