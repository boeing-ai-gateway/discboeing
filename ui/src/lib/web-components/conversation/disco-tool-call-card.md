# `<disco-tool-call>` component card

Generic collapsible tool-call card and fallback for unimplemented optimized tool components.

## Status

- Stability: experimental
- Rendering role: content/tool/structural
- Shadow DOM: yes
- Primary source of data: attributes and semantic children/text

## Public API

### Attributes and properties

| Attribute     | Property     | Type      | Default           | Reflects | Description            |
| ------------- | ------------ | --------- | ----------------- | -------- | ---------------------- |
| `part-id`     | `partId`     | `string`  | `undefined`       | no       | Stable part id.        |
| `call-id`     | `callId`     | `string`  | `""`              | no       | Tool call id.          |
| `name`        | `name`       | `string`  | `Tool`            | no       | Tool name.             |
| `state`       | `state`      | `string`  | `input-available` | no       | Tool state.            |
| `title`       | `title`      | `string`  | `undefined`       | no       | Header title override. |
| `approval-id` | `approvalId` | `string`  | `undefined`       | no       | Approval id.           |
| `open`        | `open`       | `boolean` | `false`           | no       | Expanded state.        |

### Methods

None.

### Events

| Event                          | Cancelable | Detail                        | When emitted             |
| ------------------------------ | ---------- | ----------------------------- | ------------------------ |
| `disco-tool-approval-request`  | yes        | tool approval detail          | Review action requested. |
| `disco-tool-approval-response` | no         | tool approval response detail | Approve/Deny clicked.    |
| `disco-expand-change`          | no         | `{ partId?, open }`           | Open state changes.      |

## Supported child elements

Supports default slot content unless noted by the component role. For tool input/output, content may include a JSON script child.

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

| Part        | Element    | Description            |
| ----------- | ---------- | ---------------------- |
| `container` | card       | Outer tool card.       |
| `header`    | button row | Tool title/status row. |
| `title`     | text       | Tool title.            |
| `status`    | text       | State label.           |
| `actions`   | button row | Approval actions.      |
| `content`   | details    | Slotted input/output.  |

### Stable data hooks

| Hook         | Description           |
| ------------ | --------------------- |
| `data-state` | Current visual state. |

## Layout and box model

```text
<disco-tool-call> host
└─ [part=container] card
   ├─ [part=header] button flex row
   │  ├─ status dot
   │  ├─ [part=title]
   │  └─ [part=status]
   ├─ [part=actions] approval buttons when requested
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
<disco-tool-call></disco-tool-call>
```

## Notes and constraints

- Host adapters own app-specific policy and persistence.
- Styling should use documented tokens and parts.
