#!/usr/bin/env node
/**
 * Extract WSL image files from Docker registry for Tauri bundling
 *
 * This script uses `go tool crane` (from go-containerregistry) to pull a WSL
 * Docker image from the registry and extract the rootfs archive to
 * src-tauri/resources/wsl/ for bundling into the Windows app.
 *
 * Usage: node scripts/extract-wsl-image.mjs [image-ref] [arch]
 *   image-ref: Docker image reference (defaults to ghcr.io/obot-platform/discobot-wsl:main)
 *   arch: Architecture (amd64 or arm64, defaults to host arch)
 */

import { execSync } from "node:child_process";
import { mkdtempSync, mkdirSync, rmSync, statSync } from "node:fs";
import os from "node:os";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = dirname(fileURLToPath(import.meta.url));
const projectRoot = join(__dirname, "..");
const resourcesDir = join(projectRoot, "src-tauri", "resources", "wsl");

const imageRef = process.argv[2] || "ghcr.io/obot-platform/discobot-wsl:main";
const arch = process.argv[3] || (process.arch === "arm64" ? "arm64" : "amd64");

mkdirSync(resourcesDir, { recursive: true });

function maybeLoginToGhcr() {
	const token = process.env.GITHUB_TOKEN || process.env.GH_TOKEN;
	if (!token) {
		return;
	}

	const username = process.env.GITHUB_ACTOR || process.env.GH_USERNAME || "github";
	console.log("Authenticating go tool crane with ghcr.io...");
	execSync(`go tool crane auth login ghcr.io -u "${username}" --password-stdin`, {
		cwd: projectRoot,
		input: token,
		stdio: ["pipe", "inherit", "inherit"],
	});
}

function extractionHint(error) {
	const message = error instanceof Error ? error.message : String(error);
	if (message.includes("DENIED") || message.includes("UNAUTHORIZED")) {
		return " The image ref may not exist yet, or ghcr.io may require credentials for it. Override WSL_IMAGE_REF or set GH_TOKEN/GITHUB_TOKEN if needed.";
	}
	return "";
}

console.log(`Extracting WSL image files for ${arch}...`);
console.log(`Image: ${imageRef}`);
console.log(`Output directory: ${resourcesDir}`);

const extractFiles = ["discobot-rootfs.tar.zst"];
const tempDir = mkdtempSync(join(os.tmpdir(), "discobot-wsl-image-"));
const tempTarPath = join(tempDir, "image.tar");

try {
	maybeLoginToGhcr();
	console.log(`Exporting image with go tool crane (platform linux/${arch})...`);
	execSync(
		`go tool crane export --platform "linux/${arch}" "${imageRef}" "${tempTarPath}"`,
		{ cwd: projectRoot, stdio: "inherit" },
	);
	execSync(
		`tar xf "${tempTarPath}" -C "${resourcesDir}" ${extractFiles.join(" ")}`,
		{ cwd: projectRoot, stdio: "inherit" },
	);

	console.log("WSL image files extracted successfully:");
	for (const file of extractFiles) {
		const filePath = join(resourcesDir, file);
		try {
			const stats = statSync(filePath);
			const sizeMB = (stats.size / 1024 / 1024).toFixed(1);
			console.log(`  ${file}: ${sizeMB} MB`);
		} catch {
			console.log(`  ${file} (size unknown)`);
		}
	}
} catch (error) {
	console.error(`Failed to extract WSL image:${extractionHint(error)}`);
	console.error(error instanceof Error ? error.message : String(error));
	process.exit(1);
} finally {
	rmSync(tempDir, { force: true, recursive: true });
}
