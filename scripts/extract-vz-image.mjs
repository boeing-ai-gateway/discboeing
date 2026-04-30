#!/usr/bin/env node
/**
 * Extract VZ image files from Docker registry for Tauri bundling
 *
 * This script uses `go tool crane` (from go-containerregistry) to pull a VZ
 * Docker image from the registry and extract the kernel and rootfs files to
 * src-tauri/resources/vz/ for bundling into the macOS app.
 *
 * Usage: node scripts/extract-vz-image.mjs [image-ref] [arch]
 *   image-ref: Docker image reference (defaults to ghcr.io/obot-platform/discobot-vz:main)
 *   arch: Architecture (amd64 or arm64, defaults to host arch)
 */

import { execSync } from "node:child_process";
import { mkdtempSync, mkdirSync, rmSync, statSync, unlinkSync } from "node:fs";
import os from "node:os";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = dirname(fileURLToPath(import.meta.url));
const projectRoot = join(__dirname, "..");
const resourcesDir = join(projectRoot, "src-tauri", "resources", "vz");

const imageRef = process.argv[2] || "ghcr.io/obot-platform/discobot-vz:main";
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
		return " The image ref may not exist yet, or ghcr.io may require credentials for it. Override VZ_IMAGE_REF or set GH_TOKEN/GITHUB_TOKEN if needed.";
	}
	return "";
}

console.log(`Extracting VZ image files for ${arch}...`);
console.log(`Image: ${imageRef}`);
console.log(`Output directory: ${resourcesDir}`);

const extractFiles = ["vmlinuz", "kernel-version", "discobot-rootfs.squashfs"];
const outputFiles =
	arch === "arm64"
		? ["vmlinux", "kernel-version", "discobot-rootfs.squashfs"]
		: extractFiles;
const tempDir = mkdtempSync(join(os.tmpdir(), "discobot-vz-image-"));
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

	if (arch === "arm64") {
		const vmlinuzPath = join(resourcesDir, "vmlinuz");
		const vmlinuxPath = join(resourcesDir, "vmlinux");
		console.log("Decompressing vmlinuz → vmlinux for arm64...");
		execSync(`gunzip -c "${vmlinuzPath}" > "${vmlinuxPath}"`, {
			cwd: projectRoot,
			stdio: "inherit",
		});
		unlinkSync(vmlinuzPath);
	}

	console.log("VZ image files extracted successfully:");
	for (const file of outputFiles) {
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
	console.error(`Failed to extract VZ image:${extractionHint(error)}`);
	console.error(error instanceof Error ? error.message : String(error));
	process.exit(1);
} finally {
	rmSync(tempDir, { force: true, recursive: true });
}
