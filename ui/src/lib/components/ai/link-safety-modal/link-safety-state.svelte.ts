export class LinkSafetyState {
	pendingUrl = $state<string | null>(null);

	get isOpen(): boolean {
		return this.pendingUrl !== null;
	}

	get url(): string {
		return this.pendingUrl ?? "";
	}

	requestOpen(url: string) {
		this.pendingUrl = url;
	}

	close() {
		this.pendingUrl = null;
	}
}
