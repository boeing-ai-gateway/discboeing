#!/usr/bin/env npx tsx
/**
 * HCS Image Watcher - Entry point
 *
 * Watches the Dockerfile, ./hcs, and ./vm-assets directories for changes and
 * automatically rebuilds the HCS guest artifact image, extracts the Windows HCS
 * resources, and updates server/.env for local Windows sandbox development.
 *
 * Usage: npx tsx scripts/hcs-watcher/index.ts
 */

import { watch } from "node:fs";
import { access, constants, mkdir, readdir, unlink } from "node:fs/promises";
import { basename, dirname, join } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

import {
  buildDockerTargetFiles,
  defaultRunCommand,
  type CommandRunner,
  updateEnvFile,
} from "../vz-watcher/watcher.js";

const hcsArtifacts = [
  "discboeing-rootfs.vhd",
  "wsl-kernel",
  "kernel-version",
  "wsl-kernel-ref",
  "HcsLinuxVmLauncher.exe",
  "gvproxy.exe",
  "gvforwarder",
] as const;

type HcsArtifactName = (typeof hcsArtifacts)[number];

export const hcsWatchDirectories = ["hcs", "vm-assets"] as const;

interface Logger {
  log: (message: string) => void;
  error: (message: string) => void;
  success: (message: string) => void;
}

function createLogger(): Logger {
  return {
    log: (message: string) => {
      const timestamp = new Date().toISOString().slice(11, 19);
      console.log(`\x1b[36m[hcs-watcher ${timestamp}]\x1b[0m ${message}`);
    },
    error: (message: string) => {
      const timestamp = new Date().toISOString().slice(11, 19);
      console.error(`\x1b[31m[hcs-watcher ${timestamp}]\x1b[0m ${message}`);
    },
    success: (message: string) => {
      const timestamp = new Date().toISOString().slice(11, 19);
      console.log(`\x1b[32m[hcs-watcher ${timestamp}]\x1b[0m ${message}`);
    },
  };
}

function shouldIgnorePath(filename: string | null): boolean {
  if (!filename) return true;
  return (
    filename.includes("node_modules") ||
    filename.startsWith(".") ||
    filename.includes("/.") ||
    filename.includes("\\.")
  );
}

async function fileExists(filePath: string): Promise<boolean> {
  try {
    await access(filePath, constants.F_OK);
    return true;
  } catch {
    return false;
  }
}

export class HcsWatcher {
  private buildInProgress = false;
  private pendingBuild = false;
  private debounceTimer: ReturnType<typeof setTimeout> | null = null;
  private readonly logger = createLogger();
  private readonly runCommand: CommandRunner;

  constructor(
    private readonly projectRoot: string,
    private readonly envFilePath: string,
    private readonly outputDir: string,
    runCommand: CommandRunner = defaultRunCommand,
  ) {
    this.runCommand = runCommand;
  }

  async buildImage(): Promise<boolean> {
    this.logger.log(
      "Building HCS artifact image (docker build --target hcs-image)...",
    );
    await mkdir(this.outputDir, { recursive: true });

    try {
      await buildDockerTargetFiles({
        runCommand: this.runCommand,
        projectRoot: this.projectRoot,
        target: "hcs-image",
        temporaryTagPrefix: "discboeing-hcs-watcher-extract",
        artifacts: hcsArtifacts.map((name) => ({
          source: `/${name}`,
          destination: join(this.outputDir, name),
        })),
      });
    } catch (error) {
      this.logger.error("HCS artifact image build failed:");
      this.logger.error(error instanceof Error ? error.message : String(error));
      return false;
    }

    this.logger.success("HCS artifact image build succeeded");
    return true;
  }

  async verifyArtifacts(): Promise<
    Record<HcsArtifactName, string> | undefined
  > {
    const paths = Object.fromEntries(
      hcsArtifacts.map((name) => [name, join(this.outputDir, name)]),
    ) as Record<HcsArtifactName, string>;

    for (const [name, artifactPath] of Object.entries(paths)) {
      if (!(await fileExists(artifactPath))) {
        this.logger.error(`HCS artifact ${name} not found at ${artifactPath}`);
        return undefined;
      }
    }

    return paths;
  }

  async cleanupStaleArtifacts(
    currentArtifacts: Record<HcsArtifactName, string>,
  ): Promise<void> {
    const currentNames = new Set(
      Object.values(currentArtifacts).map((artifactPath) =>
        basename(artifactPath),
      ),
    );
    const entries = await readdir(this.outputDir, { withFileTypes: true });

    await Promise.all(
      entries.map(async (entry) => {
        if (!entry.isFile() || currentNames.has(entry.name)) {
          return;
        }
        if (entry.name.endsWith(".tmp")) {
          await unlink(join(this.outputDir, entry.name));
        }
      }),
    );
  }

  async updateEnv(
    artifacts: Record<HcsArtifactName, string>,
  ): Promise<boolean> {
    const success = await updateEnvFile(this.envFilePath, {
      HCS_LAUNCHER_PATH: artifacts["HcsLinuxVmLauncher.exe"],
      HCS_KERNEL_PATH: artifacts["wsl-kernel"],
      HCS_ROOT_DISK_PATH: artifacts["discboeing-rootfs.vhd"],
    });

    if (success) {
      this.logger.success(
        `Updated ${this.envFilePath}:\n` +
          `  HCS_LAUNCHER_PATH=${artifacts["HcsLinuxVmLauncher.exe"]}\n` +
          `  HCS_KERNEL_PATH=${artifacts["wsl-kernel"]}\n` +
          `  HCS_ROOT_DISK_PATH=${artifacts["discboeing-rootfs.vhd"]}`,
      );
    } else {
      this.logger.error(`Failed to write ${this.envFilePath}`);
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

    try {
      if (!(await this.buildImage())) {
        return;
      }

      const artifacts = await this.verifyArtifacts();
      if (!artifacts) {
        return;
      }

      if (await this.updateEnv(artifacts)) {
        await this.cleanupStaleArtifacts(artifacts);
      }
      this.logger.log(
        "HCS artifacts ready. Restart server to use the new HCS image.",
      );
    } catch (err) {
      this.logger.error(`Build failed: ${err}`);
    } finally {
      this.buildInProgress = false;
      if (this.pendingBuild) {
        this.logger.log("Processing pending build request...");
        await this.doBuild();
      }
    }
  }

  scheduleBuild(): void {
    if (this.debounceTimer) {
      clearTimeout(this.debounceTimer);
    }
    if (this.buildInProgress) {
      this.pendingBuild = true;
    }
    this.debounceTimer = setTimeout(() => {
      this.debounceTimer = null;
      void this.doBuild();
    }, 500);
  }

  async start(): Promise<void> {
    this.logger.log("Starting HCS artifact watcher...");

    for (const watchDirectory of hcsWatchDirectories) {
      const watchDir = join(this.projectRoot, watchDirectory);
      await access(watchDir, constants.F_OK);
      this.logger.log(`Watching ${watchDir} for changes`);
      watch(watchDir, { recursive: true }, (eventType, filename) => {
        if (shouldIgnorePath(filename)) return;
        this.logger.log(
          `Change detected in ${watchDirectory}: ${filename} (${eventType})`,
        );
        this.scheduleBuild();
      });
    }

    this.logger.log(`Watching ${this.projectRoot} for Dockerfile changes`);
    watch(this.projectRoot, (eventType, filename) => {
      if (filename !== "Dockerfile") return;
      this.logger.log(`Dockerfile changed (${eventType})`);
      this.scheduleBuild();
    });

    this.logger.log("Performing initial build...");
    await this.doBuild();

    const shutdown = () => {
      this.logger.log("Shutting down...");
      process.exit(0);
    };
    process.on("SIGINT", shutdown);
    process.on("SIGTERM", shutdown);
    this.logger.log("Watcher ready. Press Ctrl+C to stop.");
  }
}

const __dirname = dirname(fileURLToPath(import.meta.url));
const ROOT_DIR = join(__dirname, "../..");
const SERVER_ENV_PATH = join(ROOT_DIR, "server", ".env");
const OUTPUT_DIR = join(ROOT_DIR, "build", "hcs");

if (
  process.argv[1] &&
  import.meta.url === pathToFileURL(process.argv[1]).href
) {
  const watcher = new HcsWatcher(ROOT_DIR, SERVER_ENV_PATH, OUTPUT_DIR);
  watcher.start().catch((err) => {
    if (err instanceof Error && /ENOENT/.test(err.message)) {
      console.error(`Fatal error: missing watch path (${err.message})`);
      process.exit(1);
    }
    console.error(`Fatal error: ${err}`);
    process.exit(1);
  });
}
