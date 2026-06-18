# `<disco-tool-ask-user-question>` component card

Renders an interactive AskUserQuestion tool from semantic question, option, and
answer children.

## Status

- Stability: experimental
- Rendering role: interactive tool
- Shadow DOM: yes
- Primary source of data: semantic children

## Public API

### Attributes and properties

| Attribute     | Property     | Type     | Default             | Reflects | Description                                                     |
| ------------- | ------------ | -------- | ------------------- | -------- | --------------------------------------------------------------- |
| `part-id`     | `partId`     | `string` | `undefined`         | no       | Stable renderer part id.                                        |
| `call-id`     | `callId`     | `string` | `""`                | no       | Tool call id.                                                   |
| `state`       | `state`      | `string` | `"input-available"` | no       | Tool state, such as `approval-requested` or `output-available`. |
| `approval-id` | `approvalId` | `string` | `undefined`         | no       | Approval/question id used when submitting answers.              |

### Methods

None.

### Events

| Event                        | Cancelable | Detail                                                                                                           | When emitted                                 |
| ---------------------------- | ---------- | ---------------------------------------------------------------------------------------------------------------- | -------------------------------------------- |
| `disco-tool-question-submit` | no         | `{ messageId?: string; partId?: string; callId?: string; approvalId?: string; answers: Record<string, string> }` | User submits answers for a pending question. |

The event bubbles and is composed so app adapters can listen above the custom
element boundary.

## Supported child elements

```html
<disco-tool-ask-user-question
	part-id="message-1:part-0"
	call-id="tool-123"
	approval-id="approval-456"
	state="approval-requested"
>
	<disco-question
		name="primary-renderer"
		header="Primary renderer"
		type="single"
	>
		<span slot="question">Which renderer should we inspect first?</span>
		<span slot="notes">Optional context shown above the picker.</span>

		<disco-option value="current">
			<span slot="label">Current ConversationPane</span>
			<span slot="description">Start with the current app renderer.</span>
		</disco-option>
	</disco-question>

	<disco-answer
		question="Previous question"
		answer="Previous answer"
	></disco-answer>
</disco-tool-ask-user-question>
```

| Child              | Count | Required | Purpose                                               |
| ------------------ | ----- | -------- | ----------------------------------------------------- |
| `<disco-question>` | 0..n  | no       | Defines one question step.                            |
| `<disco-answer>`   | 0..n  | no       | Defines completed answer rows for non-pending output. |

### `<disco-question>` attributes

| Attribute  | Type                 | Default                       | Description                                       |
| ---------- | -------------------- | ----------------------------- | ------------------------------------------------- |
| `name`     | `string`             | question text or generated id | Stable answer key inside the component.           |
| `header`   | `string`             | `Question N`                  | Short step label.                                 |
| `type`     | `single \| multiple` | `single`                      | Option selection mode.                            |
| `multiple` | boolean              | `false`                       | Alternative way to request multi-select behavior. |

### `<disco-question>` slots

| Slot       | Fallback                            | Description              |
| ---------- | ----------------------------------- | ------------------------ |
| `question` | `question` attribute or direct text | Visible question prompt. |
| `notes`    | `notes` attribute                   | Optional context block.  |

### `<disco-option>` attributes

| Attribute     | Type     | Default     | Description                |
| ------------- | -------- | ----------- | -------------------------- |
| `value`       | `string` | label text  | Form value for the option. |
| `label`       | `string` | direct text | Short option label.        |
| `description` | `string` | empty       | Supporting option text.    |

### `<disco-option>` slots

| Slot          | Fallback                         | Description             |
| ------------- | -------------------------------- | ----------------------- |
| `label`       | `label` attribute or direct text | Short option label.     |
| `description` | `description` attribute          | Supporting option text. |

### `<disco-answer>` attributes

| Attribute  | Type     | Default                      | Description    |
| ---------- | -------- | ---------------------------- | -------------- |
| `question` | `string` | `question` slot              | Question text. |
| `answer`   | `string` | `answer` slot or direct text | Answer text.   |

## Styling API

### CSS custom properties

| Token                                     | Default/fallback chain                                      | Applies to                         |
| ----------------------------------------- | ----------------------------------------------------------- | ---------------------------------- |
| `--disco-conversation-foreground`         | `--disco-foreground`, `--foreground`, `#111827`             | Text color.                        |
| `--disco-conversation-muted-foreground`   | `--disco-muted-foreground`, `--muted-foreground`, `#6b7280` | Secondary text.                    |
| `--disco-conversation-background`         | `--disco-background`, `--background`, `#fff`                | Textarea background.               |
| `--disco-conversation-card`               | `--disco-card`, `--card`, `#fff`                            | Outer card surface.                |
| `--disco-conversation-muted`              | `--disco-muted`, `--muted`, `#f3f4f6`                       | Notes and hover surfaces.          |
| `--disco-conversation-border`             | `--disco-border`, `--border`, `#e5e7eb`                     | Borders.                           |
| `--disco-conversation-primary`            | `--disco-primary`, `--primary`, `#2563eb`                   | Selected state and primary button. |
| `--disco-conversation-primary-foreground` | `--primary-foreground`, `#fff`                              | Primary button text.               |
| `--disco-conversation-font-sans`          | `--disco-font-sans`, `--font-sans`, `system-ui`             | Font family.                       |
| `--disco-radius`                          | `0.75rem`                                                   | Border radii.                      |

### Shadow parts

| Part         | Element         | Description                         |
| ------------ | --------------- | ----------------------------------- |
| `container`  | root stack      | Unbordered outer wrapper.           |
| `header`     | header row      | Agent question/status/raw controls. |
| `status`     | status label    | Current tool status label.          |
| `raw-toggle` | button          | Toggles raw question data view.     |
| `raw`        | pre block       | Raw semantic question payload.      |
| `card`       | picker card     | Bordered question picker surface.   |
| `answers`    | answer list     | Completed answer container.         |
| `notes`      | notes block     | Optional context surface.           |
| `steps`      | step navigation | Multi-question step buttons.        |
| `question`   | active question | Current question group.             |
| `options`    | option list     | Current option controls.            |
| `actions`    | footer          | Back/Continue/Submit controls.      |

### Stable data hooks

| Hook         | Description                                           |
| ------------ | ----------------------------------------------------- |
| `data-state` | Mirrors current tool state on the internal container. |

## Layout and box model

```text
<disco-tool-ask-user-question> host
└─ [part=container] flex column wrapper
   margin: 0
   border: none
   padding: 0
   display: flex column
   gap: 1rem
   ├─ [part=header] flex row
   │  ├─ title/status inline group
   │  └─ [part=raw-toggle] button
   ├─ [part=raw] pre block, when raw view is active
   └─ [part=card] card, unless raw view is active
      border: 1px solid var(--disco-conversation-border)
      border-radius: var(--disco-radius)
      padding: 1rem
      display: flex column
      gap: 1rem
      ├─ .intro block
      ├─ [part=notes] block, optional
      ├─ [part=steps] flex row wrap, optional
      ├─ [part=question] flex column
      │  ├─ .question-text block
      │  └─ [part=options] flex column
      │     └─ .option flex row
      └─ [part=actions] flex row, justify-between
```

| Box                | Display         | Margin | Border | Padding            | Gap        | Sizing notes                       |
| ------------------ | --------------- | ------ | ------ | ------------------ | ---------- | ---------------------------------- |
| `:host`            | `block`         | `0`    | none   | none               | n/a        | Width follows parent.              |
| `[part=container]` | `flex column`   | `0`    | none   | `0`                | `1rem`     | Header plus card/raw stack.        |
| `[part=header]`    | `flex row`      | `0`    | none   | `0`                | `1rem`     | Header/status/raw toggle row.      |
| `[part=raw]`       | `block`         | `0`    | `1px`  | `0.75rem`          | n/a        | Replaces picker while raw is open. |
| `[part=card]`      | `flex column`   | `0`    | `1px`  | `1rem`             | `1rem`     | Question picker card surface.      |
| `[part=steps]`     | `flex row wrap` | `0`    | none   | `0`                | `0.25rem`  | Only shown for multiple questions. |
| `[part=question]`  | `flex column`   | `0`    | none   | `0`                | `0.75rem`  | Active question only.              |
| `[part=options]`   | `flex column`   | `0`    | none   | `0`                | `0.375rem` | Contains labels/inputs.            |
| `.option`          | `flex row`      | `0`    | `1px`  | `0.625rem 0.75rem` | `0.75rem`  | Click target for an option.        |
| `[part=actions]`   | `flex row`      | `0`    | none   | `0.5rem 0 0`       | `0.5rem`   | Footer controls.                   |

## States

| State           | Trigger                                                     | Visual/layout effect               |
| --------------- | ----------------------------------------------------------- | ---------------------------------- |
| pending         | `state="approval-requested"`                                | Shows interactive question picker. |
| answered/output | one or more `<disco-answer>` children and non-pending state | Shows completed answers.           |
| empty           | no question or answer children                              | Shows empty fallback text.         |
| raw             | raw toggle button is pressed                                | Replaces picker with raw JSON.     |

## Accessibility

- Native radio/checkbox inputs provide keyboard interaction for options.
- The raw toggle is a native button with `aria-pressed`.
- Step buttons are regular buttons.
- Submit button is disabled until every question has an answer.
- Further work: add explicit group/fieldset semantics for each question.

## Notes and constraints

- Visible/interactable question data is expressed as semantic child elements, not JSON metadata.
- The component owns only local UI state. Hosts own persistence, API calls, and stream resumption.
- On submit, hosts should listen for `disco-tool-question-submit` and bridge it to app-specific behavior.
