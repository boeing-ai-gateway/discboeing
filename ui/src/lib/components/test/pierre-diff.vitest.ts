import assert from "node:assert/strict";
import { test } from "vitest";

import {
	buildDiffCacheKey,
	buildDiffFileContents,
	buildWhitespaceIgnoredFileDiff,
	equalIgnoringWhitespace,
	getLanguageFromPath,
	normalizeWhitespaceForDiff,
} from "../../pierre-diff-utils";

test("getLanguageFromPath detects Go files", () => {
	assert.equal(getLanguageFromPath("server/internal/app/service.go"), "go");
});

test("buildDiffCacheKey scopes cache entries by path and language", () => {
	assert.equal(
		buildDiffCacheKey(
			"server/internal/app/service.go",
			"package app",
			"patch-hash",
		),
		"server/internal/app/service.go:go:patch-hash",
	);
	assert.equal(
		buildDiffCacheKey("Dockerfile", "FROM golang:1.26", "patch-hash"),
		"Dockerfile:docker:patch-hash",
	);
});

test("buildDiffFileContents keeps same patch hashes isolated across file types", () => {
	const goFile = buildDiffFileContents(
		"server/internal/app/service.go",
		"package app",
		"shared-patch-hash",
	);
	const tsFile = buildDiffFileContents(
		"src/lib/service.ts",
		"export const value = 1;",
		"shared-patch-hash",
	);

	assert.equal(goFile.lang, "go");
	assert.equal(tsFile.lang, "typescript");
	assert.notEqual(goFile.cacheKey, tsFile.cacheKey);
});

test("buildDiffFileContents falls back to a path-scoped length key without an explicit checksum", () => {
	const file = buildDiffFileContents("src/lib/example.svelte", "<div />", null);

	assert.equal(file.cacheKey, "src/lib/example.svelte:svelte:7");
});

test("normalizeWhitespaceForDiff collapses whitespace-only changes", () => {
	assert.equal(
		normalizeWhitespaceForDiff("\tconst  value =  1;  \n\n  return value;\t"),
		"const value = 1;\n\nreturn value;",
	);
});

test("equalIgnoringWhitespace treats formatting-only edits as unchanged", () => {
	assert.equal(
		equalIgnoringWhitespace(
			"function example() {\n\treturn 1;\n}",
			"function   example()\t{\n  return 1;\n}",
		),
		true,
	);
	assert.equal(
		equalIgnoringWhitespace(
			"function example() {\n\treturn 1;\n}",
			"function example() {\n\treturn 2;\n}",
		),
		false,
	);
});

test("buildWhitespaceIgnoredFileDiff preserves original indentation", () => {
	const oldFile = buildDiffFileContents(
		"src/lib/example.ts",
		"function example() {\n\treturn 1;\n}\n",
		"old",
	);
	const newFile = buildDiffFileContents(
		"src/lib/example.ts",
		"function example() {\n    return 2;\n}\n",
		"new",
	);

	const diff = buildWhitespaceIgnoredFileDiff(oldFile, newFile);

	assert.equal(diff.hunks.length, 1);
	assert.equal(diff.deletionLines[1], "\treturn 1;\n");
	assert.equal(diff.additionLines[1], "    return 2;\n");
});
