import { api } from "$lib/api-client";
import type { Service } from "$lib/api-types";
import {
	sortServiceItems,
	toServiceItem,
} from "$lib/session/domains/session-domain.helpers";
import type { SessionServicesDomain } from "$lib/session/session-context.types";
import { createEntityStore } from "$lib/store/create-entity-store.svelte";

type CreateSessionServicesDomainArgs = {
	sessionId: string;
	hasSession: () => boolean;
	getActiveServiceId: () => string | null;
	openService: (serviceId: string) => void;
};

export function createSessionServicesDomain(
	args: CreateSessionServicesDomainArgs,
): SessionServicesDomain {
	const store = createEntityStore<Service, string>({
		owner: `SessionServices:${args.sessionId}`,
		enabled: () => args.hasSession(),
		list: {
			load: async () => {
				const { services } = await api.getServices(args.sessionId);
				return services;
			},
		},
		indexed: {
			getKey: (service) => service.id,
		},
	});
	const resource = store.all();

	$effect(() => {
		if (
			typeof window === "undefined" ||
			!resource.list.some(
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

	const list = $derived(sortServiceItems(resource.list.map(toServiceItem)));
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
			try {
				await api.startService(args.sessionId, serviceId);
			} finally {
				await resource.refresh();
			}
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
		bindLocalhost: async (serviceId: string, port: number) => {
			if (!args.hasSession()) {
				return;
			}
			try {
				await api.bindServiceLocalhost(args.sessionId, serviceId, { port });
			} finally {
				await resource.refresh();
			}
		},
		unbindLocalhost: async (serviceId: string) => {
			if (!args.hasSession()) {
				return;
			}
			try {
				await api.unbindServiceLocalhost(args.sessionId, serviceId);
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
