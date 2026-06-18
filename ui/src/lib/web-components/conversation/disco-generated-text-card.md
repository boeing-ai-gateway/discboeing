# `<disco-generated-text>` component card

Collapsible generated-text panel used when a user command/skill/script has extra generated content.

## Status

- Stability: experimental
- Rendering role: content/tool/structural
- Shadow DOM: yes
- Primary source of data: attributes and semantic children/text

## Public API

### Attributes and properties

| Attribute      | Property      | Type      | Default          | Reflects | Description                                                  |
| -------------- | ------------- | --------- | ---------------- | -------- | ------------------------------------------------------------ |
| `part-id`      | `partId`      | `string`  | `undefined`      | no       | Stable part id.                                              |
| `label`        | `label`       | `string`  | `Generated text` | no       | Trigger/content label.                                       |
| `open`         | `open`        | `boolean` | `false`          | no       | Expanded state when the component owns its own trigger.      |
| `content-only` | `contentOnly` | `boolean` | `false`          | no       | Always shows content and suppresses the component's trigger. |

### Methods

None.

### Events

None.

## Supported child elements

Supports direct text content in the default slot. The text is rendered as
markdown when the generated-text content is visible.

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

| Part        | Element       | Description                                                        |
| ----------- | ------------- | ------------------------------------------------------------------ |
| `container` | outer block   | Generated-text wrapper.                                            |
| `trigger`   | button        | Expandable label row. Not rendered when `content-only` is present. |
| `label`     | text          | Content label rendered inside the content panel.                   |
| `content`   | content panel | Rendered generated text.                                           |

### Stable data hooks

| Hook                | Description                                        |
| ------------------- | -------------------------------------------------- |
| `data-open`         | Whether content is currently visible.              |
| `data-content-only` | Whether the triggerless content-only mode is used. |

## Layout and box model

```text
<disco-generated-text> host
└─ [part=container]
   ├─ [part=trigger] button, unless content-only
   └─ [part=content] markdown/text when open
```

| Box                | Display            | Margin | Border             | Padding            | Gap                | Sizing notes                                          |
| ------------------ | ------------------ | ------ | ------------------ | ------------------ | ------------------ | ----------------------------------------------------- |
| `:host`            | `block`            | `0`    | none               | none               | n/a                | Width follows parent unless component is inline-like. |
| `[part=container]` | component-specific | `0`    | component-specific | component-specific | component-specific | See block diagram.                                    |

## States

| State          | Trigger                           | Visual/layout effect                                             |
| -------------- | --------------------------------- | ---------------------------------------------------------------- |
| `open`         | `open` attribute/property         | Shows slotted/details content and keeps the trigger visible.     |
| `content-only` | `content-only` attribute/property | Shows slotted/details content and hides the component's trigger. |

## Accessibility

- Uses native buttons/inputs where interactive.
- Host should provide surrounding landmarks and labels as needed.

## Examples

```html
<disco-generated-text label="Generated text" content-only>
	Model-visible command text.
</disco-generated-text>
```

## Notes and constraints

- Host adapters own app-specific policy and persistence.
- Styling should use documented tokens and parts.
