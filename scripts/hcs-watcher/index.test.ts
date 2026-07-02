import assert from "node:assert/strict";
import { mkdtemp, mkdir, readFile, stat, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";

import type { CommandRunner } from "../vz-watcher/watcher.js";
import { HcsWatcher, hcsWatchDirectories } from "./index.js";

const expectedArtifacts = [
  "discboeing-rootfs.vhd",
  "wsl-kernel",
  "kernel-version",
  "wsl-kernel-ref",
  "HcsLinuxVmLauncher.exe",
  "gvproxy.exe",
  "gvforwarder",
];

test("watches HCS launcher and guest asset inputs", () => {
  assert.deepEqual([...hcsWatchDirectories], ["hcs", "vm-assets"]);
});

test("doBuild extracts HCS artifacts and updates env paths", async () => {
  const rootDir = await mkdtemp(join(tmpdir(), "discboeing-hcs-watcher-"));
  const envFilePath = join(rootDir, "server", ".env");
  const outputDir = join(rootDir, "build", "hcs");

  await mkdir(join(rootDir, "server"), { recursive: true });
  await mkdir(outputDir, { recursive: true });

  let buildCount = 0;
  const calls: Array<{ command: string; args: string[]; cwd: string }> = [];
  const mockRunner: CommandRunner = async (command, args, cwd) => {
    calls.push({ command, args, cwd });
    if (args[0] === "build") {
      buildCount++;
      return { stdout: "", stderr: "", exitCode: 0 };
    }
    if (args[0] === "create") {
      return { stdout: `container-${buildCount}\n`, stderr: "", exitCode: 0 };
    }
    if (args[0] === "cp") {
      const artifactName = (args[1] ?? "").split(":/")[1];
      assert.ok(artifactName, "expected docker cp source artifact");
      await writeFile(
        join(rootDir, args[2]),
        `artifact:${artifactName}`,
        "utf-8",
      );
      return { stdout: "", stderr: "", exitCode: 0 };
    }
    return { stdout: "", stderr: "", exitCode: 0 };
  };

  const watcher = new HcsWatcher(rootDir, envFilePath, outputDir, mockRunner);
  await watcher.doBuild();

  const envContent = await readFile(envFilePath, "utf-8");
  const envLines = Object.fromEntries(
    envContent
      .trim()
      .split("\n")
      .map((line) => line.split("=", 2)),
  );
  assert.equal(
    envLines.HCS_LAUNCHER_PATH,
    join(outputDir, "HcsLinuxVmLauncher.exe"),
  );
  assert.equal(envLines.HCS_KERNEL_PATH, join(outputDir, "wsl-kernel"));
  assert.equal(
    envLines.HCS_ROOT_DISK_PATH,
    join(outputDir, "discboeing-rootfs.vhd"),
  );

  for (const artifact of expectedArtifacts) {
    await stat(join(outputDir, artifact));
  }

  const buildCalls = calls.filter((call) => call.args[0] === "build");
  assert.equal(buildCalls.length, 1, "expected one docker build");
  assert.equal(buildCalls[0].command, "docker");
  assert.equal(buildCalls[0].cwd, rootDir);
  assert.deepEqual(buildCalls[0].args.slice(0, 4), [
    "build",
    "--target",
    "hcs-image",
    "-t",
  ]);
  assert.match(buildCalls[0].args[4] ?? "", /^discboeing-hcs-watcher-extract:/);
  assert.equal(buildCalls[0].args[5], ".");

  const cpCalls = calls.filter((call) => call.args[0] === "cp");
  assert.equal(cpCalls.length, expectedArtifacts.length);
  for (const [index, call] of cpCalls.entries()) {
    assert.equal(call.command, "docker");
    assert.equal(call.cwd, rootDir);
    assert.equal(call.args[1], `container-1:/${expectedArtifacts[index]}`);
    assert.equal(
      call.args[2],
      `build/hcs/${expectedArtifacts[index]}`,
      "expected docker cp to use a project-relative path for WSL-routed Docker",
    );
  }

  assert.equal(
    calls.filter((call) => call.args[0] === "rm").length,
    1,
    "expected temporary container to be removed",
  );
  assert.equal(
    calls.filter((call) => call.args[0] === "rmi").length,
    1,
    "expected temporary image to be removed",
  );
});
