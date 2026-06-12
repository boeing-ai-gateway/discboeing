function getLocalStorage(): Storage | null {
	if (typeof window === "undefined") {
		return null;
	}

	const storage = window.localStorage;
	if (
		typeof storage?.getItem !== "function" ||
		typeof storage?.setItem !== "function" ||
		typeof storage?.removeItem !== "function"
	) {
		return null;
	}

	return storage;
}

export function readStorage(key: string): string | null {
	const storage = getLocalStorage();
	if (!storage) {
		return null;
	}

	return storage.getItem(key);
}

export function writeStorage(key: string, value: string | null): void {
	const storage = getLocalStorage();
	if (!storage) {
		return;
	}

	if (value === null) {
		storage.removeItem(key);
		return;
	}

	storage.setItem(key, value);
}
