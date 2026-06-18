# `<disco-message-content>` component card

Message text renderer for markdown or plain text content.

## Status

- Stability: experimental
- Rendering role: content/tool/structural
- Shadow DOM: yes
- Primary source of data: attributes and semantic children/text

## Public API

### Attributes and properties

| Attribute | Property | Type               | Default     | Reflects | Description       |
| --------- | -------- | ------------------ | ----------- | -------- | ----------------- |
| `format`  | `format` | `markdown \| text` | `text`      | no       | Rendering format. |
| `part-id` | `partId` | `string`           | `undefined` | no       | Stable part id.   |

### Methods

| Method            | Parameters     | Returns | Description                                              |
| ----------------- | -------------- | ------- | -------------------------------------------------------- |
| `appendTextDelta` | `text: string` | `void`  | Appends streamed text to the host content and rerenders. |

### Events

| Event                     | Detail                                                 | Bubbles | Composed | Cancelable | Description                                                                |
| ------------------------- | ------------------------------------------------------ | ------- | -------- | ---------- | -------------------------------------------------------------------------- |
| `disco-link-open-request` | `{ url: string; messageId?: string; partId?: string }` | yes     | yes      | yes        | Emitted before opening a plain-text or markdown link. Prevent to block it. |

## Supported child elements

Supports direct text content. Markdown format renders text through `<disco-markdown>`.

## Slots

None.

## Text content

Direct text is rendered when the component supports a default slot; otherwise it is ignored.

## Styling API

### CSS custom properties

| Token                                   | Default/fallback chain                                      | Applies to           |
| --------------------------------------- | ----------------------------------------------------------- | -------------------- |
| `--disco-conversation-foreground`       | `--disco-foreground`, `--foreground`, `#111827`             | Primary text.        |
| `--disco-conversation-muted-foreground` | `--disco-muted-foreground`, `--muted-foreground`, `#6b7280` | Secondary text.      |
| `--disco-conversation-background`       | `--disco-background`, `--background`, `#fff`                | Background surfaces. |
| `--disco-conversation-border`           | `--disco-border`, `--border`, `#e5e7eb`                     | Borders.             |
| `--disco-conversation-font-sans`        | `--disco-font-sans`, `--font-sans`, `system-ui`             | Font family.         |

### Shadow parts

| Part        | Element           | Description         |
| ----------- | ----------------- | ------------------- |
| `container` | outer content box | Text content block. |
| `markdown`  | markdown renderer | Markdown output.    |
| `text`      | pre-wrap text     | Plain text output.  |

### Stable data hooks

None.

## Layout and box model

```text
<disco-message-content> host
└─ [part=container]
   width/max-width from message tokens
   padding/radius/background from message tokens
   ├─ [part=markdown] for markdown
   └─ [part=text] for text
```

| Box                | Display            | Margin | Border             | Padding            | Gap                | Sizing notes                                          |
| ------------------ | ------------------ | ------ | ------------------ | ------------------ | ------------------ | ----------------------------------------------------- |
| `:host`            | `block`            | `0`    | none               | none               | n/a                | Width follows parent unless component is inline-like. |
| `[part=container]` | component-specific | `0`    | component-specific | component-specific | component-specific | See block diagram.                                    |

## States

None.

## Accessibility

- Uses native buttons/inputs where interactive.
- Host should provide surrounding landmarks and labels as needed.

## Examples

```html
<disco-message-content></disco-message-content>
```

## Notes and constraints

- Host adapters own app-specific policy and persistence.
- Styling should use documented tokens and parts.
