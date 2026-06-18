# `<disco-option>` component card

Semantic option definition consumed by `<disco-question>`.

## Status

- Stability: experimental
- Rendering role: metadata/semantic child
- Shadow DOM: yes
- Primary source of data: attributes, slots, and text content

## Public API

### Attributes and properties

None.

### Methods

None.

### Events

None.

## Supported child elements

Used only as a semantic child. `disco-option` renders as `display: contents` and is read by its parent component.

## Slots

| Slot    | Allowed on       | Fallback    | Description                |
| ------- | ---------------- | ----------- | -------------------------- |
| default | `<disco-option>` | direct text | Semantic fallback content. |

## Text content

Direct text is meaningful as fallback content for the parent parser.

## Styling API

### CSS custom properties

None. This element exposes no visual tokens.

### Shadow parts

None.

### Stable data hooks

None.

## Layout and box model

```text
<disco-option> host
└─ default slot
   display: contents
```

| Box     | Display    | Margin | Border | Padding | Gap | Sizing notes                  |
| ------- | ---------- | ------ | ------ | ------- | --- | ----------------------------- |
| `:host` | `contents` | `0`    | none   | none    | n/a | Does not create a visual box. |

## States

None.

## Accessibility

- Accessibility is provided by the consuming parent component.
- This element is semantic input, not a standalone control.

## Examples

```html
<disco-option value="web">
	<span slot="label">Web components</span>
</disco-option>
```

## Notes and constraints

- Not intended to be styled directly.
- Parent components parse attributes, slots, and text content.
