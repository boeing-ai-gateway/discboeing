#!/usr/bin/env npx tsx
/**
 * WSL Rootfs Watcher - Entry point
 *
 * Watches the Dockerfile and ./vm-assets directory for changes and automatically
 * rebuilds the shared guest image output, verifies the WSL rootfs archive,
 * publishes it under a digest-based filename, and updates server/.env with
 * WSL_ROOTFS_ARCHIVE_PATH for local Windows sandbox development.
 *
 * Usage: npx tsx scripts/wsl-watcher/index.ts
 */

import { createHash } from "node:crypto";
import { createReadStream, watch } from "node:fs";
import {
  access,
  constants,
  mkdir,
  readdir,
  rename,
  unlink,
} from "node:fs/promises";
import { basename, dirname, join } from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";
import {
  buildDockerTargetFiles,
  defaultRunCommand,
  type CommandRunner,
  updateEnvFile,
} from "../vz-watcher/watcher.js";

const rootfsArchiveName = "discobot-rootfs.tar.zst";
const versionedRootfsArchivePattern = /^discobot-rootfs-[0-9a-f]{12}\.tar\.zst$/;

interface Logger {
  log: (message: string) => void;
  error: (message: string) => void;
  success: (message: string) => void;
}

function createLogger(): Logger {
  return {
    log: (message: string) => {
      const timestamp = new Date().toISOString().slice(11, 19);
      console.log(`\x1b[36m[wsl-watcher ${timestamp}]\x1b[0m ${message}`);
    },
    error: (message: string) => {
      const timestamp = new Date().toISOString().slice(11, 19);
      console.error(`\x1b[31m[wsl-watcher ${timestamp}]\x1b[0m ${message}`);
    },
    success: (message: string) => {
      const timestamp = new Date().toISOString().slice(11, 19);
      console.log(`\x1b[32m[wsl-watcher ${timestamp}]\x1b[0m ${message}`);
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

async function computeFileDigest(filePath: string): Promise<string> {
  return await new Promise((resolve, reject) => {
    const hash = createHash("sha256");
    const stream = createReadStream(filePath);

    stream.on("data", (chunk) => {
      hash.update(chunk);
    });
    stream.on("end", () => {
      resolve(hash.digest("hex"));
    });
    stream.on("error", reject);
  });
}

export class WslWatcher {
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
      "Building WSL rootfs artifact (docker build --target wsl-image)...",
    );
    await mkdir(this.outputDir, { recursive: true });

    try {
      await buildDockerTargetFiles({
        runCommand: this.runCommand,
        projectRoot: this.projectRoot,
        target: "wsl-image",
        temporaryTagPrefix: "discobot-wsl-watcher-extract",
        artifacts: [
          {
            source: "/discobot-rootfs.tar.zst",
            destination: join(this.outputDir, rootfsArchiveName),
          },
        ],
      });
    } catch (error) {
      this.logger.error("WSL rootfs build failed:");
      this.logger.error(error instanceof Error ? error.message : String(error));
      return false;
    }

    this.logger.success("WSL rootfs build succeeded");
    return true;
  }

  async updateEnv(rootfsArchivePath: string): Promise<boolean> {
    const success = await updateEnvFile(this.envFilePath, {
      WSL_ROOTFS_ARCHIVE_PATH: rootfsArchivePath,
    });
    if (success) {
      this.logger.success(
        `Updated ${this.envFilePath} with WSL_ROOTFS_ARCHIVE_PATH=${rootfsArchivePath}`,
      );
    } else {
      this.logger.error(`Failed to write ${this.envFilePath}`);
    }
    return success;
  }

  async publishVersionedRootfsArchive(rootfsArchivePath: string): Promise<string> {
    const digest = await computeFileDigest(rootfsArchivePath);
    const versionedRootfsArchivePath = join(
      this.outputDir,
      `discobot-rootfs-${digest.slice(0, 12)}.tar.zst`,
    );

    if (versionedRootfsArchivePath === rootfsArchivePath) {
      return rootfsArchivePath;
    }

    try {
      await access(versionedRootfsArchivePath, constants.F_OK);
      await unlink(rootfsArchivePath);
      this.logger.log(
        `Reusing existing WSL rootfs artifact ${basename(versionedRootfsArchivePath)}`,
      );
    } catch {
      await rename(rootfsArchivePath, versionedRootfsArchivePath);
      this.logger.log(
        `Published WSL rootfs artifact as ${basename(versionedRootfsArchivePath)}`,
      );
    }

    return versionedRootfsArchivePath;
  }

  async cleanupOldRootfsArchives(currentRootfsArchivePath: string): Promise<void> {
    const currentRootfsArchiveName = basename(currentRootfsArchivePath);
    const entries = await readdir(this.outputDir, { withFileTypes: true });

    await Promise.all(
      entries.map(async (entry) => {
        if (
          !entry.isFile() ||
          entry.name === currentRootfsArchiveName ||
          !versionedRootfsArchivePattern.test(entry.name)
        ) {
          return;
        }
        await unlink(join(this.outputDir, entry.name));
      }),
    );
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

      const stagedRootfsArchivePath = join(this.outputDir, rootfsArchiveName);
      try {
        await access(stagedRootfsArchivePath, constants.F_OK);
      } catch {
        this.logger.error(
          `WSL rootfs archive not found at ${stagedRootfsArchivePath}`,
        );
        return;
      }

      const rootfsArchivePath = await this.publishVersionedRootfsArchive(
        stagedRootfsArchivePath,
      );
      if (await this.updateEnv(rootfsArchivePath)) {
        await this.cleanupOldRootfsArchives(rootfsArchivePath);
      }
      this.logger.log(
        "WSL rootfs artifact ready. Restart server to use the new rootfs archive.",
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
    this.logger.log("Starting WSL rootfs watcher...");

    const watchDir = join(this.projectRoot, "vm-assets");
    await access(watchDir, constants.F_OK);
    this.logger.log(`Watching ${watchDir} for changes`);
    watch(watchDir, { recursive: true }, (eventType, filename) => {
      if (shouldIgnorePath(filename)) return;
      this.logger.log(`Change detected: ${filename} (${eventType})`);
      this.scheduleBuild();
    });

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
const OUTPUT_DIR = join(ROOT_DIR, "build", "wsl");

if (
  process.argv[1] &&
  import.meta.url === pathToFileURL(process.argv[1]).href
) {
  const watcher = new WslWatcher(ROOT_DIR, SERVER_ENV_PATH, OUTPUT_DIR);
  watcher.start().catch((err) => {
    if (err instanceof Error && /ENOENT/.test(err.message)) {
      console.error(`Fatal error: missing watch path (${err.message})`);
      process.exit(1);
    }
    console.error(`Fatal error: ${err}`);
    process.exit(1);
  });
}
