# `<disco-tool-edit>` component card

Optimized file edit renderer for the `Edit` tool. This component is a semantic custom-element wrapper around the shared optimized tool view, so app renderers can emit tool-specific HTML instead of mounting the Svelte optimized-renderer bridge.

## Status

- Stability: experimental
- Rendering role: content/tool/optimized
- Shadow DOM: yes
- Primary source of data: attributes

## Public API

### Attributes and properties

| Attribute      | Property      | Type      | Default           | Reflects | Description                                       |
| -------------- | ------------- | --------- | ----------------- | -------- | ------------------------------------------------- |
| `part-id`      | `partId`      | `string`  | `undefined`       | no       | Stable conversation part id.                      |
| `call-id`      | `callId`      | `string`  | `undefined`       | no       | Tool call id.                                     |
| `state`        | `state`       | `string`  | `input-available` | no       | Tool state used for the status pill.              |
| `title`        | `title`       | `string`  | `undefined`       | no       | Optional header title override.                   |
| `input`        | `input`       | `string`  | `""`              | no       | JSON-encoded tool input, or plain text fallback.  |
| `output`       | `output`      | `string`  | `""`              | no       | JSON-encoded tool output, or plain text fallback. |
| `error-text`   | `errorText`   | `string`  | `""`              | no       | Error text shown below the optimized content.     |
| `default-open` | `defaultOpen` | `boolean` | `false`           | no       | Initial expanded state.                           |

### Methods

None.

### Events

None. The component owns only local disclosure/raw-view state.

## Supported child elements

None. Data is supplied through attributes so the component can be rendered from serialized conversation parts.

## Slots

None.

## Text content

Ignored.

## Styling API

### CSS custom properties

| Token                      | Default/fallback chain          | Applies to                 |
| -------------------------- | ------------------------------- | -------------------------- |
| `--disco-background`       | `--background`, `#fff`          | Content and raw surfaces.  |
| `--disco-foreground`       | `--foreground`, `#111827`       | Main text.                 |
| `--disco-border`           | `--border`, `#e5e7eb`           | Content borders.           |
| `--disco-muted`            | `--muted`, `#f3f4f6`            | Status and hover surfaces. |
| `--disco-muted-foreground` | `--muted-foreground`, `#6b7280` | Header and labels.         |
| `--disco-font-mono`        | `--font-mono`, monospace        | Raw/body code text.        |

### Shadow parts

| Part         | Element | Description                                    |
| ------------ | ------- | ---------------------------------------------- |
| `container`  | wrapper | Unbordered outer stack.                        |
| `header`     | row     | Disclosure title, status, and raw toggle.      |
| `trigger`    | button  | Expands/collapses optimized content.           |
| `title`      | text    | Computed or overridden tool title.             |
| `status`     | pill    | Human-readable tool state.                     |
| `raw-toggle` | button  | Switches between optimized and raw JSON views. |
| `raw`        | pre     | Raw normalized payload.                        |
| `content`    | section | Optimized content wrapper.                     |
| `summary`    | dl-like | Key/value summary rows.                        |
| `body`       | pre     | Main preview/output body.                      |
| `error`      | div     | Error text.                                    |

### Stable data hooks

| Hook             | Description                              |
| ---------------- | ---------------------------------------- |
| `data-state`     | Current tool state.                      |
| `data-tool-name` | Source tool name handled by the wrapper. |

## Layout and box model

```text
<disco-tool-edit> host
└─ [part=container] unbordered stack
   ├─ [part=header] flex row
   │  ├─ [part=trigger] title/status disclosure button
   │  └─ [part=raw-toggle] icon button
   └─ open content
      ├─ [part=raw] raw JSON view, or
      └─ [part=content] bordered optimized surface
         ├─ [part=summary] key/value rows
         ├─ [part=body] preview/output
         └─ [part=error] optional error text
```

| Box                | Display | Margin | Border | Padding  | Gap      | Sizing notes                  |
| ------------------ | ------- | ------ | ------ | -------- | -------- | ----------------------------- |
| `:host`            | `block` | `0`    | none   | none     | n/a      | Width follows parent.         |
| `[part=container]` | `flex`  | `0`    | none   | none     | `.75rem` | Column stack.                 |
| `[part=content]`   | `flex`  | `0`    | `1px`  | `.75rem` | `.75rem` | Bordered optimized surface.   |
| `[part=raw]`       | `block` | `0`    | `1px`  | `.75rem` | n/a      | Scrollable preformatted JSON. |

## States

| State         | Trigger           | Visual/layout effect                        |
| ------------- | ----------------- | ------------------------------------------- |
| open/closed   | Header trigger    | Shows or hides raw/optimized content.       |
| raw/optimized | Raw toggle        | Swaps content surface for raw JSON payload. |
| tool state    | `state` attribute | Updates status label and `data-state`.      |

## Accessibility

- Uses native buttons for disclosure and raw-view toggling.
- Disclosure button exposes `aria-expanded`.
- Raw toggle exposes `aria-pressed` and a state-specific label.

## Examples

```html
<disco-tool-edit
	part-id="message-1:part-0"
	call-id="toolu_123"
	state="output-available"
	input='{"example":true}'
	output='{"output":"Done"}'
></disco-tool-edit>
```

## Notes and constraints

- The wrapper maps one optimized tool name to the shared optimized view.
- The renderer should pass JSON strings for structured `input` and `output`.
- For unknown tools, use `<disco-tool-call>` as the generic fallback.
