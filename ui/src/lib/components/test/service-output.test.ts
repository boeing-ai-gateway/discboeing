import assert from "node:assert/strict";
import test from "node:test";

import { renderServiceOutputText } from "../../service-output";

test("renderServiceOutputText preserves plain text", () => {
	assert.equal(renderServiceOutputText("ready\nwaiting"), "ready\nwaiting");
});

test("renderServiceOutputText strips ANSI styling codes", () => {
	assert.equal(
		renderServiceOutputText(
			"\u001b[1m\u001b[96m   Building\u001b[0m electron-builder v26.0.0",
		),
		"   Building electron-builder v26.0.0",
	);
});

test("renderServiceOutputText applies carriage-return line replacement", () => {
	const raw = [
		"\u001b[1m\u001b[96m   Building\u001b[0m [========>             ] 575/633: electron-builder",
		"\r\u001b[K\u001b[1m\u001b[96m   Building\u001b[0m [===================>  ] 586/633: electron-updater",
		"\r\u001b[K\u001b[1m\u001b[92m   Compiling\u001b[0m discobot v0.1.0",
	].join("");

	assert.equal(renderServiceOutputText(raw), "   Compiling discobot v0.1.0");
});

test("renderServiceOutputText handles carriage-return overwrite without clearing", () => {
	assert.equal(renderServiceOutputText("abcd\rxy"), "xycd");
});
