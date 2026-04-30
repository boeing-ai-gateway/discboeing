#!/usr/bin/env node
import { execFileSync } from "node:child_process";
import { readdirSync } from "node:fs";
import { dirname, extname, join } from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = dirname(fileURLToPath(import.meta.url));
const projectRoot = join(__dirname, "..");

if (process.platform === "win32") {
	console.log("Skipping shellcheck on Windows");
	process.exit(0);
}

const shellFiles = [];

function collectShellFiles(dir) {
	for (const entry of readdirSync(dir, { withFileTypes: true })) {
		if (entry.name === ".git" || entry.name === "node_modules") {
			continue;
		}

		const fullPath = join(dir, entry.name);
		if (entry.isDirectory()) {
			collectShellFiles(fullPath);
			continue;
		}
		if (entry.isFile() && extname(entry.name) === ".sh") {
			shellFiles.push(fullPath);
		}
	}
}

collectShellFiles(projectRoot);

if (shellFiles.length === 0) {
	process.exit(0);
}

execFileSync("shellcheck", shellFiles, {
	cwd: projectRoot,
	stdio: "inherit",
});
