# `<disco-tool-output>` component card

Tool output payload display for generic tool calls.

## Status

- Stability: experimental
- Rendering role: content/tool/structural
- Shadow DOM: yes
- Primary source of data: attributes and semantic children/text

## Public API

### Attributes and properties

| Attribute | Property | Type  | Default | Reflects | Description |
| --------- | -------- | ----- | ------- | -------- | ----------- | ------------------------ |
| `format`  | `format` | `json | text`   | `text`   | no          | Output rendering format. |

### Methods

None.

### Events

None.

## Supported child elements

Supports either direct text content or `<script type="application/json">` when `format="json"`.

## Slots

None.

## Text content

Direct text is rendered when the component supports a default slot; otherwise it is ignored.

## Styling API

### CSS custom properties

| Token                      | Default/fallback chain          | Applies to      |
| -------------------------- | ------------------------------- | --------------- |
| `--disco-background`       | `--background`, `#fff`          | Surfaces.       |
| `--disco-foreground`       | `--foreground`, `#111827`       | Text.           |
| `--disco-border`           | `--border`, `#e5e7eb`           | Borders.        |
| `--disco-muted`            | `--muted`, `#f3f4f6`            | Muted surfaces. |
| `--disco-muted-foreground` | `--muted-foreground`, `#6b7280` | Secondary text. |

### Shadow parts

| Part      | Element        | Description              |
| --------- | -------------- | ------------------------ |
| `content` | pre/code block | Rendered output content. |

### Stable data hooks

None.

## Layout and box model

```text
<disco-tool-output> host
└─ [part=content] pre-like block
   margin: 0
   border: 1px solid var(--disco-border)
   padding: 0.75rem
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
<disco-tool-output></disco-tool-output>
```

## Notes and constraints

- Host adapters own app-specific policy and persistence.
- Styling should use documented tokens and parts.
