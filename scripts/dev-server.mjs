import { execSync, spawn } from "node:child_process";
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
	return process.env.SERVER_LOG_PATH ?? join(defaultStateHome(), "discobot", "logs", "server.log");
}

const serverLogPath = resolveServerLogPath();
mkdirSync(dirname(serverLogPath), { recursive: true });
const serverLog = createWriteStream(serverLogPath, { flags: "w" });

if (os.platform() === "win32") {
	const binariesDir = join(projectRoot, "src-tauri", "binaries");
	const targetTriple =
		os.arch() === "arm64" ? "aarch64-pc-windows-msvc" : "x86_64-pc-windows-msvc";
	const helperPath = join(binariesDir, `discobot-wsl-helper-${targetTriple}.exe`);
	mkdirSync(binariesDir, { recursive: true });
	execSync(`go build -o "${helperPath}" ./cmd/wsl-helper`, {
		cwd: serverDir,
		stdio: "inherit",
	});
}

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
