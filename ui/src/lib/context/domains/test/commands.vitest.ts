import type { AgentCommand } from "$lib/api-types";
import { expect, test } from "vitest";

import { isUiAgentCommand } from "$lib/agent-command-helpers";

test("ui agent commands require discobot ui metadata", () => {
	const uiCommand = createCommand("ui", { discobot: { ui: true } });
	const falseUiCommand = createCommand("hidden", { discobot: { ui: false } });
	const missingUiCommand = createCommand("missing", { discobot: {} });
	const missingDiscobotCommand = createCommand("plain");

	expect(
		[
			uiCommand,
			falseUiCommand,
			missingUiCommand,
			missingDiscobotCommand,
			undefined,
		].filter(isUiAgentCommand),
	).toEqual([uiCommand]);
});

function createCommand(
	name: string,
	overrides: Partial<AgentCommand> = {},
): AgentCommand {
	return {
		name,
		description: `${name} command`,
		kind: "custom",
		...overrides,
	};
}
