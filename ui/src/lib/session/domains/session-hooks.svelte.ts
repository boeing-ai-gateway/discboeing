import { api } from "$lib/api-client";
import type { HooksStatusResponse } from "$lib/api-types";
import { createResource } from "$lib/resource/create-resource.svelte";
import {
	mergeHookOutput,
	toHooksStatus,
} from "$lib/session/domains/session-domain.helpers";
import type {
	HookOutputState,
	SessionHooksService,
} from "$lib/session/session-context.types";

type CreateSessionHooksDomainArgs = {
	sessionId: string;
	hasSession: () => boolean;
};

export function createSessionHooksDomain(
	args: CreateSessionHooksDomainArgs,
): SessionHooksService {
	let outputById = $state<Record<string, HookOutputState>>({});
	let rerunningHookIds = $state<string[]>([]);
	let streamedStatus = $state<HooksStatusResponse | null>(null);
	const resource = createResource<HooksStatusResponse | null>({
		owner: "SessionHooks",
		enabled: () => args.hasSession(),
		createEmptyValue: () => null,
		load: async () => {
			const nextStatus = await api.getHooksStatus(args.sessionId);
			await loadOutputs(nextStatus);
			streamedStatus = null;
			return nextStatus;
		},
	});

	const resolvedHooksData = $derived.by(() => streamedStatus ?? resource.data);

	$effect(() => {
		if (args.hasSession()) {
			return;
		}
		streamedStatus = null;
		outputById = {};
		rerunningHookIds = [];
	});

	const status = $derived.by(() => {
		const baseStatus = toHooksStatus(resolvedHooksData);
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
	});

	async function loadOutputs(nextStatus: HooksStatusResponse | null) {
		if (!nextStatus) {
			outputById = {};
			return;
		}

		const outputs = await Promise.allSettled(
			Object.keys(nextStatus.hooks).map(async (hookId) => {
				const response = await api.getHookOutput(args.sessionId, hookId);
				return [hookId, response] as const;
			}),
		);

		outputById = outputs.reduce<Record<string, HookOutputState>>(
			(nextOutputById, result) => {
				if (result.status === "rejected") {
					console.warn(
						"Failed to load hook output; continuing without it",
						result.reason,
					);
					return nextOutputById;
				}
				const [hookId, response] = result.value;
				return mergeHookOutput(nextOutputById, hookId, response);
			},
			{},
		);
	}

	async function refresh() {
		await resource.refresh();
	}

	async function applyStatusUpdate(nextStatus: HooksStatusResponse) {
		if (!args.hasSession()) {
			return;
		}
		streamedStatus = nextStatus;
		await loadOutputs(nextStatus);
		resource.invalidate();
	}

	return {
		get status() {
			return status;
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
			if (!args.hasSession() || rerunningHookIds.includes(hookId)) {
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
