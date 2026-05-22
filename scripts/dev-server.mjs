import { spawn } from "node:child_process";
import { createWriteStream, mkdirSync } from "node:fs";
import os from "node:os";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = dirname(fileURLToPath(import.meta.url));
const projectRoot = dirname(__dirname);
const serverDir = join(projectRoot, "server");
const config = os.platform() === "win32" ? ".air.windows.toml" : ".air.toml";

function defaultStateHome() {
  if (process.env.XDG_STATE_HOME) {
    return process.env.XDG_STATE_HOME;
  }
  if (os.platform() === "win32") {
    return process.env.LOCALAPPDATA ?? join(os.homedir(), "AppData", "Local");
  }
  return join(os.homedir(), ".local", "state");
}

function resolveServerLogPath() {
  return (
    process.env.SERVER_LOG_PATH ??
    join(defaultStateHome(), "discobot", "logs", "server.log")
  );
}

const serverLogPath = resolveServerLogPath();
mkdirSync(dirname(serverLogPath), { recursive: true });
const serverLog = createWriteStream(serverLogPath, { flags: "w" });

console.log(`[dev-server] writing server log to ${serverLogPath}`);

const child = spawn(
  "go",
  ["run", "github.com/air-verse/air@latest", "-c", config],
  {
    cwd: serverDir,
    stdio: ["inherit", "pipe", "pipe"],
    env: {
      ...process.env,
      SERVER_LOG_PATH: serverLogPath,
      SUGGESTIONS_ENABLED: "true",
    },
  },
);

child.stdout.on("data", (chunk) => {
  process.stdout.write(chunk);
  serverLog.write(chunk);
});

child.stderr.on("data", (chunk) => {
  process.stderr.write(chunk);
  serverLog.write(chunk);
});

child.on("error", (err) => {
  console.error(`[dev-server] failed to start: ${err.message}`);
  serverLog.end();
  process.exit(1);
});

child.on("exit", (code, signal) => {
  serverLog.end();
  if (signal) {
    process.kill(process.pid, signal);
    return;
  }
  process.exit(code ?? 0);
});
