export function getErrorMessage(error: unknown): string | null {
	if (error == null || error === "") {
		return null;
	}
	if (typeof error === "string") {
		return error;
	}
	if (error instanceof Error) {
		return error.message;
	}
	if (typeof error === "object") {
		const record = error as Record<string, unknown>;
		for (const key of ["errorText", "message", "error", "detail"]) {
			const value = record[key];
			if (typeof value === "string" && value) {
				return value;
			}
		}
		try {
			return JSON.stringify(error);
		} catch {
			return String(error);
		}
	}
	return String(error);
}
