# `<disco-message>` component card

Message container for one user, assistant, system, or synthetic message. It owns user-bubble layout and the vertical part stack.

## Status

- Stability: experimental
- Rendering role: structural
- Shadow DOM: yes
- Primary source of data: attributes and semantic children

## Public API

### Attributes and properties

| Attribute     | Property      | Type      | Default   | Reflects | Description                          |
| ------------- | ------------- | --------- | --------- | -------- | ------------------------------------ | ---------- | -------------------- | ------------------------ |
| `from`        | `from`        | `user     | assistant | system`  | `assistant`                          | no         | Message author role. |
| `state`       | `state`       | `pending  | streaming | complete | error`                               | `complete` | no                   | Message lifecycle state. |
| `provisional` | `provisional` | `boolean` | `false`   | no       | Marks optimistic local user content. |
| `synthetic`   | `synthetic`   | `boolean` | `false`   | no       | Marks generated/system content.      |

### Methods

| Method             | Parameters          | Returns            | Description                         |
| ------------------ | ------------------- | ------------------ | ----------------------------------- |
| `appendPart(init)` | `DiscoPartInit`     | `Element`          | Imperatively append a part element. |
| `setState(state)`  | `DiscoMessageState` | `void`             | Update message state.               |
| `toMessageInit()`  | none                | `DiscoMessageInit` | Serialize message state/content.    |

### Events

None.

## Supported child elements

Expected children include `<disco-message-content>`, `<disco-reasoning>`, `<disco-tool-call>`, `<disco-tool-ask-user-question>`, `<disco-generated-text>`, `<disco-attachment>`, `<disco-browser-activity>`, `<disco-event>`, and `<disco-step-group>`.

## Slots

None.

## Text content

Direct text is rendered through the slot but should normally be wrapped in `<disco-message-content>`.

## Styling API

### CSS custom properties

| Token                                   | Default/fallback chain                                      | Applies to             |
| --------------------------------------- | ----------------------------------------------------------- | ---------------------- |
| `--disco-conversation-foreground`       | `--disco-foreground`, `--foreground`, `#111827`             | Primary text.          |
| `--disco-conversation-muted-foreground` | `--disco-muted-foreground`, `--muted-foreground`, `#6b7280` | Secondary text.        |
| `--disco-conversation-background`       | `--disco-background`, `--background`, `#fff`                | Background surfaces.   |
| `--disco-conversation-border`           | `--disco-border`, `--border`, `#e5e7eb`                     | Borders.               |
| `--disco-conversation-font-sans`        | `--disco-font-sans`, `--font-sans`, `system-ui`             | Font family.           |
| `--disco-message-gap`                   | `0.75rem`                                                   | Gap between parts.     |
| `--disco-message-user-padding`          | `0.75rem 1rem`                                              | User bubble padding.   |
| `--disco-message-user-radius`           | `0.5rem`                                                    | User bubble radius.    |
| `--disco-message-user-max-width`        | `min(30rem, 92%)`                                           | User bubble max width. |

### Shadow parts

| Part        | Element                | Description                   |
| ----------- | ---------------------- | ----------------------------- |
| `container` | outer stack            | Message block.                |
| `content`   | part stack/user bubble | Slotted message content area. |

### Stable data hooks

| Hook               | Description                 |
| ------------------ | --------------------------- |
| `data-from`        | Message author role.        |
| `data-state`       | Message state.              |
| `data-provisional` | Boolean provisional marker. |
| `data-synthetic`   | Boolean synthetic marker.   |

## Layout and box model

```text
<disco-message> host
└─ [part=container] flex column
   gap: 0.5rem
   └─ [part=content] flex column
      gap: var(--disco-message-gap)
      if from=user: width fit-content, background bubble, padding token
```

| Box                | Display       | Margin | Border     | Padding     | Gap      | Sizing notes                                  |
| ------------------ | ------------- | ------ | ---------- | ----------- | -------- | --------------------------------------------- |
| `:host`            | `block`       | `0`    | none       | none        | n/a      | Min-width 0; text inherits conversation font. |
| `[part=container]` | `flex column` | `0`    | none       | `0`         | `0.5rem` | User messages align right.                    |
| `[part=content]`   | `flex column` | `0`    | user: none | user: token | token    | User content becomes a bubble.                |

## States

| State        | Trigger             | Visual/layout effect                    |
| ------------ | ------------------- | --------------------------------------- |
| `from=user`  | `from="user"`       | Right-aligned bubble with user surface. |
| `from!=user` | default             | Full-width assistant stack.             |
| `streaming`  | `state="streaming"` | State hook for hosts.                   |

## Accessibility

- Semantic role is provided by surrounding conversation; no ARIA role by default.
- Slotted interactive children keep their own keyboard behavior.

## Examples

```html
<disco-message from="assistant" state="complete">
	<disco-message-content>Done.</disco-message-content>
</disco-message>
```

## Notes and constraints

- Message metadata should use a `<disco-metadata>` child when needed.
- User bubble styling affects descendants through message tokens.
