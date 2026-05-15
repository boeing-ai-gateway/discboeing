import { api } from "$lib/api-client";
import type { HookOutputResponse, HooksStatusResponse } from "$lib/api-types";
import { createResource } from "$lib/resource/create-resource.svelte";
import {
	mergeHookOutput,
	toHooksStatus,
} from "$lib/session/domains/session-domain.helpers";
import type {
	HookOutputState,
	SessionHooksService,
} from "$lib/session/session-context.types";

const HOOK_STATUS_POLL_MS = 5_000;

type CreateSessionHooksDomainArgs = {
	sessionId: string;
	canLoadSessionData: () => boolean;
};

export function createSessionHooksDomain(
	args: CreateSessionHooksDomainArgs,
): SessionHooksService {
	let outputById = $state<Record<string, HookOutputState>>({});
	let rerunningHookIds = $state<string[]>([]);
	let streamedStatus = $state<HooksStatusResponse | null>(null);
	const resource = createResource<HooksStatusResponse | null>({
		owner: "SessionHooks",
		enabled: args.canLoadSessionData,
		createEmptyValue: () => null,
		load: async () => {
			const nextStatus = await api.getHooksState(args.sessionId);
			if (args.canLoadSessionData()) {
				loadOutputs(nextStatus.outputs);
				streamedStatus = null;
			}
			return nextStatus;
		},
	});

	function getResolvedHooksData() {
		return streamedStatus ?? resource.data;
	}

	function getStatus() {
		const baseStatus = toHooksStatus(getResolvedHooksData());
		if (rerunningHookIds.length === 0) {
			return baseStatus;
		}

		return {
			...baseStatus,
			pendingHookIds: baseStatus.pendingHookIds.filter(
				(hookId) => !rerunningHookIds.includes(hookId),
			),
			hooks: baseStatus.hooks.map((hook) => {
				if (!rerunningHookIds.includes(hook.hookId)) {
					return hook;
				}
				return {
					...hook,
					lastResult: "running" as const,
				};
			}),
		};
	}

	$effect(() => {
		if (
			!args.canLoadSessionData() ||
			resource.isRefreshing ||
			(getStatus().pendingHookIds.length === 0 &&
				!getStatus().hooks.some((hook) => hook.lastResult === "running"))
		) {
			return;
		}

		const timeout = window.setTimeout(() => {
			void refresh();
		}, HOOK_STATUS_POLL_MS);

		return () => {
			window.clearTimeout(timeout);
		};
	});

	function loadOutputs(outputs: Record<string, HookOutputResponse>) {
		outputById = Object.entries(outputs).reduce<
			Record<string, HookOutputState>
		>(
			(nextOutputById, [hookId, response]) =>
				mergeHookOutput(nextOutputById, hookId, response),
			{},
		);
	}

	async function refresh() {
		await resource.refresh();
	}

	async function applyStatusUpdate(nextStatus: HooksStatusResponse) {
		if (!args.canLoadSessionData()) {
			return;
		}
		streamedStatus = nextStatus;
		try {
			const nextState = await api.getHooksState(args.sessionId);
			if (!args.canLoadSessionData()) {
				return;
			}
			streamedStatus = nextState;
			loadOutputs(nextState.outputs);
		} catch (error) {
			console.warn("Failed to load hook state; continuing with status", error);
		}
		resource.invalidate();
	}

	return {
		get status() {
			return getStatus();
		},
		get outputById() {
			void resource.data;
			return outputById;
		},
		get resourceStatus() {
			return resource.status;
		},
		get error() {
			return resource.error;
		},
		get isRefreshing() {
			return resource.isRefreshing;
		},
		get isStale() {
			return resource.isStale;
		},
		get fetchedAt() {
			return resource.fetchedAt;
		},
		rerun: (hookId: string) => {
			if (
				!args.canLoadSessionData() ||
				rerunningHookIds.includes(hookId) ||
				!getStatus().hooks.some((hook) => hook.hookId === hookId)
			) {
				return;
			}
			rerunningHookIds = [...rerunningHookIds, hookId];
			void (async () => {
				try {
					await api.rerunHook(args.sessionId, hookId);
				} finally {
					rerunningHookIds = rerunningHookIds.filter((id) => id !== hookId);
					await refresh();
				}
			})();
		},
		refresh,
		invalidate: resource.invalidate,
		applyStatusUpdate,
	};
}
