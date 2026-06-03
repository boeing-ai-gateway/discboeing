import type { Context } from "$lib/context/context.types";
import { getContextForCommand } from "$lib/context/context.svelte";

export type CommandContext = Context;

export function getCommandContext(): CommandContext {
	return getContextForCommand();
}
