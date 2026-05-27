import * as monaco from "monaco-editor";
import { readFile } from "../api/files";
import { uriToPath } from "../editor/uri";

export type LSPClientOptions = {
	root: string;
	language: string;
	openFile: (path: string, selection?: monaco.IRange) => Promise<void>;
};

export class LSPClient {
	private socket: WebSocket;
	private nextId = 1;
	private pending = new Map<number, (value: unknown) => void>();
	private diagnostics = new Map<string, monaco.editor.IMarkerData[]>();

	constructor(private options: LSPClientOptions) {
		const scheme = location.protocol === "https:" ? "wss" : "ws";
		this.socket = new WebSocket(`${scheme}://${location.host}/api/lsp/${options.language}`);
		this.socket.addEventListener("message", (event) => this.handleMessage(event.data));
		this.socket.addEventListener("open", () => {
			void this.request("initialize", {
				processId: null,
				rootUri: `file://${options.root}`,
				capabilities: {}
			}).then(() => this.notify("initialized", {}));
		});
		this.registerProviders();
	}

	didOpen(model: monaco.editor.ITextModel, languageId: string) {
		this.notify("textDocument/didOpen", {
			textDocument: {
				uri: model.uri.toString(),
				languageId,
				version: model.getVersionId(),
				text: model.getValue()
			}
		});
	}

	didChange(model: monaco.editor.ITextModel) {
		this.notify("textDocument/didChange", {
			textDocument: { uri: model.uri.toString(), version: model.getVersionId() },
			contentChanges: [{ text: model.getValue() }]
		});
	}

	didSave(model: monaco.editor.ITextModel) {
		this.notify("textDocument/didSave", { textDocument: { uri: model.uri.toString() }, text: model.getValue() });
	}

	didClose(model: monaco.editor.ITextModel) {
		this.notify("textDocument/didClose", { textDocument: { uri: model.uri.toString() } });
	}

	async definition(model: monaco.editor.ITextModel, position: monaco.Position) {
		const result = await this.request("textDocument/definition", {
			textDocument: { uri: model.uri.toString() },
			position: toLSPPosition(position)
		});
		const location = Array.isArray(result) ? result[0] : result;
		if (location && typeof location === "object" && "uri" in location && "range" in location) {
			const target = location as { uri: string; range: LSPRange };
			await this.options.openFile(uriToPath(this.options.root, target.uri), fromLSPRange(target.range));
		}
	}

	async references(model: monaco.editor.ITextModel, position: monaco.Position) {
		return this.request("textDocument/references", {
			textDocument: { uri: model.uri.toString() },
			position: toLSPPosition(position),
			context: { includeDeclaration: true }
		});
	}

	async symbols(model: monaco.editor.ITextModel) {
		return this.request("textDocument/documentSymbol", { textDocument: { uri: model.uri.toString() } });
	}

	private registerProviders() {
		monaco.languages.registerHoverProvider(this.options.language, {
			provideHover: async (model, position) => {
				const result = await this.request("textDocument/hover", {
					textDocument: { uri: model.uri.toString() },
					position: toLSPPosition(position)
				});
				return toMonacoHover(result);
			}
		});
		monaco.languages.registerDefinitionProvider(this.options.language, {
			provideDefinition: async (model, position) => {
				const result = await this.request("textDocument/definition", {
					textDocument: { uri: model.uri.toString() },
					position: toLSPPosition(position)
				});
				return toMonacoLocations(result);
			}
		});
		monaco.languages.registerReferenceProvider(this.options.language, {
			provideReferences: async (model, position) => {
				const result = await this.references(model, position);
				return toMonacoLocations(result) as monaco.languages.Location[];
			}
		});
		monaco.languages.registerDocumentSymbolProvider(this.options.language, {
			provideDocumentSymbols: async (model) => {
				const result = await this.symbols(model);
				return toMonacoSymbols(result);
			}
		});
	}

	private request(method: string, params: unknown): Promise<unknown> {
		const id = this.nextId++;
		this.send({ jsonrpc: "2.0", id, method, params });
		return new Promise((resolve) => this.pending.set(id, resolve));
	}

	private notify(method: string, params: unknown) {
		this.send({ jsonrpc: "2.0", method, params });
	}

	private send(message: unknown) {
		const data = JSON.stringify(message);
		if (this.socket.readyState === WebSocket.OPEN) {
			this.socket.send(data);
		} else {
			this.socket.addEventListener("open", () => this.socket.send(data), { once: true });
		}
	}

	private handleMessage(data: string) {
		const message = JSON.parse(data) as Record<string, unknown>;
		if (typeof message.id === "number" && this.pending.has(message.id)) {
			this.pending.get(message.id)?.(message.result);
			this.pending.delete(message.id);
			return;
		}
		if (message.method === "textDocument/publishDiagnostics") {
			const params = message.params as { uri: string; diagnostics: Array<{ message: string; severity?: number; range: LSPRange }> };
			const markers = params.diagnostics.map((diagnostic) => ({
				message: diagnostic.message,
				severity: diagnostic.severity === 1 ? monaco.MarkerSeverity.Error : monaco.MarkerSeverity.Warning,
				...rangeToMarker(diagnostic.range)
			}));
			this.diagnostics.set(params.uri, markers);
			const model = monaco.editor.getModel(monaco.Uri.parse(params.uri));
			if (model) monaco.editor.setModelMarkers(model, this.options.language, markers);
		}
	}
}

type LSPRange = { start: { line: number; character: number }; end: { line: number; character: number } };

function toLSPPosition(position: monaco.Position) {
	return { line: position.lineNumber - 1, character: position.column - 1 };
}

function fromLSPRange(range: LSPRange): monaco.IRange {
	return {
		startLineNumber: range.start.line + 1,
		startColumn: range.start.character + 1,
		endLineNumber: range.end.line + 1,
		endColumn: range.end.character + 1
	};
}

function rangeToMarker(range: LSPRange) {
	return fromLSPRange(range);
}

function toMonacoHover(result: unknown): monaco.languages.Hover | null {
	if (!result || typeof result !== "object" || !("contents" in result)) return null;
	const hover = result as { contents: unknown; range?: LSPRange };
	const contents = Array.isArray(hover.contents) ? hover.contents : [hover.contents];
	return {
		contents: contents.map((item) => {
			if (typeof item === "string") return { value: item };
			if (item && typeof item === "object" && "value" in item) return { value: String((item as { value: unknown }).value) };
			return { value: String(item) };
		}),
		range: hover.range ? fromLSPRange(hover.range) : undefined
	};
}

function toMonacoLocations(result: unknown): monaco.languages.Location | monaco.languages.Location[] | null {
	if (!result) return null;
	const values = Array.isArray(result) ? result : [result];
	return values
		.filter((value): value is { uri: string; range: LSPRange } => Boolean(value && typeof value === "object" && "uri" in value && "range" in value))
		.map((location) => ({ uri: monaco.Uri.parse(location.uri), range: fromLSPRange(location.range) }));
}

function toMonacoSymbols(result: unknown): monaco.languages.DocumentSymbol[] {
	if (!Array.isArray(result)) return [];
	return result
		.filter((item): item is { name: string; kind: number; range: LSPRange; selectionRange?: LSPRange; children?: unknown[] } =>
			Boolean(item && typeof item === "object" && "name" in item && "kind" in item && "range" in item)
		)
		.map((item) => ({
			name: item.name,
			detail: "",
			kind: item.kind as monaco.languages.SymbolKind,
			tags: [],
			range: fromLSPRange(item.range),
			selectionRange: fromLSPRange(item.selectionRange ?? item.range),
			children: toMonacoSymbols(item.children ?? [])
		}));
}

export async function ensureTextLoaded(path: string) {
	return readFile(path);
}
