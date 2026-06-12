import { describe, expect, test } from "vitest";

import type {
	ListSessionFilesResponse,
	SessionDiffFilesResponse,
	SessionDiffResponse,
} from "$lib/api-types";
import { createSessionRecord } from "$lib/context/domains/sessions";
import {
	applyDiffSnapshotToRecord,
	applyDiffStatusSnapshotToRecord,
} from "$lib/context/domains/diff";
import { applyFileSubtreeSnapshotToRecord } from "$lib/context/domains/files";

describe("session snapshot cache updates", () => {
	test("keeps full diff references when a snapshot is unchanged", () => {
		const record = createSessionRecord("session-1");
		const diff = {
			files: [
				{
					path: "src/app.ts",
					status: "modified",
					additions: 1,
					deletions: 2,
					binary: false,
					patch: "@@ patch",
				},
			],
			stats: { filesChanged: 1, additions: 1, deletions: 2 },
		} satisfies SessionDiffResponse;

		applyDiffSnapshotToRecord(record, diff);
		const value = record.diff.value;
		const status = record.diff.status;

		applyDiffSnapshotToRecord(record, structuredClone(diff));

		expect(record.diff.value).toBe(value);
		expect(record.diff.status).toBe(status);
	});

	test("updates full diff references when a snapshot changes", () => {
		const record = createSessionRecord("session-1");
		const diff = {
			files: [],
			stats: { filesChanged: 0, additions: 0, deletions: 0 },
		} satisfies SessionDiffResponse;
		const changedDiff = {
			files: [
				{
					path: "src/app.ts",
					status: "modified",
					additions: 1,
					deletions: 0,
					binary: false,
					patch: "@@ patch",
				},
			],
			stats: { filesChanged: 1, additions: 1, deletions: 0 },
		} satisfies SessionDiffResponse;

		applyDiffSnapshotToRecord(record, diff);
		const value = record.diff.value;

		applyDiffSnapshotToRecord(record, changedDiff);

		expect(record.diff.value).toBe(changedDiff);
		expect(record.diff.value).not.toBe(value);
	});

	test("keeps diff file status references when a snapshot is unchanged", () => {
		const record = createSessionRecord("session-1");
		const diff = {
			files: [{ path: "src/app.ts", status: "modified" }],
			stats: { filesChanged: 1, additions: 1, deletions: 2 },
		} satisfies SessionDiffFilesResponse;

		applyDiffStatusSnapshotToRecord(record, diff);
		const files = record.diff.files;
		const status = record.diff.filesStatus;

		applyDiffStatusSnapshotToRecord(record, structuredClone(diff));

		expect(record.diff.files).toBe(files);
		expect(record.diff.filesStatus).toBe(status);
	});

	test("keeps file subtree status when a snapshot is unchanged", () => {
		const record = createSessionRecord("session-1");
		const response = {
			path: ".",
			entries: [
				{ name: "README.md", type: "file", size: 12 },
				{ name: "src", type: "directory" },
			],
		} satisfies ListSessionFilesResponse;

		applyFileSubtreeSnapshotToRecord(record, response);
		const rootNode = record.files.nodesByPath[""];
		const readmeNode = record.files.nodesByPath["README.md"];
		const status = record.files.statusBySubtree[""];

		applyFileSubtreeSnapshotToRecord(record, structuredClone(response));

		expect(record.files.nodesByPath[""]).toBe(rootNode);
		expect(record.files.nodesByPath["README.md"]).toBe(readmeNode);
		expect(record.files.statusBySubtree[""]).toBe(status);
	});
});
