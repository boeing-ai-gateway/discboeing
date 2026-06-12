import type { AgentCommand } from "$lib/api-types";

export function isUiAgentCommand(
	command: AgentCommand | undefined,
): command is AgentCommand {
	return command?.discobot?.ui === true;
}
