#!/usr/bin/env node

import { spawnSync } from "node:child_process";
import { mkdirSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = dirname(fileURLToPath(import.meta.url));
const projectRoot = dirname(__dirname);
const agentGoDir = join(projectRoot, "agent-go");
const outputName = process.platform === "win32" ? "disco.exe" : "disco";
const outputPath = join("bin", outputName);

mkdirSync(join(agentGoDir, "bin"), { recursive: true });

const result = spawnSync("go", ["build", "-o", outputPath, "./cmd/agent-api"], {
  cwd: agentGoDir,
  stdio: "inherit",
});

if (result.error) {
  console.error(result.error.message);
  process.exit(1);
}

process.exit(result.status ?? 0);
