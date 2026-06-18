# `<disco-attachment>` component card

Inline file/attachment chip with preview, filename, and optional media type.

## Status

- Stability: experimental
- Rendering role: content/tool/structural
- Shadow DOM: yes
- Primary source of data: attributes and semantic children/text

## Public API

### Attributes and properties

| Attribute    | Property    | Type     | Default      | Reflects | Description                |
| ------------ | ----------- | -------- | ------------ | -------- | -------------------------- |
| `part-id`    | `partId`    | `string` | `undefined`  | no       | Stable part id.            |
| `kind`       | `kind`      | `string` | `file`       | no       | Attachment kind.           |
| `src`        | `src`       | `string` | `undefined`  | no       | Open/download source URL.  |
| `filename`   | `filename`  | `string` | `Attachment` | no       | Visible filename.          |
| `media-type` | `mediaType` | `string` | `undefined`  | no       | Optional MIME/media label. |

### Methods

None.

### Events

| Event                           | Cancelable | Detail                                     | When emitted                   |
| ------------------------------- | ---------- | ------------------------------------------ | ------------------------------ |
| `disco-attachment-open-request` | yes        | `{ partId?, src?, filename?, mediaType? }` | User activates the attachment. |

## Supported child elements

No semantic children. Activation is based on attributes.

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

| Part        | Element          | Description                    |
| ----------- | ---------------- | ------------------------------ |
| `container` | link/button chip | Outer attachment chip.         |
| `preview`   | icon box         | File preview icon area.        |
| `info`      | text stack       | Filename and media type stack. |
| `filename`  | text             | Filename label.                |

### Stable data hooks

None.

## Layout and box model

```text
<disco-attachment> host
└─ [part=container] inline-flex chip
   border: var(--disco-attachment-border)
   padding: var(--disco-attachment-padding)
   gap: var(--disco-attachment-gap)
   ├─ [part=preview]
   └─ [part=info]
      └─ [part=filename]
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
<disco-attachment></disco-attachment>
```

## Notes and constraints

- Host adapters own app-specific policy and persistence.
- Styling should use documented tokens and parts.
