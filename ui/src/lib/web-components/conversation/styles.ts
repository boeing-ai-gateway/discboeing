export const conversationHostStyles = `
	:host {
		--disco-conversation-background: var(--disco-background, var(--background, #ffffff));
		--disco-conversation-foreground: var(--disco-foreground, var(--foreground, #111827));
		--disco-conversation-muted: var(--disco-muted, var(--muted, #f9fafb));
		--disco-conversation-muted-foreground: var(--disco-muted-foreground, var(--muted-foreground, #6b7280));
		--disco-conversation-border: var(--disco-border, var(--border, #e5e7eb));
		--disco-conversation-card: var(--disco-card, var(--card, #ffffff));
		--disco-conversation-accent: var(--disco-accent, var(--accent, #f3f4f6));
		--disco-conversation-secondary: var(--disco-secondary, var(--secondary, #f3f4f6));
		--disco-conversation-destructive: var(--disco-destructive, var(--destructive, #dc2626));
		--disco-conversation-primary: var(--disco-primary, var(--primary, #2563eb));
		--disco-conversation-radius: var(--disco-radius, 0.75rem);
		--disco-conversation-font-sans: var(--disco-font-sans, var(--font-sans, system-ui, sans-serif));
		--disco-conversation-font-mono: var(--disco-font-mono, var(--font-mono, ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace));
		color: var(--disco-conversation-foreground);
		font-family: var(--disco-conversation-font-sans);
	}

	*,
	*::before,
	*::after {
		box-sizing: border-box;
	}
`;

export const buttonResetStyles = `
	button {
		font: inherit;
	}
`;
