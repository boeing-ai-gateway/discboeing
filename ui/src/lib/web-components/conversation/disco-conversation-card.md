# `<disco-conversation>` component card

Root scrollable conversation surface. It sets conversation-level theme tokens, width, padding, and scroll behavior for slotted messages/turns.

## Status

- Stability: experimental
- Rendering role: structural
- Shadow DOM: yes
- Primary source of data: semantic children and attributes

## Public API

### Attributes and properties

| Attribute     | Property     | Type                                | Default | Reflects | Description                                           |
| ------------- | ------------ | ----------------------------------- | ------- | -------- | ----------------------------------------------------- |
| `status`      | `status`     | `idle \| loading \| ready \| error` | `ready` | no       | Conversation loading/status marker.                   |
| `auto-scroll` | `autoScroll` | `boolean`                           | `true`  | no       | Whether the viewport auto-scrolls when content grows. |
| `chat-width`  | `chatWidth`  | `full \| constrained`               | `full`  | no       | Controls max-width behavior for content.              |

### Methods

None.

### Events

| Event                       | Cancelable | Detail                    | When emitted                      |
| --------------------------- | ---------- | ------------------------- | --------------------------------- |
| `disco-scroll-state-change` | no         | `{ atBottom: boolean }`   | Viewport scroll position changes. |
| `disco-expand-change`       | no         | component-specific detail | A descendant expands/collapses.   |

## Supported child elements

<disco-message> and <disco-turn> children are expected. Other flow content is rendered in order through the default slot.

## Slots

None.

## Text content

Direct text is allowed but should generally be wrapped in semantic message elements.

## Styling API

### CSS custom properties

| Token                                   | Default/fallback chain                                      | Applies to                 |
| --------------------------------------- | ----------------------------------------------------------- | -------------------------- |
| `--disco-conversation-foreground`       | `--disco-foreground`, `--foreground`, `#111827`             | Primary text.              |
| `--disco-conversation-muted-foreground` | `--disco-muted-foreground`, `--muted-foreground`, `#6b7280` | Secondary text.            |
| `--disco-conversation-background`       | `--disco-background`, `--background`, `#fff`                | Background surfaces.       |
| `--disco-conversation-border`           | `--disco-border`, `--border`, `#e5e7eb`                     | Borders.                   |
| `--disco-conversation-font-sans`        | `--disco-font-sans`, `--font-sans`, `system-ui`             | Font family.               |
| `--disco-conversation-padding`          | `1rem`                                                      | Viewport padding.          |
| `--disco-conversation-gap`              | `1.5rem`                                                    | Content stack gap.         |
| `--disco-conversation-max-width`        | `48rem`                                                     | Constrained content width. |

### Shadow parts

| Part            | Element          | Description                                    |
| --------------- | ---------------- | ---------------------------------------------- |
| `viewport`      | scroll container | Scrollable root viewport.                      |
| `content`       | content stack    | Flow layout for slotted conversation children. |
| `scroll-button` | button           | Scroll-to-bottom affordance.                   |

### Stable data hooks

| Hook          | Description                                            |
| ------------- | ------------------------------------------------------ |
| `data-status` | Current conversation status on the viewport/container. |

## Layout and box model

```text
<disco-conversation> host
└─ [part=viewport] scroll block
   padding: var(--disco-conversation-padding)
   └─ [part=content] flex column
      gap: var(--disco-conversation-gap)
      max-width: depends on chat-width
```

| Box               | Display        | Margin   | Border | Padding | Gap   | Sizing notes                                   |
| ----------------- | -------------- | -------- | ------ | ------- | ----- | ---------------------------------------------- |
| `:host`           | `block`        | `0`      | none   | none    | n/a   | Fills parent height when parent constrains it. |
| `[part=viewport]` | `block scroll` | `0`      | none   | token   | n/a   | Owns overflow-y scrolling.                     |
| `[part=content]`  | `flex column`  | `0 auto` | none   | `0`     | token | Constrained or full width.                     |

## States

| State     | Trigger            | Visual/layout effect               |
| --------- | ------------------ | ---------------------------------- |
| `ready`   | `status="ready"`   | Normal content.                    |
| `loading` | `status="loading"` | Status hook only; hosts may style. |
| `error`   | `status="error"`   | Status hook only; hosts may style. |

## Accessibility

- Scroll button is a native button.
- Descendant events bubble through this element for host adapters.

## Examples

```html
<disco-conversation chat-width="constrained">
	<disco-message from="user">Hello</disco-message>
</disco-conversation>
```

## Notes and constraints

- Does not fetch or own app state.
- Hosts should listen to descendant `disco-*` events here or above.
