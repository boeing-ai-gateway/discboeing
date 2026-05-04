import assert from "node:assert/strict";
import { mkdtemp, mkdir, readFile, stat, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { basename, join } from "node:path";
import test from "node:test";

import type { CommandRunner } from "../vz-watcher/watcher.js";
import { WslWatcher } from "./index.js";

test("doBuild publishes a digest-named archive and rotates it when content changes", async () => {
	const rootDir = await mkdtemp(join(tmpdir(), "discobot-wsl-watcher-"));
	const envFilePath = join(rootDir, "server", ".env");
	const outputDir = join(rootDir, "build", "wsl");

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
			const destination = join(rootDir, args[2]);
			await writeFile(
				destination,
				buildCount === 1 ? "rootfs-v1" : "rootfs-v2",
				"utf-8",
			);
			return { stdout: "", stderr: "", exitCode: 0 };
		}
		return { stdout: "", stderr: "", exitCode: 0 };
	};

	const watcher = new WslWatcher(rootDir, envFilePath, outputDir, mockRunner);
	await watcher.doBuild();

	const firstEnvContent = await readFile(envFilePath, "utf-8");
	const firstPath = firstEnvContent.match(/^WSL_ROOTFS_ARCHIVE_PATH=(.+)$/m)?.[1];
	assert.ok(firstPath, "expected first WSL rootfs path to be written to .env");
	assert.match(
		basename(firstPath),
		/^discobot-rootfs-[0-9a-f]{12}\.tar\.zst$/,
		"expected first rootfs archive filename to include a digest",
	);
	await stat(firstPath);
	await assert.rejects(stat(join(outputDir, "discobot-rootfs.tar.zst")));

	await watcher.doBuild();

	const secondEnvContent = await readFile(envFilePath, "utf-8");
	const secondPath = secondEnvContent.match(/^WSL_ROOTFS_ARCHIVE_PATH=(.+)$/m)?.[1];
	assert.ok(secondPath, "expected second WSL rootfs path to be written to .env");
	assert.notEqual(
		firstPath,
		secondPath,
		"expected changed rootfs content to change the archive path",
	);
	await stat(secondPath);
	await assert.rejects(stat(firstPath));
	assert.equal(buildCount, 2);

	const buildCalls = calls.filter((call) => call.args[0] === "build");
	assert.equal(buildCalls.length, 2, "expected one docker build per watcher build");
	for (const call of buildCalls) {
		assert.equal(call.command, "docker");
		assert.equal(call.cwd, rootDir);
		assert.deepEqual(call.args.slice(0, 4), ["build", "--target", "wsl-image", "-t"]);
		assert.match(call.args[4] ?? "", /^discobot-wsl-watcher-extract:/);
		assert.equal(call.args[5], ".");
	}

	const createCalls = calls.filter((call) => call.args[0] === "create");
	assert.equal(createCalls.length, 2, "expected one docker create per watcher build");
	assert.deepEqual(
		createCalls.map((call) => call.args.slice(1)),
		buildCalls.map((call) => [call.args[4], "/__discobot_artifact__"]),
		"expected docker create to use the image built for each watcher build with an explicit artifact-only command",
	);

	const cpCalls = calls.filter((call) => call.args[0] === "cp");
	assert.equal(cpCalls.length, 2, "expected one docker cp per watcher build");
	for (const [index, call] of cpCalls.entries()) {
		assert.equal(call.command, "docker");
		assert.equal(call.cwd, rootDir);
		assert.equal(call.args[1], `container-${index + 1}:/discobot-rootfs.tar.zst`);
		assert.equal(
			call.args[2],
			"build/wsl/discobot-rootfs.tar.zst",
			"expected docker cp to use a project-relative path for WSL-routed Docker",
		);
	}

	assert.equal(
		calls.filter((call) => call.args[0] === "rm").length,
		2,
		"expected temporary containers to be removed after each build",
	);
	assert.equal(
		calls.filter((call) => call.args[0] === "rmi").length,
		2,
		"expected temporary images to be removed after each build",
	);
});
