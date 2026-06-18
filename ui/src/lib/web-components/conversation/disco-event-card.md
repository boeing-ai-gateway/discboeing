# `<disco-event>` component card

Generic collapsible event card for unsupported or structured events.

## Status

- Stability: experimental
- Rendering role: content/tool/structural
- Shadow DOM: yes
- Primary source of data: attributes and semantic children/text

## Public API

### Attributes and properties

| Attribute | Property  | Type      | Default     | Reflects | Description                  |
| --------- | --------- | --------- | ----------- | -------- | ---------------------------- |
| `part-id` | `partId`  | `string`  | `undefined` | no       | Stable part id.              |
| `kind`    | `kind`    | `string`  | `event`     | no       | Event kind.                  |
| `title`   | `title`   | `string`  | derived     | no       | Visible title.               |
| `summary` | `summary` | `string`  | `undefined` | no       | Collapsed summary.           |
| `open`    | `open`    | `boolean` | `false`     | no       | Whether content is expanded. |

### Methods

None.

### Events

None.

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

| Part        | Element    | Description            |
| ----------- | ---------- | ---------------------- |
| `container` | card       | Outer event card.      |
| `header`    | button row | Expandable header.     |
| `content`   | details    | Slotted event details. |

### Stable data hooks

None.

## Layout and box model

```text
<disco-event> host
└─ [part=container] card
   ├─ [part=header] button
   └─ [part=content] slotted details when open
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
<disco-event></disco-event>
```

## Notes and constraints

- Host adapters own app-specific policy and persistence.
- Styling should use documented tokens and parts.
