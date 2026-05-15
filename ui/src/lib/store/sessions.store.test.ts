import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import test from "node:test";

function readStoreSource(fileName: string) {
	return readFileSync(path.resolve(import.meta.dirname, fileName), "utf-8");
}

function assertStoreUsesEntityStore(options: {
	fileName: string;
	ownerPattern: RegExp;
	listLoadPattern: RegExp;
	indexedKeyPattern: RegExp;
	indexedLoadPattern?: RegExp;
	createPattern?: RegExp;
	updatePattern?: RegExp;
	removePattern?: RegExp;
	extraPatterns?: RegExp[];
}) {
	const source = readStoreSource(options.fileName);

	assert.match(
		source,
		/import \{[\s\S]*createEntityStore,[\s\S]*type CreateEntityStoreArgs,[\s\S]*\} from "\.\/create-entity-store\.svelte";/,
	);
	assert.match(
		source,
		/const [a-zA-Z]+ = \{[\s\S]*\} satisfies CreateEntityStoreArgs</,
	);
	assert.match(
		source,
		/#resource = createEntityStore<[\s\S]*>\([\s\S]*[a-zA-Z]+,[\s\S]*\);|#resource = createEntityStore<[\s\S]*>\([a-zA-Z]+\);/,
	);
	assert.match(source, options.ownerPattern);
	assert.match(source, options.listLoadPattern);
	assert.match(source, options.indexedKeyPattern);
	if (options.indexedLoadPattern) {
		assert.match(source, options.indexedLoadPattern);
	}
	if (options.createPattern) {
		assert.match(source, options.createPattern);
	}
	if (options.updatePattern) {
		assert.match(source, options.updatePattern);
	}
	if (options.removePattern) {
		assert.match(source, options.removePattern);
	}
	for (const pattern of options.extraPatterns ?? []) {
		assert.match(source, pattern);
	}
}

test("SessionStore delegates entity cache behavior to createEntityStore", () => {
	const source = readStoreSource("sessions.store.svelte.ts");

	assert.match(
		source,
		/import \{[\s\S]*createEntityStore,[\s\S]*type CreateEntityStoreArgs,[\s\S]*\} from "\.\/create-entity-store\.svelte";/,
	);
	assert.match(
		source,
		/const sessionStoreResourceArgs = \{[\s\S]*\} satisfies CreateEntityStoreArgs</,
	);
	assert.match(
		source,
		/#resource = createEntityStore<[\s\S]*typeof sessionStoreResourceArgs[\s\S]*>\(sessionStoreResourceArgs\);/,
	);
	assert.match(
		source,
		/indexed: \{[\s\S]*getKey: \(session(?:: Session)?\) => session\.id,/,
	);
	assert.match(
		source,
		/list: \{[\s\S]*load: async \(\) => \{[\s\S]*api\.getSessions\(\)/,
	);
	assert.match(
		source,
		/indexed: \{[\s\S]*load: \(id(?:: string)?\) => api\.getSession\(id\),/,
	);
	assert.match(source, /notFound: "evict",/);
	assert.match(source, /return this\.#resource\.all\(\)\.list;/);
	assert.match(source, /const cached = this\.#resource\.peek\(id\);/);
	assert.match(source, /void this\.fetchOne\(id\)\.finally/);
	assert.match(source, /this\.#resource\.upsert\(session\);/);
	assert.match(source, /await this\.#resource\.all\(\)\.ensure\(\);/);
	assert.match(
		source,
		/#fetchOneRequests = new RequestCoalescer<string, Session \| null>\(\);/,
	);
	assert.match(source, /const session = await api\.getSession\(id\);/);
	assert.match(source, /this\.#resource\.evict\(id\);/);
	assert.match(source, /return this\.#resource\.create\(data\);/);
	assert.match(source, /return this\.#resource\.update\(id, data\);/);
	assert.match(source, /return this\.#resource\.remove\(id\);/);
});

test("WorkspaceStore delegates entity cache behavior to createEntityStore", () => {
	assertStoreUsesEntityStore({
		fileName: "workspaces.store.svelte.ts",
		ownerPattern: /owner: "WorkspaceStore",/,
		listLoadPattern: /list: \{[\s\S]*api\.getWorkspaces\(\)/,
		indexedKeyPattern:
			/indexed: \{[\s\S]*getKey: \(workspace(?:: Workspace)?\) => workspace\.id,/,
		indexedLoadPattern:
			/indexed: \{[\s\S]*load: \(id(?:: string)?\) => api\.getWorkspace\(id\),/,
		createPattern:
			/create: \{[\s\S]*api\.createWorkspace\(data\)[\s\S]*after: "merge",/,
		updatePattern:
			/update: \{[\s\S]*api\.updateWorkspace\(id, data\)[\s\S]*after: "merge",/,
		extraPatterns: [
			/#fetchOneRequests = new RequestCoalescer<string, Workspace \| null>\(\);/,
			/await this\.#resource\.all\(\)\.ensure\(\);/,
			/this\.#resource\.upsert\(workspace\);/,
			/return this\.#resource\.create\(data\);/,
			/return this\.#resource\.update\(id, data\);/,
			/this\.#resource\.evict\(id\);/,
		],
	});
});

test("ThreadStore delegates entity cache behavior to createEntityStore", () => {
	assertStoreUsesEntityStore({
		fileName: "threads.store.svelte.ts",
		ownerPattern: /owner: `ThreadStore:\$\{args\.sessionId\}`,/,
		listLoadPattern: /list: \{[\s\S]*api\.getThreads\(args\.sessionId\)/,
		indexedKeyPattern:
			/indexed: \{[\s\S]*getKey: \(thread(?:: Thread)?\) => thread\.id,/,
		indexedLoadPattern:
			/indexed: \{[\s\S]*load: \(id(?:: string)?\) => api\.getThread\(args\.sessionId, id\),/,
		createPattern:
			/create: \{[\s\S]*api\.createThread\(args\.sessionId, data\)[\s\S]*after: "merge",/,
		updatePattern:
			/update: \{[\s\S]*api\.updateThread\(args\.sessionId, id, data\)[\s\S]*after: "merge",/,
		removePattern:
			/remove: \{[\s\S]*api\.deleteThread\(args\.sessionId, id\)[\s\S]*after: "evict",/,
		extraPatterns: [
			/enabled: this\.#enabled,/,
			/#backgroundFetches = new SvelteSet<string>\(\);/,
			/await this\.#resource\.all\(\)\.refresh\(\);/,
			/if \(!this\.#enabled\(\)\) \{[\s\S]*return;/,
			/const thread = await api\.getThread\(this\.#sessionId, threadId\);/,
			/this\.#resource\.upsert\(thread\);/,
			/return this\.#resource\.create\(data\);/,
			/return this\.#resource\.update\(threadId, data\);/,
			/return this\.#resource\.remove\(threadId\);/,
		],
	});
});
