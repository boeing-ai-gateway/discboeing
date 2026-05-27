export function pathToUri(root: string, path: string): string {
	const cleanRoot = root.replace(/\/+$/, "");
	const cleanPath = path.replace(/^\/+/, "");
	return `file://${cleanRoot}/${cleanPath}`;
}

export function uriToPath(root: string, uri: string): string {
	const prefix = `file://${root.replace(/\/+$/, "")}/`;
	return uri.startsWith(prefix) ? uri.slice(prefix.length) : uri;
}
