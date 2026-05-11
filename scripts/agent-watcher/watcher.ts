/**
 * Agent Watcher Module
 *
 * Core logic for watching the agent directory and root Dockerfile,
 * triggering Docker builds when changes are detected.
 */

import { spawn } from "node:child_process";
import { existsSync, type FSWatcher, watch } from "node:fs";
import { access, constants, readFile, writeFile } from "node:fs/promises";
import { delimiter, dirname, join, win32 } from "node:path";

export interface WatcherConfig {
	/** Primary agent directory (agent-go) */
	agentDir: string;
	/** Additional directories to watch (e.g., agent init process) */
	additionalDirs?: string[];
	/** Project root directory (where Dockerfile lives) */
	projectRoot: string;
	envFilePath: string;
	imageName: string;
	imageTag: string;
	dockerCommand?: string;
	debounceMs: number;
	/** Optional: Dockerfile build target stage (e.g., "runtime-shell") */
	buildTarget?: string;
	/** Optional: remote image repository to tag and push for remote sandboxes */
	remoteImageRepository?: string;
	/** Optional: custom command runner for testing */
	runCommand?: CommandRunner;
	/** Optional: custom logger */
	logger?: Logger;
}

export interface CommandResult {
	stdout: string;
	stderr: string;
	exitCode: number;
}

export interface BuildImageResult {
	localImageRef: string;
	remoteImageRef?: string;
}

export type CommandRunner = (
	command: string,
	args: string[],
	cwd: string,
) => Promise<CommandResult>;

export interface Logger {
	log: (message: string) => void;
	error: (message: string) => void;
	success: (message: string) => void;
}

function isWindowsStylePath(path: string): boolean {
	return /^[A-Za-z]:[\\/]/.test(path);
}

/**
 * Resolves the Docker CLI command.
 * On Windows, prefer Docker Desktop's standard install location so a stale
 * PATH in the current shell does not break the watcher.
 */
export function resolveDockerCommand(
	platform = process.platform,
	env: NodeJS.ProcessEnv = process.env,
	fileExists: (path: string) => boolean = existsSync,
): string {
	if (platform !== "win32") {
		return "docker";
	}

	const configuredDocker = env.DOCKER_EXE?.trim();
	if (configuredDocker) {
		return configuredDocker;
	}

	for (const baseDir of [env.ProgramFiles, env.ProgramW6432]) {
		if (!baseDir) {
			continue;
		}
		const dockerPath = win32.join(
			baseDir,
			"Docker",
			"Docker",
			"resources",
			"bin",
			"docker.exe",
		);
		if (fileExists(dockerPath)) {
			return dockerPath;
		}
	}

	return "docker";
}

export function getCommandEnv(
	command: string,
	env: NodeJS.ProcessEnv = process.env,
): NodeJS.ProcessEnv {
	const pathModule = isWindowsStylePath(command) ? win32 : null;
	const commandDir = pathModule
		? pathModule.dirname(command)
		: dirname(command);
	if (commandDir === ".") {
		return env;
	}

	const pathDelimiter = pathModule ? win32.delimiter : delimiter;
	const currentPath = env.PATH ?? env.Path ?? "";
	const pathEntries = currentPath
		.split(pathDelimiter)
		.map((entry) => entry.trim())
		.filter(Boolean);
	if (pathEntries.includes(commandDir)) {
		return env;
	}

	return {
		...env,
		PATH: [commandDir, currentPath].filter(Boolean).join(pathDelimiter),
	};
}

function isDockerCommand(command: string): boolean {
	const baseName = command.split(/[/\\]/).at(-1)?.toLowerCase() ?? "";
	return baseName === "docker" || baseName === "docker.exe";
}

export function imageRepositoryFromRef(
	imageRef: string | undefined,
): string | undefined {
	const trimmed = imageRef?.trim();
	if (!trimmed) {
		return undefined;
	}

	const withoutDigest = trimmed.split("@", 1)[0];
	const lastSlash = withoutDigest.lastIndexOf("/");
	const lastColon = withoutDigest.lastIndexOf(":");
	if (lastColon > lastSlash) {
		return withoutDigest.slice(0, lastColon);
	}
	return withoutDigest;
}

export function resolveCommandInvocation(
	command: string,
	args: string[],
	platform = process.platform,
	env: NodeJS.ProcessEnv = process.env,
): { command: string; args: string[] } {
	const distro = env.DISCOBOT_DOCKER_WSL_DISTRO?.trim();
	if (platform === "win32" && distro && isDockerCommand(command)) {
		return {
			command: "wsl.exe",
			args: ["-d", distro, "--", "docker", ...args],
		};
	}
	return { command, args };
}

/** Default command runner using child_process.spawn */
export async function defaultRunCommand(
	command: string,
	args: string[],
	cwd: string,
): Promise<CommandResult> {
	return new Promise((resolve) => {
		const invocation = resolveCommandInvocation(command, args);
		const proc = spawn(invocation.command, invocation.args, {
			cwd,
			env: getCommandEnv(invocation.command),
			stdio: ["pipe", "pipe", "pipe"],
		});

		let stdout = "";
		let stderr = "";

		proc.stdout.on("data", (data) => {
			stdout += data.toString();
		});

		proc.stderr.on("data", (data) => {
			stderr += data.toString();
		});

		proc.on("close", (code) => {
			resolve({ stdout, stderr, exitCode: code ?? 1 });
		});

		proc.on("error", (err) => {
			stderr += err.message;
			resolve({ stdout, stderr, exitCode: 1 });
		});
	});
}

/** Default console logger with colors */
export function createDefaultLogger(): Logger {
	return {
		log: (message: string) => {
			const timestamp = new Date().toISOString().slice(11, 19);
			console.log(`\x1b[36m[agent-watcher ${timestamp}]\x1b[0m ${message}`);
		},
		error: (message: string) => {
			const timestamp = new Date().toISOString().slice(11, 19);
			console.error(`\x1b[31m[agent-watcher ${timestamp}]\x1b[0m ${message}`);
		},
		success: (message: string) => {
			const timestamp = new Date().toISOString().slice(11, 19);
			console.log(`\x1b[32m[agent-watcher ${timestamp}]\x1b[0m ${message}`);
		},
	};
}

/**
 * Updates an env file with the SANDBOX_IMAGE variables.
 * Creates the file if it doesn't exist.
 * Replaces existing variables if present, otherwise appends.
 */
export async function updateEnvFile(
	envFilePath: string,
	imageRef: string,
	remoteImageRef?: string,
): Promise<boolean> {
	let envContent = "";

	try {
		await access(envFilePath, constants.F_OK);
		envContent = await readFile(envFilePath, "utf-8");
	} catch {
		// File doesn't exist, create it
		envContent = "";
	}

	const lines = envContent.split("\n");
	let foundSandboxImage = false;
	let foundSandboxImageRemote = false;
	const newLines = lines.map((line) => {
		if (line.startsWith("SANDBOX_IMAGE=")) {
			foundSandboxImage = true;
			return `SANDBOX_IMAGE=${imageRef}`;
		}
		if (remoteImageRef && line.startsWith("SANDBOX_IMAGE_REMOTE=")) {
			foundSandboxImageRemote = true;
			return `SANDBOX_IMAGE_REMOTE=${remoteImageRef}`;
		}
		return line;
	});

	if (!foundSandboxImage || (remoteImageRef && !foundSandboxImageRemote)) {
		// Remove trailing empty lines and add the new var
		while (newLines.length > 0 && newLines[newLines.length - 1] === "") {
			newLines.pop();
		}
		if (!foundSandboxImage) {
			newLines.push(`SANDBOX_IMAGE=${imageRef}`);
		}
		if (remoteImageRef && !foundSandboxImageRemote) {
			newLines.push(`SANDBOX_IMAGE_REMOTE=${remoteImageRef}`);
		}
		newLines.push(""); // End with newline
	}

	const newContent = newLines.join("\n");

	try {
		await writeFile(envFilePath, newContent, "utf-8");
		return true;
	} catch {
		return false;
	}
}

/**
 * Checks if a path should be ignored by the watcher.
 */
export function shouldIgnorePath(filename: string | null): boolean {
	if (!filename) return true;
	return (
		filename.includes("node_modules") ||
		filename.startsWith(".") ||
		filename.includes("/.")
	);
}

export class AgentWatcher {
	private config: WatcherConfig;
	private runCommand: CommandRunner;
	public logger: Logger;
	private dockerCommand: string;
	private watchers: FSWatcher[] = [];
	private dockerfileWatcher: FSWatcher | null = null;
	private buildInProgress = false;
	private pendingBuild = false;
	private debounceTimer: ReturnType<typeof setTimeout> | null = null;

	/** Event callbacks for testing */
	public onBuildStart?: () => void;
	public onBuildComplete?: (success: boolean, imageRef: string | null) => void;
	public onEnvUpdate?: (imageRef: string) => void;
	public onFileChange?: (filename: string, eventType: string) => void;

	constructor(config: WatcherConfig) {
		this.config = config;
		this.runCommand = config.runCommand ?? defaultRunCommand;
		this.logger = config.logger ?? createDefaultLogger();
		this.dockerCommand = config.dockerCommand ?? resolveDockerCommand();
	}

	get imageRef(): string {
		return `${this.config.imageName}:${this.config.imageTag}`;
	}

	async checkAgentDirExists(): Promise<boolean> {
		try {
			await access(this.config.agentDir, constants.F_OK);
			return true;
		} catch {
			return false;
		}
	}

	async buildImage(): Promise<BuildImageResult | null> {
		this.logger.log(`Building Docker image ${this.imageRef}...`);

		const result = await this.runCommand(
			this.dockerCommand,
			[
				"build",
				...(this.config.buildTarget
					? ["--target", this.config.buildTarget]
					: []),
				"-t",
				this.imageRef,
				".",
			],
			this.config.projectRoot,
		);

		if (result.exitCode !== 0) {
			this.logger.error("Docker build failed:");
			this.logger.error(result.stderr || result.stdout);
			return null;
		}

		this.logger.success("Docker build succeeded");

		// Get the image ID to use as a stable tag
		const inspectResult = await this.runCommand(
			this.dockerCommand,
			["inspect", "--format={{.Id}}", this.imageRef],
			this.config.projectRoot,
		);

		if (inspectResult.exitCode !== 0) {
			this.logger.error("Failed to inspect image:");
			this.logger.error(inspectResult.stderr || inspectResult.stdout);
			return null;
		}

		// Extract the first 8 characters of the image ID (after "sha256:")
		const imageId = inspectResult.stdout.trim();
		const shortId = imageId.replace(/^sha256:/, "").slice(0, 8);

		// Create a tag with discobot-local/ prefix using the short image ID
		// This allows the image to be recognized as a local build and ensures
		// the tag is stable for the same image content
		const localImageRef = `discobot-local/${this.config.imageName}:${shortId}`;

		// Tag the image with the ID-based reference
		const tagResult = await this.runCommand(
			this.dockerCommand,
			["tag", this.imageRef, localImageRef],
			this.config.projectRoot,
		);

		if (tagResult.exitCode !== 0) {
			this.logger.error("Failed to tag image:");
			this.logger.error(tagResult.stderr || tagResult.stdout);
			return null;
		}

		this.logger.log(`Tagged as: ${localImageRef}`);
		const remoteRepository = this.config.remoteImageRepository?.trim();
		if (!remoteRepository) {
			return { localImageRef };
		}

		const remoteImageRef = `${remoteRepository}:${shortId}`;
		const remoteTagResult = await this.runCommand(
			this.dockerCommand,
			["tag", localImageRef, remoteImageRef],
			this.config.projectRoot,
		);
		if (remoteTagResult.exitCode !== 0) {
			this.logger.error("Failed to tag remote image:");
			this.logger.error(remoteTagResult.stderr || remoteTagResult.stdout);
			return null;
		}

		this.logger.log(`Tagged remote image as: ${remoteImageRef}`);
		const pushResult = await this.runCommand(
			this.dockerCommand,
			["push", remoteImageRef],
			this.config.projectRoot,
		);
		if (pushResult.exitCode !== 0) {
			this.logger.error("Failed to push remote image:");
			this.logger.error(pushResult.stderr || pushResult.stdout);
			return null;
		}

		this.logger.success(`Pushed remote image: ${remoteImageRef}`);
		return { localImageRef, remoteImageRef };
	}

	async updateEnv(imageRef: string, remoteImageRef?: string): Promise<boolean> {
		const success = await updateEnvFile(
			this.config.envFilePath,
			imageRef,
			remoteImageRef,
		);
		if (success) {
			this.logger.success(
				`Updated ${this.config.envFilePath} with SANDBOX_IMAGE=${imageRef}`,
			);
			if (remoteImageRef) {
				this.logger.success(
					`Updated ${this.config.envFilePath} with SANDBOX_IMAGE_REMOTE=${remoteImageRef}`,
				);
			}
			this.onEnvUpdate?.(imageRef);
		} else {
			this.logger.error(`Failed to write ${this.config.envFilePath}`);
		}
		return success;
	}

	async doBuild(): Promise<void> {
		if (this.buildInProgress) {
			this.pendingBuild = true;
			return;
		}

		this.buildInProgress = true;
		this.pendingBuild = false;
		this.onBuildStart?.();

		try {
			const buildResult = await this.buildImage();
			if (!buildResult) {
				this.onBuildComplete?.(false, null);
				return;
			}

			// Use the image ID (sha256 digest) for deterministic container creation
			await this.updateEnv(
				buildResult.localImageRef,
				buildResult.remoteImageRef,
			);

			this.logger.log(
				"Image ready. Server will use the new image for new containers.",
			);
			this.onBuildComplete?.(true, buildResult.localImageRef);
		} finally {
			this.buildInProgress = false;

			if (this.pendingBuild) {
				this.logger.log("Processing pending build request...");
				this.doBuild();
			}
		}
	}

	scheduleBuild(): void {
		if (this.debounceTimer) {
			clearTimeout(this.debounceTimer);
		}

		// If a build is in progress, mark that we need to rebuild when it completes.
		// This ensures changes aren't missed if the debounce timer hasn't fired yet
		// when the current build finishes.
		if (this.buildInProgress) {
			this.pendingBuild = true;
		}

		this.debounceTimer = setTimeout(() => {
			this.debounceTimer = null;
			this.doBuild();
		}, this.config.debounceMs);
	}

	async start(): Promise<void> {
		this.logger.log("Starting agent image watcher...");

		if (!(await this.checkAgentDirExists())) {
			this.logger.error(`Agent directory not found: ${this.config.agentDir}`);
			throw new Error(`Agent directory not found: ${this.config.agentDir}`);
		}

		// Collect all directories to watch
		const dirsToWatch = [this.config.agentDir];
		if (this.config.additionalDirs) {
			dirsToWatch.push(...this.config.additionalDirs);
		}

		for (const dir of dirsToWatch) {
			this.logger.log(`Watching ${dir} for changes`);
		}

		if (this.dockerCommand !== "docker") {
			this.logger.log(`Using Docker CLI at ${this.dockerCommand}`);
		}

		// Do an initial build
		this.logger.log("Performing initial build...");
		await this.doBuild();

		// Watch for changes in all directories
		for (const dir of dirsToWatch) {
			const watcher = watch(dir, { recursive: true }, (eventType, filename) => {
				if (shouldIgnorePath(filename)) {
					return;
				}

				this.logger.log(`Change detected: ${filename} (${eventType})`);
				this.onFileChange?.(filename ?? "", eventType);
				this.scheduleBuild();
			});

			watcher.on("error", (err) => {
				this.logger.error(`Watcher error for ${dir}: ${err}`);
			});

			watcher.on("close", () => {
				this.logger.error(`Watcher for ${dir} closed unexpectedly!`);
			});

			this.watchers.push(watcher);
		}

		// Watch for changes to Dockerfile at project root
		// Note: We watch the directory instead of the file directly because
		// fs.watch() on a single file is unreliable on many platforms (especially WSL2).
		// The watcher can silently stop working after certain filesystem operations.
		this.logger.log(
			`Watching ${this.config.projectRoot} for Dockerfile changes`,
		);

		this.dockerfileWatcher = watch(
			this.config.projectRoot,
			(eventType, filename) => {
				if (filename !== "Dockerfile") {
					return;
				}
				this.logger.log(`Dockerfile changed (${eventType})`);
				this.onFileChange?.(filename, eventType);
				this.scheduleBuild();
			},
		);

		this.dockerfileWatcher.on("error", (err) => {
			this.logger.error(`Dockerfile watcher error: ${err}`);
		});

		this.dockerfileWatcher.on("close", () => {
			this.logger.error("Dockerfile watcher closed unexpectedly!");
		});

		// Handle graceful shutdown
		const shutdown = () => {
			this.logger.log("Shutting down...");
			this.stop();
			process.exit(0);
		};

		process.on("SIGINT", shutdown);
		process.on("SIGTERM", shutdown);

		this.logger.log("Watcher ready. Press Ctrl+C to stop.");
	}

	stop(): void {
		if (this.debounceTimer) {
			clearTimeout(this.debounceTimer);
			this.debounceTimer = null;
		}
		for (const watcher of this.watchers) {
			watcher.close();
		}
		this.watchers = [];
		if (this.dockerfileWatcher) {
			this.dockerfileWatcher.close();
			this.dockerfileWatcher = null;
		}
	}
}
