# `<disco-turn>` component card

Collapsible/structured turn grouping one or more user and assistant messages.

## Status

- Stability: experimental
- Rendering role: content/tool/structural
- Shadow DOM: yes
- Primary source of data: attributes and semantic children/text

## Public API

### Attributes and properties

| Attribute | Property | Type      | Default | Reflects | Description     |
| --------- | -------- | --------- | ------- | -------- | --------------- |
| `open`    | `open`   | `boolean` | `false` | no       | Expanded state. |

### Methods

None.

### Events

| Event                 | Cancelable | Detail              | When emitted             |
| --------------------- | ---------- | ------------------- | ------------------------ |
| `disco-expand-change` | no         | `{ open: boolean }` | Turn open state changes. |

## Supported child elements

Supports default slot content unless noted by the component role. For tool input/output, content may include a JSON script child.

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

| Part          | Element         | Description         |
| ------------- | --------------- | ------------------- |
| `container`   | outer turn      | Turn wrapper.       |
| `trigger`     | button          | State/turn trigger. |
| `state-line`  | decorative line | Turn marker line.   |
| `state-label` | text            | Turn state label.   |
| `content`     | message stack   | Slotted messages.   |

### Stable data hooks

| Hook         | Description           |
| ------------ | --------------------- |
| `data-state` | Current visual state. |

## Layout and box model

```text
<disco-turn> host
└─ [part=container]
   padding-top may use var(--disco-turn-spacing) externally
   ├─ [part=trigger] optional turn row
   └─ [part=content] flex column message stack
```

| Box                | Display            | Margin | Border             | Padding            | Gap                | Sizing notes                                          |
| ------------------ | ------------------ | ------ | ------------------ | ------------------ | ------------------ | ----------------------------------------------------- |
| `:host`            | `block`            | `0`    | none               | none               | n/a                | Width follows parent unless component is inline-like. |
| `[part=container]` | component-specific | `0`    | component-specific | component-specific | component-specific | See block diagram.                                    |

## States

| State  | Trigger                   | Visual/layout effect           |
| ------ | ------------------------- | ------------------------------ |
| `open` | `open` attribute/property | Shows slotted/details content. |

## Accessibility

- Uses native buttons/inputs where interactive.
- Host should provide surrounding landmarks and labels as needed.

## Examples

```html
<disco-turn></disco-turn>
```

## Notes and constraints

- Host adapters own app-specific policy and persistence.
- Styling should use documented tokens and parts.
