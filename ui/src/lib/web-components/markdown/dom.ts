export function getCustomElementHost<T extends HTMLElement = HTMLElement>(
	node: Node,
): T {
	const root = node.getRootNode();
	if (root instanceof ShadowRoot) {
		return root.host as T;
	}
	return node as T;
}

export function emitComposedEvent<T>(
	element: Element,
	type: string,
	detail: T,
	options: { cancelable?: boolean } = {},
): boolean {
	return element.dispatchEvent(
		new CustomEvent(type, {
			detail,
			bubbles: true,
			composed: true,
			cancelable: options.cancelable ?? false,
		}),
	);
}
