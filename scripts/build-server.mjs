import { execSync } from "node:child_process";
import { cpSync, mkdirSync, readFileSync, rmSync } from "node:fs";
import os from "node:os";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = dirname(fileURLToPath(import.meta.url));
const projectRoot = dirname(__dirname);
const serverDir = join(projectRoot, "server");
const uiBuildDir = join(projectRoot, "ui", "build");
const embeddedUIDir = join(serverDir, "static", "ui", "dist");
const binariesDir = join(projectRoot, "electron", "binaries");
const packageJSON = JSON.parse(
  readFileSync(join(projectRoot, "package.json"), "utf-8"),
);

function getGitValue(command) {
  try {
    return execSync(command, {
      cwd: projectRoot,
      stdio: ["ignore", "pipe", "ignore"],
      encoding: "utf-8",
    }).trim();
  } catch {
    return "";
  }
}

// Get version from environment (CI) or default to the package version for local builds.
// In CI, DISCOBOT_VERSION may be set from the git tag (for example, "0.1.0-12").
const version = process.env.DISCOBOT_VERSION || packageJSON.version;

const gitCommit = process.env.GITHUB_SHA || getGitValue("git rev-parse HEAD");
const gitShortCommit = gitCommit ? gitCommit.slice(0, 12) : "";
const gitTag =
  process.env.GITHUB_REF_TYPE === "tag"
    ? process.env.GITHUB_REF_NAME || ""
    : process.env.DISCOBOT_GIT_TAG ||
      getGitValue("git describe --tags --exact-match");
const sentryRelease = `discobot@${version}${gitShortCommit ? `+${gitShortCommit}` : ""}`;

const uiBuildEnv = {
  ...process.env,
  PUBLIC_SENTRY_APP_VERSION: process.env.PUBLIC_SENTRY_APP_VERSION || version,
  PUBLIC_SENTRY_RELEASE: process.env.PUBLIC_SENTRY_RELEASE || sentryRelease,
  PUBLIC_SENTRY_DIST: process.env.PUBLIC_SENTRY_DIST || "electron",
  PUBLIC_SENTRY_GIT_COMMIT: process.env.PUBLIC_SENTRY_GIT_COMMIT || gitCommit,
  PUBLIC_SENTRY_GIT_TAG: process.env.PUBLIC_SENTRY_GIT_TAG || gitTag,
};

const hasSentryDSN = Boolean(process.env.PUBLIC_SENTRY_DSN);

// GitHub OAuth client ID for git operations (device flow, repo scope).
// Set via DISCOBOT_GITHUB_OAUTH_CLIENT_ID in CI; empty string in dev builds.
const githubOAuthClientID = process.env.DISCOBOT_GITHUB_OAUTH_CLIENT_ID || "";

// Create binaries directory
mkdirSync(binariesDir, { recursive: true });

function syncEmbeddedUI() {
  console.log(
    `Building Svelte UI with Sentry release ${uiBuildEnv.PUBLIC_SENTRY_RELEASE}...`,
  );
  console.log(`Sentry DSN configured: ${hasSentryDSN ? "yes" : "no"}`);
  execSync("pnpm ui:build", {
    cwd: projectRoot,
    stdio: "inherit",
    env: uiBuildEnv,
  });

  console.log("Syncing built UI into server/static/ui/dist...");
  if (os.platform() !== "win32") {
    rmSync(embeddedUIDir, { recursive: true, force: true });
  }
  // Windows can keep open handles on the embedded UI directory when another
  // local server process is running, so update files in place there instead of
  // deleting the directory first.
  mkdirSync(embeddedUIDir, { recursive: true });
  cpSync(uiBuildDir, embeddedUIDir, { recursive: true, force: true });
}

function prepareNextUI() {
  console.log("Preparing Datastar UI assets...");
  execSync(
    "pnpm --dir ./discobot assets:build && pnpm --dir ./discobot generate",
    {
      cwd: projectRoot,
      stdio: "inherit",
    },
  );
}

// Get target triple from environment or detect from current platform
function getTargetTriple() {
  // Use DISCOBOT_TARGET_TRIPLE if set (from CI workflow).
  if (process.env.DISCOBOT_TARGET_TRIPLE) {
    return process.env.DISCOBOT_TARGET_TRIPLE;
  }

  const platform = os.platform();
  const arch = os.arch();

  if (platform === "linux") {
    if (arch === "x64") return "x86_64-unknown-linux-gnu";
    if (arch === "arm64") return "aarch64-unknown-linux-gnu";
  } else if (platform === "darwin") {
    if (arch === "x64") return "x86_64-apple-darwin";
    if (arch === "arm64") return "aarch64-apple-darwin";
  } else if (platform === "win32") {
    if (arch === "x64") return "x86_64-pc-windows-msvc";
    if (arch === "arm64") return "aarch64-pc-windows-msvc";
  }

  throw new Error(`Unsupported platform: ${platform} ${arch}`);
}

const targetTriple = getTargetTriple();
const ext = targetTriple.includes("windows") ? ".exe" : "";
const serverOutputName = `discobot-server-${targetTriple}${ext}`;
const serverOutputPath = join(binariesDir, serverOutputName);
const staleWSLHelperOutputPath = join(
  binariesDir,
  `discobot-wsl-helper-${targetTriple}${ext}`,
);
rmSync(staleWSLHelperOutputPath, { force: true });

// Map target triple to Go cross-compilation env vars
function getGoEnv(triple) {
  const archMap = {
    x86_64: "amd64",
    aarch64: "arm64",
  };
  const osMap = {
    "apple-darwin": "darwin",
    "unknown-linux-gnu": "linux",
    "pc-windows-msvc": "windows",
  };

  const [cpu, ...rest] = triple.split("-");
  const osKey = rest.join("-");

  return {
    GOARCH: archMap[cpu],
    GOOS: osMap[osKey],
  };
}

const goEnv = getGoEnv(targetTriple);
console.log(
  `Building discobot-server ${version} for ${targetTriple} (GOOS=${goEnv.GOOS}, GOARCH=${goEnv.GOARCH})...`,
);

syncEmbeddedUI();
prepareNextUI();

// Build with version and compiled-in client IDs injected via ldflags
const ldflags = [
  `-X github.com/obot-platform/discobot/server/internal/version.Version=${version}`,
  `-X github.com/obot-platform/discobot/server/internal/config.GitHubOAuthClientID=${githubOAuthClientID}`,
].join(" ");
execSync(
  `go build -ldflags "${ldflags}" -o "${serverOutputPath}" ./cmd/server`,
  {
    cwd: serverDir,
    stdio: "inherit",
    env: { ...process.env, ...goEnv },
  },
);

console.log(`Built: ${serverOutputPath} (version: ${version})`);
