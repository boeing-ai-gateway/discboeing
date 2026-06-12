import { describe, expect, test } from "vitest";

import type { Session } from "$lib/api-types";
import type { Context } from "$lib/context/context.types";
import { createSessionRecord } from "$lib/context/domains/sessions";
import { shouldLoadSessionToolbar } from "$lib/shell-selectors";
import {
	createInitialDataState,
	createInitialViewState,
} from "$lib/context/initial-state";

function createContext(): Context {
	return {
		data: createInitialDataState(),
		view: createInitialViewState(),
		commands: {} as Context["commands"],
	};
}

describe("ng shell selectors", () => {
	test("loads the selected session toolbar", () => {
		const context = createContext();
		context.view.selection.sessionId = "session-1";

		expect(shouldLoadSessionToolbar(context, "session-1")).toBe(true);
	});

	test("keeps non-stopped mounted sessions loadable", () => {
		const context = createContext();
		context.data.sessions.byId["session-1"] = createSessionRecord("session-1", {
			id: "session-1",
			sandboxStatus: "ready",
		} as Session);

		expect(shouldLoadSessionToolbar(context, "session-1")).toBe(true);
	});

	test("does not load a stopped unselected session toolbar", () => {
		const context = createContext();
		context.data.sessions.byId["session-1"] = createSessionRecord("session-1", {
			id: "session-1",
			sandboxStatus: "stopped",
		} as Session);

		expect(shouldLoadSessionToolbar(context, "session-1")).toBe(false);
	});
});
