import { api } from "$lib/api-client";
import type { Service } from "$lib/api-types";
import { createResource } from "$lib/resource/create-resource.svelte";
import {
	sortServiceItems,
	toServiceItem,
} from "$lib/session/domains/session-domain.helpers";
import type { SessionServicesDomain } from "$lib/session/session-context.types";

type CreateSessionServicesDomainArgs = {
	sessionId: string;
	hasSession: () => boolean;
	getActiveServiceId: () => string | null;
	openService: (serviceId: string) => void;
};

export function createSessionServicesDomain(
	args: CreateSessionServicesDomainArgs,
): SessionServicesDomain {
	const resource = createResource<Service[]>({
		owner: "SessionServices",
		enabled: () => args.hasSession(),
		createEmptyValue: () => [],
		load: async () => {
			const { services } = await api.getServices(args.sessionId);
			return services;
		},
	});

	$effect(() => {
		if (
			typeof window === "undefined" ||
			!resource.data.some(
				(service) =>
					service.status === "starting" || service.status === "stopping",
			)
		) {
			return;
		}

		const timeout = window.setTimeout(() => {
			void resource.refresh();
		}, 5000);

		return () => {
			window.clearTimeout(timeout);
		};
	});

	const list = $derived(sortServiceItems(resource.data.map(toServiceItem)));
	const active = $derived(
		list.find((service) => service.id === args.getActiveServiceId()) ?? null,
	);

	return {
		get list() {
			return list;
		},
		get active() {
			return active;
		},
		get status() {
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
		open: args.openService,
		start: async (serviceId: string) => {
			if (!args.hasSession()) {
				return;
			}
			await api.startService(args.sessionId, serviceId);
			await resource.refresh();
		},
		stop: async (serviceId: string) => {
			if (!args.hasSession()) {
				return;
			}
			try {
				await api.stopService(args.sessionId, serviceId);
			} finally {
				await resource.refresh();
			}
		},
		refresh: async () => {
			await resource.refresh();
		},
		invalidate: resource.invalidate,
	};
}
