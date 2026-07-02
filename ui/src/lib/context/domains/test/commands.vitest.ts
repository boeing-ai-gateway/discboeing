import type { AgentCommand } from "$lib/api-types";
import { expect, test } from "vitest";

import { isUiAgentCommand } from "$lib/agent-command-helpers";

test("ui agent commands require discboeing ui metadata", () => {
	const uiCommand = createCommand("ui", { discboeing: { ui: true } });
	const falseUiCommand = createCommand("hidden", { discboeing: { ui: false } });
	const missingUiCommand = createCommand("missing", { discboeing: {} });
	const missingDiscboeingCommand = createCommand("plain");

	expect(
		[
			uiCommand,
			falseUiCommand,
			missingUiCommand,
			missingDiscboeingCommand,
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
