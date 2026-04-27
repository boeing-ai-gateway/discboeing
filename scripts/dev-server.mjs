import { spawn } from "node:child_process";
import os from "node:os";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = dirname(fileURLToPath(import.meta.url));
const projectRoot = dirname(__dirname);
const serverDir = join(projectRoot, "server");
const config = os.platform() === "win32" ? ".air.windows.toml" : ".air.toml";

const child = spawn(
	"go",
	["run", "github.com/air-verse/air@latest", "-c", config],
	{
		cwd: serverDir,
		stdio: "inherit",
		env: { ...process.env, SUGGESTIONS_ENABLED: "true" },
	},
);

child.on("exit", (code, signal) => {
	if (signal) {
		process.kill(process.pid, signal);
		return;
	}
	process.exit(code ?? 0);
});
