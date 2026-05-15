import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import path from "node:path";
import test from "node:test";

const ENTITY_STORE_FILE = path.resolve(
	import.meta.dirname,
	"./create-entity-store.svelte.ts",
);

function readEntityStoreSource() {
	return readFileSync(ENTITY_STORE_FILE, "utf-8");
}

test("createEntityStore composes createResource for list and per-item SWR state", () => {
	const source = readEntityStoreSource();

	assert.match(
		source,
		/import \{ createResource \} from "\.\.\/resource\/create-resource\.svelte";/,
	);
	assert.match(source, /const itemResources = new SvelteMap</);
	assert.match(
		source,
		/const itemStates = new SvelteMap<TId, EntityItemState<TItem>>\(\);/,
	);
	assert.match(source, /const listResource = createResource<TItem\[]>\(\{/);
	assert.match(source, /owner: `\$\{args\.owner\}:list`,/);
	assert.match(source, /createEmptyValue: \(\) => \[\],/);
	assert.match(source, /staleAfterMs: args\.list\.cache\?\.staleAfterMs,/);
	assert.match(source, /retry: args\.list\.cache\?\.retry,/);
	assert.match(
		source,
		/syncItemResourcesFromList\(items, \{ markFresh: true, clearMissing: true \}\);/,
	);
});

test("createEntityStore keeps indexed item resources synchronized with collection updates", () => {
	const source = readEntityStoreSource();

	assert.match(source, /function syncItemResourcesFromList\(/);
	assert.match(
		source,
		/for \(const \[id, resource\] of itemResources\) \{[\s\S]*resource\.setData\(nextItem, \{ markFresh: options\.markFresh \}\);/,
	);
	assert.match(source, /function setListInternal\(/);
	assert.match(
		source,
		/listResource\.setData\(items, \{ markFresh: options\.markFresh \}\);/,
	);
	assert.match(source, /syncItemResourcesFromList\(items, options\);/);
	assert.match(source, /function mergeListInternal\(/);
	assert.match(source, /mergeByKey\(current, items, getIndexedKey\)/);
	assert.match(
		source,
		/itemResources\.get\(id\)\?\.setData\(item, \{[\s\S]*markFresh: options\.markFresh,[\s\S]*\}\);/,
	);
	assert.match(source, /function evictInternal\(id: TId/);
	assert.match(
		source,
		/itemResources\.get\(id\)\?\.setData\(null, \{ markFresh: options\.markFresh \}\);/,
	);
});

test("createEntityStore builds per-item resources with indexed and list-backed loading paths", () => {
	const source = readEntityStoreSource();

	assert.match(source, /function createItemResource\(id: TId\) \{/);
	assert.match(
		source,
		/const resource = createResource<TItem \| null>\(\{[\s\S]*owner: `\$\{args\.owner\}:item:\$\{String\(id\)\}`,/,
	);
	assert.match(
		source,
		/if \(!args\.indexed\?\.load\) \{[\s\S]*const items = await listResource\.ensure\(\);[\s\S]*return findInList\(id, items\);/,
	);
	assert.match(
		source,
		/const item = await args\.indexed\.load\(id\);[\s\S]*mergeListInternal\(\[item\], \{ markFresh: true \}\);[\s\S]*return item;/,
	);
	assert.match(
		source,
		/if \(args\.indexed\?\.isNotFoundError\?\.\(error\)\) \{[\s\S]*if \(args\.indexed\.notFound === "evict"\) \{[\s\S]*evictInternal\(id, \{ markFresh: true \}\);[\s\S]*\} else \{[\s\S]*resource\.setData\(null, \{ markFresh: true \}\);/,
	);
	assert.match(
		source,
		/if \(listResource\.fetchedAt !== null\) \{[\s\S]*const cached = findInList\(id\);[\s\S]*if \(cached !== null\) \{[\s\S]*resource\.setData\(cached, \{ markFresh: true \}\);/,
	);
});

test("createEntityStore exposes stable list and item state views that read through createResource getters", () => {
	const source = readEntityStoreSource();

	assert.match(source, /const allState: EntityListState<TItem> = \{/);
	assert.match(source, /get list\(\) \{[\s\S]*return listResource\.data;/);
	assert.match(source, /ensure: \(\) => listResource\.ensure\(\),/);
	assert.match(source, /refresh: \(\) => listResource\.refresh\(\),/);
	assert.match(source, /invalidate: \(\) => listResource\.invalidate\(\),/);
	assert.match(source, /all: \(\) => allState,/);
	assert.match(
		source,
		/invalidateAll: \(\) => \{[\s\S]*listResource\.invalidate\(\);[\s\S]*for \(const resource of itemResources\.values\(\)\) \{[\s\S]*resource\.invalidate\(\);/,
	);
	assert.match(source, /const existingState = itemStates\.get\(id\);/);
	assert.match(source, /itemStates\.set\(id, state\);/);
	assert.match(source, /get item\(\) \{[\s\S]*return resource\.data;/);
	assert.match(source, /ensure: \(\) => resource\.ensure\(\),/);
	assert.match(source, /refresh: \(\) => resource\.refresh\(\),/);
	assert.match(source, /invalidate: \(\) => resource\.invalidate\(\),/);
	assert.match(
		source,
		/peek\(id: TId\) \{[\s\S]*if \(itemResources\.has\(id\)\) \{[\s\S]*return itemResources\.get\(id\)!\.peek\(\);[\s\S]*\}[\s\S]*return findInList\(id\);/,
	);
});

test("createEntityStore conditionally wires create, update, and remove cache reconciliation policies", () => {
	const source = readEntityStoreSource();

	assert.match(source, /if \(args\.create\) \{/);
	assert.match(source, /switch \(args\.create!\.after \?\? "merge"\) \{/);
	assert.match(
		source,
		/case "merge": \{[\s\S]*create\.after "merge" requires indexed\.getKey or create\.getKey[\s\S]*mergeListInternal\(\[item\], \{ markFresh: true \}\);/,
	);
	assert.match(source, /case "refresh-list":/);
	assert.match(source, /await listResource\.refresh\(\);/);
	assert.match(
		source,
		/case "refresh-item": \{[\s\S]*create\.after "refresh-item" requires indexed\.getKey or create\.getKey[\s\S]*await refreshIndexedItem\(getKey\(item\)\);/,
	);

	assert.match(source, /if \(args\.update\) \{/);
	assert.match(source, /switch \(args\.update!\.after \?\? "merge"\) \{/);
	assert.match(
		source,
		/case "merge":[\s\S]*mergeListInternal\(\[item\], \{ markFresh: true \}\);[\s\S]*getItemResource\(id\)\.setData\(item, \{ markFresh: true \}\);/,
	);
	assert.match(
		source,
		/case "refresh-item":[\s\S]*await refreshIndexedItem\(id\);/,
	);

	assert.match(source, /if \(args\.remove\) \{/);
	assert.match(source, /switch \(args\.remove!\.after \?\? "evict"\) \{/);
	assert.match(
		source,
		/case "evict":[\s\S]*evictInternal\(id, \{ markFresh: true \}\);/,
	);
	assert.match(
		source,
		/case "refresh-list":[\s\S]*await listResource\.refresh\(\);/,
	);
	assert.match(
		source,
		/return store as EntityStoreFromArgs<TItem, TId, TArgs>;/,
	);
});
