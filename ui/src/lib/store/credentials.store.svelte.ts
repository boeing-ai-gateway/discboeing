import { api } from "$lib/api-client";
import type {
	CredentialAuthType,
	CredentialEnvVar,
	CredentialInfo,
	CredentialType,
} from "$lib/api-types";
import { createResource } from "$lib/resource/create-resource.svelte";
import type { AsyncStatus } from "$lib/shell-types";

import {
	createEntityStore,
	type CreateEntityStoreArgs,
} from "./create-entity-store.svelte";
import { RequestCoalescer } from "./request-coalescer";

const credentialStoreResourceArgs = {
	owner: "CredentialStore",
	list: {
		load: async () => {
			const { credentials } = await api.getCredentials();
			return credentials;
		},
	},
	indexed: {
		getKey: (credential: CredentialInfo) => credential.id,
		load: (id: string) => api.getCredential(id),
	},
	create: {
		run: (data: {
			provider?: string;
			credentialId?: string;
			name: string;
			description?: string;
			authType: CredentialAuthType;
			apiKey?: string;
			envVars?: CredentialEnvVar[];
			agentVisible?: boolean;
			visibility?: import("$lib/api-types").CredentialVisibility;
			inactive?: boolean;
		}) => api.createCredential(data),
		after: "merge",
	},
} satisfies CreateEntityStoreArgs<
	CredentialInfo,
	string,
	{
		provider?: string;
		credentialId?: string;
		name: string;
		description?: string;
		authType: CredentialAuthType;
		apiKey?: string;
		envVars?: CredentialEnvVar[];
		agentVisible?: boolean;
		visibility?: import("$lib/api-types").CredentialVisibility;
		inactive?: boolean;
	}
>;

export class CredentialStore {
	#resource = createEntityStore<
		CredentialInfo,
		string,
		typeof credentialStoreResourceArgs
	>(credentialStoreResourceArgs);
	#credentialTypesResource = createResource<CredentialType[]>({
		owner: "CredentialStore:types",
		enabled: () => true,
		createEmptyValue: () => [],
		load: async () => {
			const { credentialTypes } = await api.getCredentialTypes();
			return credentialTypes;
		},
	});
	#fetchRequests = new RequestCoalescer<"list">();

	get list(): CredentialInfo[] {
		return this.#resource.all().list;
	}

	get credentialTypes(): CredentialType[] {
		return this.#credentialTypesResource.data;
	}

	get status(): AsyncStatus {
		return this.#resource.all().status;
	}

	/** Returns the cached credential without side effects. */
	peek(idOrProvider: string): CredentialInfo | null {
		return (
			this.#resource.peek(idOrProvider) ??
			this.#resource
				.all()
				.list.find((credential) => credential.provider === idOrProvider) ??
			null
		);
	}

	/** Returns the cached credential and triggers a background fetch of the full list on cache miss. */
	ensure(idOrProvider: string): CredentialInfo | null {
		const cached = this.peek(idOrProvider);
		if (cached === null && this.#resource.all().status === "idle") {
			void this.fetch();
		}
		return cached;
	}

	async fetch(): Promise<void> {
		await this.#fetchRequests.run("list", async () => {
			await Promise.all([
				this.#resource.all().refresh(),
				this.#credentialTypesResource.refresh(),
			]);
		});
	}

	invalidate(): void {
		this.#resource.invalidateAll();
		this.#credentialTypesResource.invalidate();
	}

	async save(data: {
		provider?: string;
		credentialId?: string;
		name: string;
		description?: string;
		authType: CredentialAuthType;
		apiKey?: string;
		envVars?: CredentialEnvVar[];
		agentVisible?: boolean;
		visibility?: import("$lib/api-types").CredentialVisibility;
		inactive?: boolean;
	}): Promise<CredentialInfo> {
		return this.#resource.create(data);
	}

	async getOne(identifier: string): Promise<CredentialInfo> {
		return api.getCredential(identifier);
	}

	async remove(identifier: string): Promise<void> {
		try {
			await api.deleteCredential(identifier);
			const cached = this.peek(identifier);
			if (cached) {
				this.#resource.evict(cached.id);
			}
			return;
		} catch (error) {
			await this.fetch();
			const stillExists = this.list.some(
				(credential) =>
					credential.id === identifier || credential.provider === identifier,
			);
			if (!stillExists) {
				return;
			}
			throw error;
		}
	}
}
