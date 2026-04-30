import { spawn } from "node:child_process";
import os from "node:os";

const selectedPlatform =
  process.env.DISCOBOT_DEV_RUNTIME_PLATFORM || os.platform();
const shouldPrintCommand = process.argv.includes("--print-command");
const isWindows = process.platform === "win32";

function getRuntimeScript(platform) {
  switch (platform) {
    case "darwin":
      return "dev:vz";
    case "win32":
      return "dev:wsl";
    default:
      return "";
  }
}

const runtimeScript = getRuntimeScript(selectedPlatform);

if (shouldPrintCommand) {
  console.log(runtimeScript || "none");
  process.exit(0);
}

if (!runtimeScript) {
  console.log(
    `[dev-runtime] No platform runtime watcher for ${selectedPlatform}`,
  );
  process.exit(0);
}

console.log(`[dev-runtime] Starting ${runtimeScript} for ${selectedPlatform}`);

const command = isWindows ? process.env.ComSpec || "cmd.exe" : "pnpm";
const args = isWindows
  ? ["/d", "/s", "/c", "pnpm", runtimeScript]
  : [runtimeScript];
const child = spawn(command, args, {
  stdio: "inherit",
  env: { ...process.env },
});

child.on("error", (err) => {
  console.error(
    `[dev-runtime] Failed to start ${runtimeScript}: ${err.message}`,
  );
  process.exit(1);
});

child.on("exit", (code, signal) => {
  if (signal) {
    process.kill(process.pid, signal);
    return;
  }
  process.exit(code ?? 0);
});
