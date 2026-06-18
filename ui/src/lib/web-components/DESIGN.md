# Web Components Design

This directory contains reusable Discobot Web Components implemented with Svelte custom elements. The components are intended to be usable by the Discobot app and by other hosts without depending on Discobot's Svelte app contexts.

## Goals

- Package reusable UI/rendering primitives as standards-based custom elements.
- Keep component internals Svelte-friendly while exposing a DOM-first API.
- Avoid dependencies on Discobot app/session/thread contexts.
- Let hosts control application policy such as link safety, downloads, approvals, and persistence.
- Keep existing Svelte app components working while Web Components are introduced incrementally.

## Directory pattern

Each component family lives in its own folder under `ui/src/lib/web-components/`:

```text
web-components/
  conversation/
    DiscoConversation.svelte
    define.ts
    dom.ts
    index.ts
    types.ts
  markdown/
    DiscoMarkdown.svelte
    define.ts
    dom.ts
    index.ts
    types.ts
```

Use this shape for new Web Component families:

- `DiscoX.svelte` contains the custom element implementation.
- `define.ts` is a side-effect entrypoint that registers the custom element.
- `index.ts` exports public types/helpers and the Svelte component for bundlers.
- `types.ts` contains public DOM element interfaces, event detail types, and event maps.
- `dom.ts` contains DOM helpers shared within the family.

## Imports and dependency boundaries

Web Components must not import from app-shell contexts or app-only component layers:

- Do not call `useAppContext`, `useSessionContext`, or `useThreadContext`.
- Do not import from `ui/src/lib/components/app/*`.
- Avoid importing Discobot desktop/runtime APIs such as `$lib/shell` from Web Components.

Hosts should adapt Web Component events to app-specific behavior. For example, the Discobot Svelte app wraps `<disco-markdown>` in an AI-owned Svelte wrapper to provide link-safety and image-download policy, but the Web Component itself only emits DOM events.

Imports from reusable libraries are allowed when they are part of the component's own implementation, such as markdown parsing/rendering dependencies or Svelte itself.

## Registration entrypoints

Use `define.ts` for side-effect registration:

```ts
import "./DiscoMarkdown.svelte";
```

Hosts can register a component with:

```ts
import "$lib/web-components/markdown/define";
```

`index.ts` should not be required for registration unless a host intentionally imports it. Keep registration side effects explicit.

## Custom element compilation

Web Component Svelte files use `<svelte:options customElement={...} />` and are compiled as custom elements through the Web Component compile path configured in `ui/svelte.config.js`.

When adding camelCase props that are also represented as HTML attributes, configure explicit attribute mappings:

```svelte
<svelte:options
	customElement={{
		tag: "disco-example",
		props: {
			partId: { attribute: "part-id", type: "String" },
			isAnimating: { attribute: "is-animating", type: "Boolean" },
		},
	}}
/>
```

Do not rely on Svelte's default lowercasing for camelCase public props. Kebab-case attributes are the public HTML API.

## Events

Public Web Component events must be DOM `CustomEvent`s with these options unless there is a specific reason not to:

- `bubbles: true`
- `composed: true`
- `cancelable: true` for host-controlled requests

Event names must use the `disco-` prefix to avoid collisions with host app or third-party event names.

Examples:

- `disco-link-click`
- `disco-image-download`
- `disco-render-error`
- `disco-link-open-request`
- `disco-tool-approval-request`

Use cancelable events for actions where the host may take over behavior. If `dispatchEvent` returns `false`, the component should not continue its fallback behavior.

Use non-cancelable events for notifications where the component has already updated internal state, such as scroll-state or expansion changes.

## Event typing

Each family should expose:

- detail types, for example `MarkdownLinkClickDetail`
- an event map, for example `DiscoMarkdownEventMap`
- typed `addEventListener` / `removeEventListener` overloads on the public element interface
- the base DOM listener overloads so the element remains structurally compatible with `HTMLElement` / `Node`
- a global `HTMLElementTagNameMap` entry

Pattern:

```ts
export type DiscoExampleEventMap = {
	"disco-example-action": CustomEvent<DiscoExampleActionDetail>;
};

export interface DiscoExampleElement extends HTMLElement {
	addEventListener<K extends keyof DiscoExampleEventMap>(
		type: K,
		listener: (
			this: DiscoExampleElement,
			event: DiscoExampleEventMap[K],
		) => void,
		options?: boolean | AddEventListenerOptions,
	): void;
	addEventListener(
		type: string,
		listener: EventListenerOrEventListenerObject | null,
		options?: boolean | AddEventListenerOptions,
	): void;
}
```

## Host-controlled behavior

Prefer events over callbacks for public host integration because events work across frameworks and plain DOM hosts.

Use properties/methods for data and imperative updates:

- properties for current state, such as `markdown`, `mode`, `isAnimating`
- methods for imperative mutation, such as `setMarkdown()` and `appendMarkdown()`
- events for host policy decisions, such as link opening, image downloads, and tool approvals

Avoid hardcoding app policy in Web Components. Examples of host policy:

- whether a link is safe to open
- how files are downloaded
- how tool approval is displayed or persisted
- how unknown part actions are handled

## Styling

Web Components should be styled internally and expose host customization through web-platform mechanisms:

- CSS custom properties for themes/tokens
- `part` attributes for stable styling hooks
- stable `data-*` attributes for behavior/testing hooks

Do not depend on Discobot app Tailwind classes being available outside the component. App wrappers may add app-specific spacing or layout around the Web Component, but reusable Web Components should remain self-contained.

### Token layering

Component families should use the same token layering pattern:

1. **App/design-system tokens** such as `--background`, `--foreground`, `--border`, `--primary`, and `--font-sans`.
2. **Shared Discobot tokens** such as `--disco-background`, `--disco-foreground`, `--disco-border`, `--disco-primary`, and `--disco-font-sans`.
3. **Component-family tokens** such as `--disco-markdown-background` or `--disco-conversation-background`.
4. **Element-specific tokens** for local layout and density, such as `--disco-message-user-max-width`.

The shared `--disco-*` tokens are the bridge between component families. For example, `<disco-message-content format="markdown">` nests `<disco-markdown>`, so the conversation renderer should pass theme values through shared `--disco-*` tokens rather than styling markdown internals directly.

Component-family tokens should map from shared tokens, and shared tokens should map from common app/design-system tokens. This keeps plain HTML hosts ergonomic while allowing Discobot's app theme to flow through naturally:

```css
disco-conversation {
	--disco-conversation-background: var(
		--disco-background,
		var(--background, #ffffff)
	);
}

disco-markdown {
	--disco-markdown-background: var(
		--disco-background,
		var(--background, #ffffff)
	);
}
```

Consumers that want broad theming should set shared tokens:

```css
my-chat-shell {
	--disco-background: #0b1020;
	--disco-foreground: #f8fafc;
	--disco-border: #334155;
	--disco-primary: #60a5fa;
}
```

Consumers that want family-specific tuning should set family tokens:

```css
disco-conversation {
	--disco-conversation-padding: 1.25rem;
	--disco-conversation-max-width: 56rem;
}

disco-markdown {
	--disco-markdown-radius: 0.5rem;
}
```

## Component documentation card

Document each public custom element with the same component card. The goal is
to make the rendered DOM API, styling surface, and box-model layout inspectable
without opening the implementation. Keep cards close to the component family
and use one file per custom element named `[component-tag]-card.md`, for
example `conversation/disco-tool-ask-user-question-card.md`.

Use this template:

````md
## `<disco-example>`

One-sentence purpose and the component's role in the family.

### Status

- Stability: experimental | stable | deprecated
- Rendering role: structural | content | tool | interactive | metadata
- Shadow DOM: yes | no
- Primary source of data: attributes | properties | semantic children | slots |
  text content | JSON metadata | methods

### Public API

#### Attributes and properties

| Attribute | Property | Type     | Default     | Reflects | Description             |
| --------- | -------- | -------- | ----------- | -------- | ----------------------- |
| `part-id` | `partId` | `string` | `undefined` | no       | Stable part identifier. |

Rules:

- Prefer kebab-case attributes as the public HTML API.
- Note whether a value is an attribute, a property-only value, or both.
- Include accepted string literal values for enums.

#### Methods

| Method      | Parameters | Returns | Description                  |
| ----------- | ---------- | ------- | ---------------------------- |
| `refresh()` | none       | `void`  | Re-reads semantic child DOM. |

Use `None` if the component has no public methods.

#### Events

| Event                  | Cancelable | Detail           | When emitted                       |
| ---------------------- | ---------- | ---------------- | ---------------------------------- |
| `disco-example-action` | yes        | `{ id: string }` | User requests an app-owned action. |

Use DOM events for host integration. Events that cross shadow DOM should be
`bubbles: true` and `composed: true`.

### Supported child elements

Describe the semantic light-DOM contract. Prefer this for user-visible or
interactive content.

```html
<disco-example id="example-1">
	<disco-example-header>Title</disco-example-header>
	<disco-example-item value="one">
		<span slot="label">One</span>
		<span slot="description">First option.</span>
	</disco-example-item>
</disco-example>
```

| Child                    | Count | Required | Purpose                 |
| ------------------------ | ----- | -------- | ----------------------- |
| `<disco-example-header>` | 0..1  | no       | Visible title content.  |
| `<disco-example-item>`   | 0..n  | no       | Repeated semantic item. |

#### Slots

| Slot    | Allowed on             | Fallback  | Description          |
| ------- | ---------------------- | --------- | -------------------- |
| `label` | `<disco-example-item>` | item text | Short visible label. |

Use `None` if the component does not expose slots.

#### Text content

Describe whether direct text nodes are meaningful, ignored, or fallback
content.

### Styling API

#### CSS custom properties

| Token                        | Default/fallback chain                       | Applies to   |
| ---------------------------- | -------------------------------------------- | ------------ |
| `--disco-example-background` | `--disco-background`, `--background`, `#fff` | Root surface |

Document tokens in precedence order. Include the fallback chain so dark-mode
and embedding bugs can be diagnosed from the docs.

#### Shadow parts

| Part        | Element    | Description       |
| ----------- | ---------- | ----------------- |
| `container` | root block | Outer visual box. |

Use `None` if the component exposes no `part` attributes.

#### Stable data hooks

| Hook         | Description           |
| ------------ | --------------------- |
| `data-state` | Current visual state. |

Use `data-*` hooks for testing/behavior only, not public styling.

### Layout and box model

Summarize the visual structure with a block diagram and computed layout table.
Values should be expressed in component tokens or concrete CSS units.

```text
<disco-example> host
└─ [part=container] block
   margin: 0
   border: 1px solid var(--disco-example-border)
   border-radius: var(--disco-radius)
   padding: 1rem
   display: flex
   gap: 0.75rem
   ├─ [part=header] block
   │  margin: 0
   │  padding: 0
   └─ [part=content] block
      margin: 0
      padding: 0
```

| Box                | Display       | Margin | Border | Padding | Gap       | Sizing notes              |
| ------------------ | ------------- | ------ | ------ | ------- | --------- | ------------------------- |
| `:host`            | `block`       | `0`    | none   | none    | n/a       | Width follows parent.     |
| `[part=container]` | `flex column` | `0`    | `1px`  | `1rem`  | `0.75rem` | `box-sizing: border-box`. |

Include overflow behavior, intrinsic sizing, and whether children participate in
normal flow, grid, flexbox, or absolute positioning.

### States

| State     | Trigger           | Visual/layout effect       |
| --------- | ----------------- | -------------------------- |
| `loading` | `state="loading"` | Shows placeholder content. |

Use `None` if the component has no state variants.

### Accessibility

- Role/name/description strategy.
- Keyboard interactions.
- Focus management.
- ARIA attributes and live regions.

### Examples

#### Minimal

```html
<disco-example>Plain content</disco-example>
```

#### Full

```html
<disco-example part-id="part-1" state="ready"> ... </disco-example>
```

### Notes and constraints

- Important implementation constraints.
- Host responsibilities.
- Known limitations.
````

For components with many children, keep the top-level card focused on the host
element and add smaller cards for each semantic child element.

## Markdown component decisions

`markdown/` is the reusable markdown renderer package.

- `<disco-markdown>` replaces the reusable rendering role of the older Svelte streamdown renderer.
- Markdown parsing/rendering logic is copied into the Web Component package so the existing Svelte markdown path can continue to work until migration is complete.
- Attribution for Streamdown-derived behavior lives in `markdown/ATTRIBUTION.md`.
- The component uses Svelte for host/property/event wiring and delegates imperative block rendering to local helper modules.
- Public events are:
  - `disco-link-click`
  - `disco-image-download`
  - `disco-render-error`
- Link and image behavior is host-controlled. The Discobot app wrapper handles link safety and desktop/browser download behavior.
- The component supports full-property updates through `markdown` and imperative updates through `setMarkdown()` / `appendMarkdown()`.
- Slotted text may be used for initial markdown content, but hosts should prefer the `markdown` property for frequently updated or large content.

## Conversation component decisions

`conversation/` is the reusable conversation display package.

- Components model messages, turns, content, reasoning, tool calls, attachments, metadata, and events as custom elements.
- Data can be supplied through attributes/properties, JSON script children, slots, and imperative methods depending on shape and update pattern.
- Host-level actions are emitted as cancelable `disco-*` events rather than callbacks.
- Public conversation events are typed in `conversation/types.ts` and include:
  - `disco-link-open-request`
  - `disco-attachment-open-request`
  - `disco-tool-approval-request`
  - `disco-tool-approval-response`
  - `disco-message-copy-request`
  - `disco-message-retry-request`
  - `disco-selection-comment-request`
  - `disco-scroll-state-change`
  - `disco-expand-change`
  - `disco-part-action`
- Components should acquire their custom-element host via a bound internal element plus `getCustomElementHost(...)`, not by relying on direct `$host()` access in a way that complicates typing/tooling.

## Svelte app adapters

Svelte app adapters may wrap Web Components when the app needs to add Discobot-specific policy. These wrappers belong in the owning component area according to the normal component layout rules.

For example, the app's AI markdown wrapper lives with the AI streamdown components because it depends on AI-owned link-safety UI. The reusable Web Component remains in `web-components/markdown` and does not import that app/AI policy.
