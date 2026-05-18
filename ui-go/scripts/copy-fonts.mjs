import { copyFile, mkdir, readdir } from "node:fs/promises";
import path from "node:path";

const root = process.cwd();
const targetDir = path.join(root, "static", "files");

const fontSources = [
	{
		dir: path.join(root, "node_modules", "@fontsource", "geist-sans", "files"),
		match: /^geist-sans-latin-(400|500|600|700)-normal\.woff2?$/,
	},
	{
		dir: path.join(root, "node_modules", "@fontsource", "jetbrains-mono", "files"),
		match: /^jetbrains-mono-.+-(400|500|600|700)-normal\.woff2?$/,
	},
];

await mkdir(targetDir, { recursive: true });

for (const source of fontSources) {
	for (const file of await readdir(source.dir)) {
		if (!source.match.test(file)) continue;
		await copyFile(path.join(source.dir, file), path.join(targetDir, file));
	}
}
