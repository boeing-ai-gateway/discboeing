import type {
	PhrasingContent,
	Root,
	RootContent,
	Table,
	TableCell,
	TableRow,
	Text,
} from "mdast";
import type { Plugin } from "unified";
import YAML from "yaml";

function createTextCell(value: string): TableCell {
	const textNode: Text = {
		type: "text",
		value,
	};

	return {
		type: "tableCell",
		children: [textNode satisfies PhrasingContent],
	};
}

function stringifyScalar(value: unknown): string {
	if (value === null) {
		return "null";
	}
	if (value === undefined) {
		return "";
	}
	if (typeof value === "string") {
		return value;
	}
	if (typeof value === "number" || typeof value === "boolean") {
		return String(value);
	}
	return JSON.stringify(value);
}

function collectRows(
	value: unknown,
	path: string,
	rows: TableRow[],
	seen = new WeakSet<object>(),
) {
	if (Array.isArray(value)) {
		if (value.length === 0) {
			rows.push({
				type: "tableRow",
				children: [createTextCell(path), createTextCell("[]")],
			});
			return;
		}

		const allScalars = value.every(
			(item) =>
				item === null ||
				item === undefined ||
				typeof item === "string" ||
				typeof item === "number" ||
				typeof item === "boolean",
		);
		if (allScalars) {
			rows.push({
				type: "tableRow",
				children: [
					createTextCell(path),
					createTextCell(value.map((item) => stringifyScalar(item)).join(", ")),
				],
			});
			return;
		}

		value.forEach((item, index) => {
			collectRows(item, `${path}[${index}]`, rows, seen);
		});
		return;
	}

	if (typeof value === "object" && value !== null) {
		if (seen.has(value)) {
			rows.push({
				type: "tableRow",
				children: [createTextCell(path), createTextCell("[Circular]")],
			});
			return;
		}
		seen.add(value);

		const entries = Object.entries(value);
		if (entries.length === 0) {
			rows.push({
				type: "tableRow",
				children: [createTextCell(path), createTextCell("{}")],
			});
			return;
		}

		for (const [key, child] of entries) {
			const nextPath = path ? `${path}.${key}` : key;
			collectRows(child, nextPath, rows, seen);
		}
		return;
	}

	rows.push({
		type: "tableRow",
		children: [createTextCell(path), createTextCell(stringifyScalar(value))],
	});
}

function createYamlTable(value: unknown): Table {
	const rows: TableRow[] = [
		{
			type: "tableRow",
			children: [createTextCell("Field"), createTextCell("Value")],
		},
	];

	collectRows(value, "", rows);

	return {
		type: "table",
		align: [null, null],
		children: rows,
	};
}

export const remarkFrontmatterTable: Plugin<[], Root> = () => (tree) => {
	tree.children = tree.children.flatMap((node): RootContent[] => {
		if (node.type !== "yaml") {
			return [node];
		}

		try {
			const parsed = YAML.parse(node.value);
			return [createYamlTable(parsed)];
		} catch {
			return [
				{
					type: "code",
					lang: "yaml",
					value: node.value,
				},
			];
		}
	});
};
