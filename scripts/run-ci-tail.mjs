#!/usr/bin/env node
import { execFileSync } from "node:child_process";

if (process.env.CI !== "true") {
	process.exit(0);
}

if (process.platform === "win32") {
	execFileSync("cmd.exe", ["/d", "/s", "/c", "pnpm build:frontend"], {
		stdio: "inherit",
	});
} else {
	execFileSync("pnpm", ["build:frontend"], { stdio: "inherit" });
}
execFileSync("git", ["diff", "--exit-code"], { stdio: "inherit" });
