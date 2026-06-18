# `<disco-step-group>` component card

Collapsible group for compacting multiple assistant steps.

## Status

- Stability: experimental
- Rendering role: content/tool/structural
- Shadow DOM: yes
- Primary source of data: attributes and semantic children/text

## Public API

### Attributes and properties

| Attribute | Property | Type      | Default | Reflects | Description              |
| --------- | -------- | --------- | ------- | -------- | ------------------------ |
| `open`    | `open`   | `boolean` | `false` | no       | Expanded state.          |
| `label`   | `label`  | `string`  | `Steps` | no       | Visible collapsed label. |

### Methods

None.

### Events

| Event                 | Cancelable | Detail              | When emitted              |
| --------------------- | ---------- | ------------------- | ------------------------- |
| `disco-expand-change` | no         | `{ open: boolean }` | Group open state changes. |

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

| Part        | Element         | Description             |
| ----------- | --------------- | ----------------------- |
| `container` | outer block     | Step group wrapper.     |
| `trigger`   | button          | Expandable label row.   |
| `line`      | decorative line | Vertical grouping line. |
| `label`     | text            | Group label.            |
| `content`   | details         | Slotted grouped steps.  |

### Stable data hooks

| Hook         | Description           |
| ------------ | --------------------- |
| `data-state` | Current visual state. |

## Layout and box model

```text
<disco-step-group> host
└─ [part=container]
   ├─ [part=trigger] button row
   │  ├─ [part=line]
   │  └─ [part=label]
   └─ [part=content] when open
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
<disco-step-group></disco-step-group>
```

## Notes and constraints

- Host adapters own app-specific policy and persistence.
- Styling should use documented tokens and parts.
