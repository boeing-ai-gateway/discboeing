export function languageForPath(path: string): string {
	const lower = path.toLowerCase();
	if (lower.endsWith(".go")) return "go";
	if (lower.endsWith(".ts")) return "typescript";
	if (lower.endsWith(".tsx")) return "typescript";
	if (lower.endsWith(".js")) return "javascript";
	if (lower.endsWith(".jsx")) return "javascript";
	if (lower.endsWith(".json")) return "json";
	if (lower.endsWith(".css")) return "css";
	if (lower.endsWith(".html")) return "html";
	if (lower.endsWith(".md")) return "markdown";
	return "plaintext";
}

export function lspLanguage(language: string): string | null {
	if (language === "go") return "go";
	if (language === "typescript" || language === "javascript") return language;
	return null;
}
