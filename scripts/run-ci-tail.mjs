#!/usr/bin/env node
import { execFileSync } from "node:child_process";

function commandName(name) {
	return process.platform === "win32" ? `${name}.cmd` : name;
}

if (process.env.CI !== "true") {
	process.exit(0);
}

execFileSync(commandName("pnpm"), ["build:frontend"], { stdio: "inherit" });
execFileSync("git", ["diff", "--exit-code"], { stdio: "inherit" });
