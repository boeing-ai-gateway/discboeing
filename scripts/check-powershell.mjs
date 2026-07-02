#!/usr/bin/env node
import { spawnSync } from "node:child_process";
import {
	existsSync,
	mkdtempSync,
	readdirSync,
	rmSync,
	statSync,
	writeFileSync,
} from "node:fs";
import { tmpdir } from "node:os";
import { dirname, extname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = dirname(fileURLToPath(import.meta.url));
const projectRoot = join(__dirname, "..");
const validModes = new Set(["check", "fix", "format", "lint"]);
const mode = process.argv[2] ?? "check";

if (!validModes.has(mode)) {
	console.error("usage: node scripts/check-powershell.mjs [check|fix|format|lint]");
	process.exit(1);
}

if (process.platform !== "win32") {
	console.log(`Skipping PowerShell ${mode} on non-Windows`);
	process.exit(0);
}

const powerShell = findPowerShell();
if (powerShell === null) {
	console.error("PowerShell was not found on PATH");
	process.exit(1);
}

const files = [];
const requestedFiles = process.argv.slice(3);
if (requestedFiles.length > 0) {
	collectRequestedPowerShellFiles(requestedFiles);
} else if (!collectGitPowerShellFiles()) {
	collectPowerShellFiles(projectRoot);
}

if (files.length === 0) {
	console.log("No PowerShell files found");
	process.exit(0);
}

const tempDir = mkdtempSync(join(tmpdir(), "discboeing-powershell-"));
const fileListPath = join(tempDir, "files.json");
writeFileSync(fileListPath, JSON.stringify(files), "utf8");

let exitCode = 1;
try {
	const result = spawnSync(
		powerShell,
		[
			"-NoLogo",
			"-NoProfile",
			"-ExecutionPolicy",
			"Bypass",
			"-File",
			join(__dirname, "check-powershell.ps1"),
			"-Mode",
			mode,
			"-FileListPath",
			fileListPath,
			"-ProjectRoot",
			projectRoot,
		],
		{
			cwd: projectRoot,
			stdio: "inherit",
		},
	);

	if (result.error) {
		throw result.error;
	}

	exitCode = result.status ?? 1;
} finally {
	rmSync(tempDir, { force: true, recursive: true });
}

process.exit(exitCode);

function findPowerShell() {
	for (const command of ["pwsh.exe", "pwsh", "powershell.exe", "powershell"]) {
		const result = spawnSync(
			command,
			["-NoLogo", "-NoProfile", "-Command", "$PSVersionTable.PSVersion.ToString()"],
			{ stdio: "ignore" },
		);
		if (!result.error && result.status === 0) {
			return command;
		}
	}

	return null;
}

function collectRequestedPowerShellFiles(requestedFiles) {
	const seen = new Set();
	for (const requestedFile of requestedFiles) {
		const fullPath = resolve(projectRoot, requestedFile);
		if (!existsSync(fullPath) || seen.has(fullPath)) {
			continue;
		}

		const stat = statSync(fullPath);
		if (stat.isFile() && extname(fullPath).toLowerCase() === ".ps1") {
			seen.add(fullPath);
			files.push(fullPath);
		}
	}
}

function collectGitPowerShellFiles() {
	const result = spawnSync(
		"git",
		["ls-files", "--cached", "--", "*.ps1"],
		{
			cwd: projectRoot,
			encoding: "utf8",
		},
	);

	if (result.error || result.status !== 0) {
		return false;
	}

	for (const line of result.stdout.split(/\r?\n/)) {
		const file = line.trim();
		if (file !== "") {
			files.push(resolve(projectRoot, file));
		}
	}

	return true;
}

function collectPowerShellFiles(dir) {
	for (const entry of readdirSync(dir, { withFileTypes: true })) {
		if (entry.name === ".git" || entry.name === "node_modules") {
			continue;
		}

		const fullPath = join(dir, entry.name);
		if (entry.isDirectory()) {
			collectPowerShellFiles(fullPath);
			continue;
		}
		if (entry.isFile() && extname(entry.name).toLowerCase() === ".ps1") {
			files.push(fullPath);
		}
	}
}
