export function readStorage(key: string): string | null {
	if (typeof window === "undefined") {
		return null;
	}

	return window.localStorage.getItem(key);
}

export function writeStorage(key: string, value: string | null): void {
	if (typeof window === "undefined") {
		return;
	}

	if (value === null) {
		window.localStorage.removeItem(key);
		return;
	}

	window.localStorage.setItem(key, value);
}
