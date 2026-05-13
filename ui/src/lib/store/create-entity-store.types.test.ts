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

type ListOnlyHasAll = Assert<HasKey<ListOnlyStore, "all">>;
type ListOnlyHasMergeList = Assert<HasKey<ListOnlyStore, "mergeList">>;
type ListOnlyHasNoGet = Assert<Not<HasKey<ListOnlyStore, "get">>>;
type ListOnlyHasNoCreate = Assert<Not<HasKey<ListOnlyStore, "create">>>;

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

type IndexedHasGet = Assert<HasKey<IndexedStore, "get">>;
type IndexedHasPeek = Assert<HasKey<IndexedStore, "peek">>;
type IndexedHasUpsert = Assert<HasKey<IndexedStore, "upsert">>;
type IndexedHasNoUpdate = Assert<Not<HasKey<IndexedStore, "update">>>;

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
		run: async (id: string): Promise<void> => {
			void id;
		},
	},
} satisfies CreateEntityStoreArgs<
	Item,
	string,
	{ name: string },
	{ name: string }
>;

type CrudStore = EntityStoreFromArgs<Item, string, typeof crudArgs>;

type CrudHasCreate = Assert<HasKey<CrudStore, "create">>;
type CrudHasUpdate = Assert<HasKey<CrudStore, "update">>;
type CrudHasRemove = Assert<HasKey<CrudStore, "remove">>;

void listOnlyArgs;
void indexedArgs;
void crudArgs;

export type StoreTypeAssertions = [
	ListOnlyHasAll,
	ListOnlyHasMergeList,
	ListOnlyHasNoGet,
	ListOnlyHasNoCreate,
	IndexedHasGet,
	IndexedHasPeek,
	IndexedHasUpsert,
	IndexedHasNoUpdate,
	CrudHasCreate,
	CrudHasUpdate,
	CrudHasRemove,
];
