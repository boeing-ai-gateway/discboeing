import { expect, test } from "vitest";

import type {
	SessionDiffFilesResponse,
	SessionDiffResponse,
} from "$lib/api-types";
import {
	applyDiffSnapshotToRecord,
	applyDiffStatusSnapshotToRecord,
} from "$lib/context/domains/diff";
import { createSessionRecord } from "$lib/context/domains/sessions";

test("applyDiffSnapshotToRecord treats null files as empty", () => {
	const record = createSessionRecord("session-1");
	const diff = {
		files: null,
		stats: {
			additions: 0,
			deletions: 0,
			filesChanged: 0,
		},
	} as unknown as SessionDiffResponse;

	applyDiffSnapshotToRecord(record, diff);

	expect(record.diff.status.state).toBe("ready");
	expect(record.diff.value?.files).toEqual([]);
});

test("applyDiffStatusSnapshotToRecord treats null files as empty", () => {
	const record = createSessionRecord("session-1");
	const diff = {
		files: null,
		stats: {
			additions: 0,
			deletions: 0,
			filesChanged: 0,
		},
	} as unknown as SessionDiffFilesResponse;

	applyDiffStatusSnapshotToRecord(record, diff);

	expect(record.diff.filesStatus.state).toBe("ready");
	expect(record.diff.files?.files).toEqual([]);
});
