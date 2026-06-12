import assert from "node:assert/strict";
import { test } from "vitest";

import {
	validateWebSearchInput,
	validateWebSearchOutput,
} from "../ai/tool-schemas/websearch-schema";

test("validateWebSearchInput accepts provider open_page actions", () => {
	const result = validateWebSearchInput({
		type: "open_page",
		url: "https://github.com/obot-platform/discobot",
	});

	assert.equal(result.success, true);
	assert.ok(result.data);
	const data = result.data;
	assert.equal(data.type, "open_page");
	assert.equal(data.url, "https://github.com/obot-platform/discobot");
});

test("validateWebSearchOutput preserves provider web search action", () => {
	const result = validateWebSearchOutput({
		type: "web_search_call",
		status: "completed",
		action: {
			type: "open_page",
			url: "https://github.com/obot-platform/discobot",
		},
	});

	assert.equal(result.success, true);
	assert.ok(result.data);
	const data = result.data as {
		type?: string;
		status?: string;
		action?: { type?: string; url?: string };
	};
	assert.equal(data.type, "web_search_call");
	assert.equal(data.status, "completed");
	assert.equal(data.action?.type, "open_page");
	assert.equal(data.action?.url, "https://github.com/obot-platform/discobot");
});
