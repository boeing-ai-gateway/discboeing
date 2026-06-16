import { harden } from "rehype-harden";
import rehypeRaw from "rehype-raw";
import rehypeSanitize, { defaultSchema } from "rehype-sanitize";
import remarkFrontmatter from "remark-frontmatter";
import remarkGfm from "remark-gfm";
import remarkParse from "remark-parse";
import remarkRehype from "remark-rehype";
import type { Root } from "hast";
import { unified } from "unified";
import type { MarkdownPluginConfig } from "./types";
import { remarkCodeMeta } from "./remark-code-meta";
import { remarkFrontmatterTable } from "./remark-frontmatter-table";

const sanitizeSchema = {
	...defaultSchema,
	protocols: {
		...defaultSchema.protocols,
		href: [...(defaultSchema.protocols?.href ?? []), "tel"],
	},
	attributes: {
		...defaultSchema.attributes,
		code: [...(defaultSchema.attributes?.code ?? []), "metastring"],
	},
};

export function parseMarkdownToHast(
	markdown: string,
	plugins?: MarkdownPluginConfig,
): Root {
	const processor = unified().use(remarkParse);

	if (plugins?.cjk) {
		for (const plugin of plugins.cjk.remarkPluginsBefore) {
			processor.use([plugin]);
		}
	}

	processor
		.use(remarkFrontmatter, ["yaml"])
		.use(remarkGfm)
		.use(remarkCodeMeta)
		.use(remarkFrontmatterTable);

	if (plugins?.cjk) {
		for (const plugin of plugins.cjk.remarkPluginsAfter) {
			processor.use([plugin]);
		}
	}

	if (plugins?.math) {
		processor.use([plugins.math.remarkPlugin]);
	}

	processor
		.use(remarkRehype, { allowDangerousHtml: true })
		.use(rehypeRaw)
		.use(rehypeSanitize, sanitizeSchema)
		.use(harden, {
			allowDataImages: true,
			allowedImagePrefixes: ["*"],
			allowedLinkPrefixes: ["*"],
			allowedProtocols: ["*"],
			defaultOrigin: undefined,
		});

	if (plugins?.math) {
		processor.use([plugins.math.rehypePlugin]);
	}

	return processor.runSync(processor.parse(markdown)) as Root;
}
