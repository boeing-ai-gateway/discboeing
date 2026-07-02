# Conversation Web Component Design

This document proposes an HTML-first conversation renderer implemented with
Svelte custom elements and exposed as a Web Component library. It is intentionally
separate from the current Svelte app components. The current `ConversationPane`
and `ai/message` components should remain in place while this design evolves.

## Companion definition assets

This design should ship with machine-readable companion files so consumers can
discover the custom element API without reading this document end-to-end.

Initial companion files live next to this document:

| File                    | Purpose                                                                                                          |
| ----------------------- | ---------------------------------------------------------------------------------------------------------------- |
| `types.ts`              | TypeScript element interfaces, init object shapes, event detail types, and `HTMLElementTagNameMap` augmentation. |
| `vscode.html-data.json` | VS Code HTML custom data for tag and attribute autocomplete in plain HTML.                                       |

Future package artifacts should also include:

| Artifact               | Purpose                                                                             |
| ---------------------- | ----------------------------------------------------------------------------------- |
| `index.d.ts`           | Published TypeScript declarations generated from or aligned with `types.ts`.        |
| `custom-elements.json` | Custom Elements Manifest for documentation generators, catalogs, and other tooling. |
| `conversation.css`     | Default CSS plus documented custom properties and shadow parts.                     |

The TypeScript definitions and VS Code custom data are hand-authored for now.
Once implementation starts, the preferred flow is to keep source JSDoc,
`types.ts`, and generated manifests aligned through tests or a generation step.

## Goals

- Model conversation data as authored DOM, not as a large `messages` property.
- Keep the light DOM meaningful and inspectable in DevTools.
- Make JavaScript helpers create or update real child elements instead of
  introducing a second source of truth.
- Let events bubble from specific child elements to the conversation root.
- Use attributes for scalar state and child elements for message parts.
- Keep styling configurable through CSS custom properties and `::part`.
- Allow Svelte to author the implementation while exposing framework-neutral
  custom elements.

## Non-goals for the first version

- Replacing `ui/src/lib/components/app/ConversationPane.svelte`.
- Matching every current Discboeing-only affordance in the first pass.
- Exposing a long-lived `conversation.messages = [...]` rendering source.
- Requiring consumers to use Svelte.

## Design principle

The DOM is the source of truth.

JavaScript APIs may help create, patch, stream, or remove messages, but those
APIs should materialize the same DOM that a user could have written by hand.
This follows native HTML patterns such as `<select><option>...</option></select>`
and `<table><tr><td>...</td></tr></table>`: script APIs manipulate the element
model; they do not replace children with a competing hidden data structure.

## Element vocabulary

The library should expose these custom elements:

| Element                    | Purpose                                                             |
| -------------------------- | ------------------------------------------------------------------- |
| `<disco-conversation>`     | Scroll container, message list coordinator, event delegation root.  |
| `<disco-turn>`             | Optional grouping element for a template-defined conversation turn. |
| `<disco-step-group>`       | Optional expandable group for collapsed assistant step messages.    |
| `<disco-message>`          | A single chat message. Mirrors a `ChatMessage`.                     |
| `<disco-message-content>`  | Text content part, either plain text or markdown.                   |
| `<disco-generated-text>`   | Expandable generated prompt text for user messages.                 |
| `<disco-reasoning>`        | Reasoning/thinking part.                                            |
| `<disco-tool-call>`        | Dynamic tool part.                                                  |
| `<disco-tool-input>`       | Tool input payload.                                                 |
| `<disco-tool-output>`      | Tool output payload.                                                |
| `<disco-attachment>`       | File/image/source attachment part.                                  |
| `<disco-browser-activity>` | Browser activity captured during a turn.                            |
| `<disco-event>`            | Generic timeline/event/data part.                                   |
| `<disco-metadata>`         | Structured metadata associated with conversation/message/part.      |

`<disco-turn>` is a public, first-class element, but it is not required. The
renderer does not infer turn boundaries from message order. The host template or
adapter decides whether to create turns and where they begin and end. Consumers
that do not care about turns may render messages directly under
`<disco-conversation>`, and no automatic grouping semantics are implied.

## Turn boundaries and open state

`<disco-turn>` is the unit of visual turn grouping when a template chooses to
render turns. A typical turn contains a user `<disco-message>` followed by an
assistant `<disco-message>`. User-message children are ordered parts such as
metadata, content, generated text, and attachments. Assistant-message children
are ordered parts such as metadata, reasoning, tool calls, browser activity, and
content; when an assistant message ends with content, preceding assistant work
may be wrapped in a `<disco-step-group>` inside that same assistant message.
Turn boundaries are not inferred by the web component. The host template or
Discboeing adapter creates turn elements and decides which messages belong to
each turn. `open` defaults to true for turns.

Expandable conversation elements use a shared boolean `open` attribute/property
for their expanded state. This mirrors native HTML patterns such as
`<details open>` and avoids a separate hidden UI state model. The first v1
expandable elements are:

- `<disco-turn open>`
- `<disco-step-group open>`
- `<disco-generated-text open>`
- `<disco-reasoning open>`
- `<disco-tool-call open>`
- `<disco-browser-activity open>`
- `<disco-event open>`

When an expandable element changes, it emits `disco-expand-change` with the
new `open` value and whichever identifiers apply (`turnId`, `messageId`,
`partId`). Event consumers can discriminate the concrete element from
`event.target`.

`<disco-reasoning>` follows the app reasoning UI by default: it renders a
brain-icon trigger, collapsed summary text, hover-revealed chevron control, and
markdown-formatted expanded content. Streaming reasoning uses the same
"Thinking..." shimmer treatment and is not user-toggleable until complete.

## Canonical minimal HTML

```html
<disco-conversation>
	<disco-turn id="turn-1" open>
		<disco-message id="m1" from="user">
			<disco-message-content>Hello.</disco-message-content>
		</disco-message>

		<disco-message id="m2" from="assistant">
			<disco-message-content>Hello! How can I help?</disco-message-content>
		</disco-message>
	</disco-turn>
</disco-conversation>
```

## Full HTML structure

This example shows the complete intended structure. It maps directly to the
current message model: a message has scalar fields/metadata and an ordered list
of parts.

```html
<disco-conversation
	id="thread-123"
	status="ready"
	auto-scroll
	chat-width="constrained"
>
	<disco-metadata>
		<script type="application/json">
			{
				"sessionId": "session-123",
				"threadId": "thread-123"
			}
		</script>
	</disco-metadata>

	<disco-turn id="turn-1" open>
		<disco-message
			id="msg-user-1"
			from="user"
			state="complete"
			created-at="2026-06-15T17:44:38Z"
		>
			<disco-metadata>
				<script type="application/json">
					{
						"originalText": "Can you inspect this file?",
						"discboeing": { "turnId": "turn-1" }
					}
				</script>
			</disco-metadata>

			<disco-message-content part-id="part-user-text-1" format="text">
				Can you inspect this file?
			</disco-message-content>

			<disco-generated-text
				part-id="part-user-generated-1"
				label="Generated text"
			>
				Can you inspect this file and summarize what it exports?
			</disco-generated-text>

			<disco-attachment
				part-id="part-user-file-1"
				kind="file"
				src="/uploads/app.ts"
				filename="app.ts"
				media-type="text/typescript"
			></disco-attachment>
		</disco-message>

		<disco-message
			id="msg-assistant-1"
			from="assistant"
			state="complete"
			model="gpt-5.5"
		>
			<disco-metadata>
				<script type="application/json">
					{
						"model": "gpt-5.5",
						"discboeing": { "turnId": "turn-1" }
					}
				</script>
			</disco-metadata>

			<disco-step-group label="2 STEPS">
				<disco-reasoning part-id="part-reasoning-1" state="complete">
					I need to read the uploaded file before summarizing it.
				</disco-reasoning>

				<disco-tool-call
					part-id="part-tool-1"
					call-id="tool-read-1"
					name="Read"
					state="output-available"
					title="Read app.ts"
				>
					<disco-tool-input format="json">
						<script type="application/json">
							{ "file_path": "/uploads/app.ts" }
						</script>
					</disco-tool-input>

					<disco-tool-output format="json">
						<script type="application/json">
							{
								"content": "export const value = 1;"
							}
						</script>
					</disco-tool-output>
				</disco-tool-call>
			</disco-step-group>

			<disco-browser-activity
				part-id="part-browser-1"
				step-count="2"
				summary="2 browser steps"
			>
				<disco-metadata>
					<script type="application/json">
						{
							"events": [
								{ "method": "navigate", "url": "http://localhost:3100" },
								{ "method": "screenshot", "filename": "step-2.png" }
							]
						}
					</script>
				</disco-metadata>
			</disco-browser-activity>

			<disco-message-content part-id="part-text-final" format="markdown">
				The file exports a constant named `value`.
			</disco-message-content>
		</disco-message>
	</disco-turn>
</disco-conversation>
```

## Mapping from message data to HTML

The current app receives `ChatMessage` values. In this Web Component API,
`ChatMessage` maps to DOM as follows:

| Message field                    | HTML representation                                   |
| -------------------------------- | ----------------------------------------------------- |
| `message.id`                     | `<disco-message id="...">`                            |
| `message.role`                   | `<disco-message from="user&#124;assistant...">`       |
| `message.status === "streaming"` | `<disco-message state="streaming">`                   |
| `message.provisional`            | `<disco-message provisional>`                         |
| `message.synthetic`              | `<disco-message synthetic>`                           |
| `message.replacesMessageId`      | `<disco-message replaces-message-id="...">`           |
| `message.replacedByMessageId`    | `<disco-message replaced-by-message-id="...">`        |
| `message.metadata`               | `<disco-metadata><script type="application/json">...` |
| `message.parts[]`                | ordered child part elements                           |

Part mappings:

| Part shape                                   | HTML representation                                                              |
| -------------------------------------------- | -------------------------------------------------------------------------------- |
| `{ type: "text", text }`                     | `<disco-message-content format="markdown&#124;text">...</disco-message-content>` |
| generated user prompt text                   | `<disco-generated-text label="Generated text">...</disco-generated-text>`        |
| `{ type: "reasoning", text, state }`         | `<disco-reasoning state="streaming&#124;complete" open>...</disco-reasoning>`    |
| `{ type: "dynamic-tool", ... }`              | `<disco-tool-call ... open>` with input/output children                          |
| `{ type: "file", filename, mediaType, url }` | `<disco-attachment kind="file" ...>`                                             |
| browser activity chunks                      | `<disco-browser-activity open>` plus JSON metadata                               |
| data/system events                           | `<disco-event kind="..." open>`                                                  |
| unknown future parts                         | `<disco-event kind="custom" part-type="...">` plus JSON metadata                 |

## Attribute conventions

Use attributes for scalar, serializable state:

```html
<disco-message
	id="msg-1"
	from="assistant"
	state="streaming"
	model="gpt-5.5"
	provisional
	synthetic
	replaces-message-id="msg-old"
	replaced-by-message-id="msg-new"
></disco-message>
```

Recommended attribute names:

| Attribute    | Applies to                        | Values                                                        |
| ------------ | --------------------------------- | ------------------------------------------------------------- |
| `from`       | `disco-message`                   | `user`, `assistant`, `system`, `tool`                         |
| `state`      | message/part/tool                 | `pending`, `streaming`, `complete`, `error`, plus tool states |
| `format`     | content/input/output              | `text`, `markdown`, `json`                                    |
| `open`       | turn/reasoning/tool/browser/event | boolean expanded/collapsed state                              |
| `part-id`    | all part elements                 | stable part identifier                                        |
| `call-id`    | `disco-tool-call`                 | tool call id                                                  |
| `name`       | `disco-tool-call`                 | tool name                                                     |
| `src`        | `disco-attachment`                | attachment URL                                                |
| `media-type` | `disco-attachment`                | MIME type                                                     |
| `filename`   | `disco-attachment`                | display filename                                              |

Use child text for human-authored content. Use
`<script type="application/json">` for structured payloads.

## Content formats

### Plain text

```html
<disco-message-content format="text">
	Literal text content.
</disco-message-content>
```

### Markdown

```html
<disco-message-content format="markdown">
	## Summary This is **markdown**.
</disco-message-content>
```

When `format="markdown"`, the element treats its light DOM text as source text
and renders sanitized markdown in its shadow DOM.

### JSON payloads

```html
<disco-tool-input format="json">
	<script type="application/json">
		{ "command": "pnpm test" }
	</script>
</disco-tool-input>
```

JSON payload elements may also expose `.value` properties for programmatic use,
but the canonical DOM remains the child `<script type="application/json">`.

## Dynamic updates

DOM mutation is the update model.

### Append a message manually

```js
const message = document.createElement("disco-message");
message.id = "msg-3";
message.setAttribute("from", "user");

const content = document.createElement("disco-message-content");
content.textContent = "Can you run the tests?";

message.append(content);
conversation.append(message);
```

### Use helper methods that write DOM

Helper methods are allowed, but they should create or update real children:

```js
conversation.appendMessage({
	id: "msg-3",
	from: "user",
	parts: [{ type: "text", text: "Can you run the tests?" }],
});

conversation.appendPart("msg-4", {
	type: "tool-call",
	callId: "tool-1",
	name: "Bash",
	state: "input-available",
	input: { command: "pnpm test" },
});
```

After these methods run, DevTools should show equivalent child elements under
`<disco-conversation>`.

### Streaming text

```html
<disco-message id="msg-stream" from="assistant" state="streaming">
	<disco-message-content
		id="stream-content"
		format="markdown"
	></disco-message-content>
</disco-message>
```

```js
const content = document.querySelector("#stream-content");
content.append("First chunk");
content.append(" next chunk");

const message = document.querySelector("#msg-stream");
message.setAttribute("state", "complete");
```

A convenience method may exist:

```js
content.appendTextDelta(" next chunk");
```

It should still update the text node/content of the element.

## Events

Events are the primary integration API. They should be lowercase kebab-case,
bubble, and be composed so hosts can listen on `<disco-conversation>`.

Request events should be cancelable when the component has a possible default
action.

```js
conversation.addEventListener("disco-link-open-request", (event) => {
	event.preventDefault();
	if (isSafe(event.detail.url)) {
		window.open(event.detail.url, "_blank", "noopener,noreferrer");
	}
});
```

Recommended events:

| Event                             | Source             | Cancelable | Purpose                                               |
| --------------------------------- | ------------------ | ---------- | ----------------------------------------------------- |
| `disco-link-open-request`         | content            | yes        | User clicked a rendered link.                         |
| `disco-attachment-open-request`   | attachment         | yes        | User opened an attachment.                            |
| `disco-tool-approval-request`     | tool call          | yes        | Tool requires host approval UI.                       |
| `disco-tool-approval-response`    | tool call          | no         | User responded inside built-in approval UI.           |
| `disco-message-copy-request`      | message            | yes        | User requested message copy.                          |
| `disco-message-retry-request`     | message            | yes        | User requested retry/regeneration.                    |
| `disco-selection-comment-request` | conversation       | yes        | User wants to comment on selected text.               |
| `disco-scroll-state-change`       | conversation       | no         | Near-bottom/stick-to-bottom changed.                  |
| `disco-expand-change`             | expandable element | no         | Turn/tool/reasoning/browser/event open state changed. |
| `disco-part-action`               | any part           | yes        | Generic custom action from an unknown part.           |

Example event detail:

```ts
type ExpandChangeDetail = {
	turnId?: string;
	messageId?: string;
	partId?: string;
	open: boolean;
};
```

## Imperative methods

Methods should command the component or create/update DOM. They should not
establish a persistent alternative data source.

Recommended methods on `<disco-conversation>`:

```ts
appendMessage(init: MessageInit): DiscoMessageElement;
replaceMessages(inits: MessageInit[]): DiscoMessageElement[];
clearMessages(): void;
getMessages(): MessageInit[];
scrollToBottom(options?: ScrollIntoViewOptions): void;
getMessage(id: string): DiscoMessageElement | null;
appendPart(messageId: string, init: PartInit): Element;
```

Recommended methods on child elements:

```ts
turn.open = true;
message.appendPart(init: PartInit): Element;
message.setState(state: MessageState): void;
reasoning.open = true;
content.appendTextDelta(text: string): void;
tool.open = true;
tool.respond(response: ToolApprovalResponse): void;
tool.setInput(value: unknown): void;
tool.setOutput(value: unknown): void;
browserActivity.open = true;
event.open = true;
```

All creation/update methods should mutate the light DOM.

The Discboeing app wrapper should render custom-element HTML declaratively from
Svelte. It should not call the imperative DOM helper methods during normal
rendering. The helper methods exist for non-Svelte consumers and targeted
streaming updates.

## Styling

Expose style hooks through CSS custom properties and `part` attributes. The
conversation components use the same token layering as `<disco-markdown>`:

1. common app/design-system tokens such as `--background`, `--foreground`,
   `--border`, `--primary`, and `--font-sans`
2. shared Discboeing tokens such as `--disco-background`,
   `--disco-foreground`, `--disco-border`, and `--disco-font-sans`
3. conversation-family tokens such as `--disco-conversation-background`
4. element-specific tokens such as `--disco-message-user-max-width`

The shared `--disco-*` tokens are intentionally the bridge to markdown. When a
`<disco-message-content format="markdown">` nests `<disco-markdown>`, markdown
inherits the same theme through shared tokens instead of depending on the
conversation implementation.

```css
disco-conversation {
	--disco-background: var(--background, #ffffff);
	--disco-foreground: var(--foreground, #111827);
	--disco-muted: var(--muted, #f9fafb);
	--disco-muted-foreground: var(--muted-foreground, #6b7280);
	--disco-border: var(--border, #e5e7eb);
	--disco-primary: var(--primary, #2563eb);
	--disco-radius: 0.75rem;
	--disco-font-sans: var(--font-sans, system-ui, sans-serif);
	--disco-font-mono: var(--font-mono, ui-monospace, monospace);

	--disco-conversation-padding: 1rem;
	--disco-conversation-gap: 1rem;
	--disco-conversation-max-width: 48rem;
	--disco-message-user-background: var(--secondary, #f3f4f6);
	--disco-message-user-padding: 0.75rem 1rem;
	--disco-message-user-radius: 0.5rem;
	--disco-message-user-max-width: min(30rem, 92%);
}

disco-conversation::part(viewport) {
	padding: 1rem;
}

disco-message::part(container) {
	margin-block: 0.5rem;
}

disco-message::part(content) {
	border-radius: var(--disco-radius);
}
```

Initial public conversation tokens:

| Token                                   | Purpose                                   |
| --------------------------------------- | ----------------------------------------- |
| `--disco-conversation-background`       | Conversation surface background.          |
| `--disco-conversation-foreground`       | Default text color.                       |
| `--disco-conversation-muted`            | Muted surfaces.                           |
| `--disco-conversation-muted-foreground` | Secondary text/icons.                     |
| `--disco-conversation-border`           | Borders and separators.                   |
| `--disco-conversation-card`             | Card/tool/event backgrounds.              |
| `--disco-conversation-accent`           | Hover/accent surfaces.                    |
| `--disco-conversation-secondary`        | User-message default bubble background.   |
| `--disco-conversation-destructive`      | Error/destructive state color.            |
| `--disco-conversation-primary`          | Links and primary accents.                |
| `--disco-conversation-radius`           | Default rounded card radius.              |
| `--disco-conversation-font-sans`        | Sans-serif font family.                   |
| `--disco-conversation-font-mono`        | Monospace font family.                    |
| `--disco-conversation-padding`          | Scroll viewport padding.                  |
| `--disco-conversation-gap`              | Gap between top-level messages/turns.     |
| `--disco-conversation-max-width`        | Width used by `chat-width="constrained"`. |
| `--disco-message-user-background`       | User bubble background.                   |
| `--disco-message-user-padding`          | User bubble padding.                      |
| `--disco-message-user-radius`           | User bubble radius.                       |
| `--disco-message-user-max-width`        | User bubble max width.                    |
| `--disco-message-gap`                   | Gap between parts in one message.         |
| `--disco-message-content-background`    | Message content background escape hatch.  |
| `--disco-message-content-padding`       | Message content padding escape hatch.     |
| `--disco-message-content-radius`        | Message content radius escape hatch.      |
| `--disco-message-content-width`         | Message content width escape hatch.       |
| `--disco-message-content-max-width`     | Message content max-width escape hatch.   |

Suggested parts:

| Element                  | Parts                                                                  |
| ------------------------ | ---------------------------------------------------------------------- |
| `disco-conversation`     | `viewport`, `content`, `scroll-button`, `empty-state`                  |
| `disco-turn`             | `container`, `trigger`, `state-line`, `state-label`, `content`         |
| `disco-message`          | `container`, `content`, `actions`                                      |
| `disco-message-content`  | `container`, `markdown`, `code-block`, `link`                          |
| `disco-reasoning`        | `container`, `trigger`, `content`                                      |
| `disco-tool-call`        | `container`, `header`, `title`, `status`, `input`, `output`, `actions` |
| `disco-attachment`       | `container`, `preview`, `info`, `filename`                             |
| `disco-browser-activity` | `container`, `header`, `title`, `summary`, `content`                   |
| `disco-event`            | `container`, `header`, `content`, `actions`                            |

## Svelte implementation notes

Each public custom element can be authored as a Svelte custom element:

```svelte
<svelte:options customElement="disco-message" />
```

The implementation should preserve light DOM children via slots where possible:

```svelte
<div part="container">
	<slot></slot>
</div>
```

For elements that transform source text, such as markdown content, the component
can read its light DOM text content and render sanitized output into shadow DOM.
It should observe child/text mutations so streaming updates re-render.

## Deferred implementation questions

- Which Discboeing-specific metadata deserves first-class attributes versus JSON
  metadata?
- Should large tool outputs remain in light DOM JSON scripts, or should helper
  methods store large payloads as properties while rendering a summarized DOM
  placeholder?
- How much virtualization is needed before a DOM-first source of truth becomes
  too expensive?
