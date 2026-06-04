import { api } from "$lib/api-client";
import type { WorkspaceValidationResult } from "$lib/api-types";
import { getCommandContext } from "$lib/context/commands";
import {
	refreshCredentialsData,
	refreshWorkspacesData,
} from "$lib/app/app-runtime.svelte";

export async function renameWorkspace(
	workspaceId: string,
	displayName: string,
): Promise<void> {
	const workspace = await api.updateWorkspace(workspaceId, {
		displayName: displayName.trim() || null,
	});
	const context = getCommandContext();
	context.data.workspaces.byId[workspace.id] = workspace;
	context.data.workspaces.items = context.data.workspaces.items.map((item) =>
		item.id === workspace.id ? workspace : item,
	);
}

export async function deleteWorkspace(workspaceId: string): Promise<void> {
	await api.deleteWorkspace(workspaceId);
	const context = getCommandContext();
	context.data.workspaces.items = context.data.workspaces.items.filter(
		(workspace) => workspace.id !== workspaceId,
	);
	delete context.data.workspaces.byId[workspaceId];
}

export async function refreshWorkspaces(): Promise<void> {
	await refreshWorkspacesData();
}

export async function validateWorkspace(
	path: string,
	sourceType: "local" | "git",
): Promise<WorkspaceValidationResult> {
	return api.validateWorkspace({ path, sourceType });
}

export async function refreshCredentials(): Promise<void> {
	await refreshCredentialsData();
}
