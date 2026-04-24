import type {
	CreateEntityStoreArgs,
	EntityStoreFromArgs,
} from "./create-entity-store.types";

type Assert<T extends true> = T;
type HasKey<T, K extends PropertyKey> = K extends keyof T ? true : false;
type Not<T extends boolean> = T extends true ? false : true;

type Item = {
	id: string;
	name: string;
};

const listOnlyArgs = {
	owner: "ListOnly",
	list: {
		load: async (): Promise<Item[]> => [],
	},
} satisfies CreateEntityStoreArgs<Item>;

type ListOnlyStore = EntityStoreFromArgs<Item, never, typeof listOnlyArgs>;

type _ListOnlyHasAll = Assert<HasKey<ListOnlyStore, "all">>;
type _ListOnlyHasMergeList = Assert<HasKey<ListOnlyStore, "mergeList">>;
type _ListOnlyHasNoGet = Assert<Not<HasKey<ListOnlyStore, "get">>>;
type _ListOnlyHasNoCreate = Assert<Not<HasKey<ListOnlyStore, "create">>>;

const indexedArgs = {
	owner: "Indexed",
	list: {
		load: async (): Promise<Item[]> => [],
	},
	indexed: {
		getKey: (item: Item) => item.id,
	},
} satisfies CreateEntityStoreArgs<Item, string>;

type IndexedStore = EntityStoreFromArgs<Item, string, typeof indexedArgs>;

type _IndexedHasGet = Assert<HasKey<IndexedStore, "get">>;
type _IndexedHasPeek = Assert<HasKey<IndexedStore, "peek">>;
type _IndexedHasUpsert = Assert<HasKey<IndexedStore, "upsert">>;
type _IndexedHasNoUpdate = Assert<Not<HasKey<IndexedStore, "update">>>;

const crudArgs = {
	owner: "Crud",
	list: {
		load: async (): Promise<Item[]> => [],
	},
	indexed: {
		getKey: (item: Item) => item.id,
	},
	create: {
		run: async ({ name }: { name: string }): Promise<Item> => ({
			id: name,
			name,
		}),
	},
	update: {
		run: async (id: string, { name }: { name: string }): Promise<Item> => ({
			id,
			name,
		}),
	},
	remove: {
		run: async (_id: string): Promise<void> => {},
	},
} satisfies CreateEntityStoreArgs<
	Item,
	string,
	{ name: string },
	{ name: string }
>;

type CrudStore = EntityStoreFromArgs<Item, string, typeof crudArgs>;

type _CrudHasCreate = Assert<HasKey<CrudStore, "create">>;
type _CrudHasUpdate = Assert<HasKey<CrudStore, "update">>;
type _CrudHasRemove = Assert<HasKey<CrudStore, "remove">>;

export {};
