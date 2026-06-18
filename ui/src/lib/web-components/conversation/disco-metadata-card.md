# `<disco-metadata>` component card

Hidden structured metadata holder for raw/backing data that is not primary user-visible HTML.

## Status

- Stability: experimental
- Rendering role: metadata
- Shadow DOM: yes
- Primary source of data: JSON script child and methods

## Public API

### Attributes and properties

None.

### Methods

| Method            | Parameters           | Returns              | Description                                         |
| ----------------- | -------------------- | -------------------- | --------------------------------------------------- |
| `setValue(value)` | `DiscoMetadataValue` | `void`               | Writes a JSON script child and refreshes `value`.   |
| `value`           | property             | `DiscoMetadataValue` | Current parsed metadata object on the host element. |

### Events

None.

## Supported child elements

Supports one direct `<script type="application/json">` child.

## Slots

None.

## Text content

Direct text is ignored unless it is inside the JSON script child.

## Styling API

### CSS custom properties

None. This element is hidden.

### Shadow parts

None.

### Stable data hooks

None.

## Layout and box model

```text
<disco-metadata> host
└─ hidden span
   └─ slot for JSON script
```

| Box     | Display | Margin | Border | Padding | Gap | Sizing notes   |
| ------- | ------- | ------ | ------ | ------- | --- | -------------- |
| `:host` | `none`  | `0`    | none   | none    | n/a | No visual box. |

## States

None.

## Accessibility

- Hidden metadata should not contain user-visible content.
- Invalid/missing JSON resolves to an empty object.

## Examples

```html
<disco-metadata>
	<script type="application/json">
		{ "turnId": "t1" }
	</script>
</disco-metadata>
```

## Notes and constraints

- Prefer semantic child elements for visible/interactable content.
- Use metadata for raw payloads, provenance, or debugging.
