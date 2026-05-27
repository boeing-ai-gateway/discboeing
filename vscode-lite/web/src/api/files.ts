export type FileInfo = {
	name: string;
	path: string;
	isDir: boolean;
	size: number;
	modTime: string;
};

export type ListResult = {
	path: string;
	entries: FileInfo[];
};

export type ReadResult = {
	path: string;
	content: string;
	modTime: string;
	size: number;
};

export async function workspace() {
	return getJSON<{ root: string; languages: Record<string, unknown> }>("/api/workspace");
}

export async function listFiles(path = ".") {
	return getJSON<ListResult>(`/api/files/tree?path=${encodeURIComponent(path)}&hidden=false`);
}

export async function readFile(path: string) {
	return getJSON<ReadResult>(`/api/files/content?path=${encodeURIComponent(path)}`);
}

export async function writeFile(path: string, content: string) {
	const response = await fetch("/api/files/content", {
		method: "PUT",
		headers: { "Content-Type": "application/json" },
		body: JSON.stringify({ path, content })
	});
	if (!response.ok) {
		throw new Error(await response.text());
	}
}

async function getJSON<T>(url: string): Promise<T> {
	const response = await fetch(url);
	if (!response.ok) {
		throw new Error(await response.text());
	}
	return (await response.json()) as T;
}
