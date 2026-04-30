#!/usr/bin/env node
import { execFileSync } from "node:child_process";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = dirname(fileURLToPath(import.meta.url));
const projectRoot = join(__dirname, "..");

const [, , targetDir, ...restArgs] = process.argv;

if (!targetDir) {
	console.error("usage: node scripts/go-test-non-integration.mjs <dir> [-- <go test args...>]");
	process.exit(1);
}

const separatorIndex = restArgs.indexOf("--");
const extraArgs = separatorIndex >= 0 ? restArgs.slice(separatorIndex + 1) : restArgs;
const cwd = join(projectRoot, targetDir);

const packages = execFileSync("go", ["list", "./..."], {
	cwd,
	encoding: "utf8",
})
	.split(/\r?\n/)
	.map((line) => line.trim())
	.filter((line) => line !== "" && !line.includes("/integration"));

if (packages.length === 0) {
	console.error(`no non-integration Go packages found under ${targetDir}`);
	process.exit(1);
}

execFileSync("go", ["test", ...extraArgs, "-v", ...packages], {
	cwd,
	stdio: "inherit",
});
