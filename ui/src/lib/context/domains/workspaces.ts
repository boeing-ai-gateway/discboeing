import { api } from "$lib/api-client";
import type { Workspace } from "$lib/api-types";
import type { CollectionCache } from "$lib/context/cache";
import {
	createCollectionCache,
	removeById,
	upsertById,
} from "$lib/context/cache";
import type { Context } from "$lib/context/context.types";

export type WorkspacesState = CollectionCache<Workspace>;

export function createWorkspacesState(): WorkspacesState {
	return createCollectionCache<Workspace>();
}

export function applyWorkspaceSnapshotToCache(
	context: Context,
	workspace: Workspace,
): void {
	upsertById(context.data.workspaces, workspace.id, workspace);
}

export async function renameWorkspace(
	context: Context,
	workspaceId: string,
	displayName: string,
): Promise<void> {
	const workspace = await api.updateWorkspace(workspaceId, {
		displayName: displayName.trim() || null,
	});
	applyWorkspaceSnapshotToCache(context, workspace);
}

export async function deleteWorkspace(
	context: Context,
	workspaceId: string,
): Promise<void> {
	await api.deleteWorkspace(workspaceId);
	removeById(context.data.workspaces, workspaceId);
}
