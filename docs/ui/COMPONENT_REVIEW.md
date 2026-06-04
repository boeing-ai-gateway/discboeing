# Svelte Component Review Guidelines

Use these guidelines when reviewing Svelte components. The goal is to keep
components simple, mostly declarative, easy to read, and idiomatic Svelte 5.

## Review goals

Every component review should answer:

1. Is this component necessary?
2. Is the markup easy to read?
3. Is the styling mostly declarative and token-based?
4. Is JavaScript limited to props, state, derived values, and event handling?
5. Does it use Svelte 5 idioms cleanly?
6. Is the component placed in the right folder for this repository's
   architecture?

## Component responsibility

A component should have one clear purpose.

Good signs:

- The component can be summarized in one sentence.
- Props are few and meaningful.
- It does not mix layout, data fetching, business rules, and rendering.
- It does not know more about the application than it needs to.

Watch for:

- Components that render unrelated UI regions.
- Large blocks of conditional logic.
- Many boolean props controlling internal branches.
- App context used in a component that should be pure.
- Components in `ui/` or `ai/` consuming global app/session/thread context.

Repository-specific placement rules:

- `ui/src/lib/components/ui/` contains pure primitives. Never add context
  consumers here.
- `ui/src/lib/components/ai/` contains self-contained compound components.
  These may use component-local context, but not global app/session/thread
  contexts.
- `ui/src/lib/components/app/` contains app shell components that may consume
  global contexts.
- `ui/src/lib/components/app/parts/` contains pure props-only implementation
  components.

Review questions:

- Could this component be simpler if part of it moved into a pure child
  component?
- Could this component be simpler if a tiny extracted child component were
  inlined?
- Is this component split because the split improves clarity, or only because
  splitting felt tidy?

Avoid splitting solely for the sake of splitting. Prefer fewer components unless
extraction improves clarity.

## Markup first

Svelte components should read mostly like HTML. The template should make the
rendered DOM easy to understand.

Prefer:

```svelte
<button
	type="button"
	class="rounded-md border border-border px-3 py-2 text-sm"
	disabled={isDisabled}
	onclick={handleClick}
>
	{label}
</button>
```

Watch for:

- Complex `{#if}` trees that hide the main UI.
- Deep `{#each}` nesting.
- Repeated wrapper `<div>` elements.
- Computed tag names or dynamic component indirection.
- Large script blocks before simple markup.

Prefer semantic elements before generic `div` elements:

- `button` for actions.
- `a` for navigation.
- `form`, `label`, `input`, and `fieldset` for form controls.
- Headings that preserve document structure.
- Native `disabled`, `required`, `selected`, and `checked` states where
  possible.

Avoid:

- Clickable `div` elements.
- ARIA used to compensate for the wrong HTML element.
- Over-engineered component wrappers around basic elements.

Review questions:

- Can I understand the rendered DOM by reading the template?
- Are there unnecessary wrapper elements?
- Are native semantics doing as much work as possible?

## CSS and styling

Prefer clean, local, declarative styling. In this repository, use Tailwind CSS
v4 utility classes and design tokens.

Prefer tokens such as:

- `bg-background`
- `text-foreground`
- `border-border`
- `bg-tree-hover`
- `bg-diff-add`

Good signs:

- Classes describe layout and visual state clearly.
- Design tokens are used instead of hard-coded colors.
- CSS custom properties are used where they improve theming.
- State styling is handled with CSS where possible.

Watch for:

- Hard-coded colors.
- Inline `style="..."` except for true dynamic values.
- Large style blocks duplicating Tailwind utilities.
- JavaScript calculating visual state that CSS could handle.
- Complex class string builders.

Prefer:

```svelte
<div class="rounded-lg border border-border bg-background p-4">
```

Avoid:

```svelte
<div style={`background: ${theme.bg}; border-color: ${theme.border};`}>
```

Review questions:

- Is JavaScript being used for something CSS can handle?
- Are repo design tokens used instead of new hard-coded visual values?
- Is the class list long because the component is doing too much?

## Minimal JavaScript

The `<script>` block should usually be boring. It should contain props, local
state, derived values, event handlers, and small formatting helpers when needed.

Watch for:

- Large imperative functions.
- DOM querying.
- Manual lifecycle code.
- Business logic embedded in UI components.
- State duplicated between props, local variables, and derived values.
- Data transformations that belong in a store, context, service, or helper.

Prefer Svelte's declarative features over manual code:

```svelte
<script lang="ts">
	let { items }: { items: Item[] } = $props();

	let visibleItems = $derived(items.filter((item) => !item.hidden));
</script>
```

Review questions:

- What is the minimum state this component actually needs?
- Is this state derived from props or context?
- Could this function be a simple derived expression?
- Is this component doing business logic instead of presentation logic?

## Svelte 5 idioms

Prefer Svelte 5 runes and current event syntax.

Use:

```svelte
<script lang="ts">
	type Props = {
		label: string;
		disabled?: boolean;
	};

	let { label, disabled = false }: Props = $props();

	let count = $state(0);
	let doubled = $derived(count * 2);

	function increment() {
		count += 1;
	}
</script>

<button disabled={disabled} onclick={increment}>
	{label}: {doubled}
</button>
```

Review for:

- `$props()` instead of old `export let`.
- `$state()` for local mutable state.
- `$derived()` for derived values.
- `$effect()` only when syncing with external systems.
- `onclick`, `oninput`, and similar event attributes instead of legacy
  `on:click` syntax.
- Snippets and render props where they clarify composition, but not when simple
  props or direct markup are clearer.

Be strict with `$effect`.

Good `$effect` use cases:

- Syncing to browser APIs.
- Subscribing or unsubscribing to external systems.
- Imperative integration with non-Svelte libraries.
- Reacting to state changes that require side effects.

Bad `$effect` use cases:

- Computing derived values.
- Mirroring props into local state.
- Running ordinary rendering logic.
- Fixing state shape after the fact.

Review question:

- If this `$effect` disappeared, could `$derived`, props, or normal event
  handling replace it?

## Props and API design

A simple component usually has a simple prop API.

Prefer:

```ts
type Props = {
  label: string;
  disabled?: boolean;
  onclick?: () => void;
};
```

Watch for:

- Many optional props.
- Many boolean appearance flags.
- Props that are passed through multiple layers unchanged.
- Internal state initialized from props and then manually kept in sync.
- Generic props that make the component hard to reason about.

Review questions:

- Is this prop part of the component's core responsibility?
- Is this prop compensating for a component that is too specific or too broad?
- Would composition be clearer than adding another option?

Prefer composition for content variation when it makes call sites clearer, but
avoid compound APIs when simple props are clearer.

## Conditional rendering

Keep conditionals obvious.

Prefer simple branches:

```svelte
{#if loading}
	<Spinner />
{:else if error}
	<ErrorMessage {error} />
{:else}
	<MessageList messages={messages} />
{/if}
```

Watch for:

- Nested conditions that obscure the main UI.
- Multiple conditions checking the same state in different places.
- Boolean names that are unclear.
- UI modes that should be explicit enum-like values.

Prefer a single explicit mode when states are mutually exclusive:

```ts
type ViewMode = "loading" | "empty" | "error" | "ready";
```

Review question:

- Can this component accidentally render two states that should be impossible
  together?

## Events and data flow

Keep data flow obvious:

- Parent components own app-level state.
- Child components receive props and call callbacks to express intent.
- Pure components receive props and do not reach into global context.
- Context consumers should be limited to app-level components where context is
  justified.

Watch for:

- Children mutating parent-owned objects.
- Deep components reaching into global context unnecessarily.
- Event handlers with broad side effects.
- Components that both render and orchestrate complex workflows.

Review question:

- Is this component displaying state, changing state, or coordinating app
  behavior? Is that the right role for it?

## Root context and CQRS

App-level state lives in one deeply reactive Svelte `$state()` root context. The
context is split by responsibility:

- `context.data` is backend, runtime, and domain data.
- `context.view` is frontend-only UI state, including dialogs, selection,
  navigation, preferences, and other view concerns.

Keep the split strict. Dialog state, expanded panels, selected UI controls,
temporary drafts, and similar browser-only state belong in `view`, not `data`.
Do not add getters, setters, or property wrapper objects to the state tree; keep
the context shape plain and directly reactive.

Components should follow the CQRS direction:

- Read current state from the root context.
- Express behavior by calling command functions.
- Let commands call backend APIs and update root context state.

Prefer:

```svelte
<script lang="ts">
	import { openSettingsDialog } from "$lib/context/commands/app-view";
	import { useContext } from "$lib/context/context.svelte";

	const context = useContext();
</script>

{#if context.view.app.updates.showBadge}
	<span class="size-2 rounded-full bg-primary"></span>
{/if}

<button type="button" onclick={() => openSettingsDialog()}>
	Settings
</button>
```

Avoid:

```svelte
<script lang="ts">
	const context = useContext();
	const updates = $derived(context.view.app.updates);

	function openSettings() {
		context.view.app.dialogs.settings.open = true;
	}
</script>
```

The context is already reactive. Do not create temporary `$derived` variables
just to rename stable context objects or single fields. A plain alias is fine
when it reduces repeated long paths and the referenced object is stable:

```svelte
<script lang="ts">
	const context = useContext();
	const keyboardShortcutsDialog = context.view.app.dialogs.keyboardShortcuts;
</script>
```

Use `$derived` when computing a new value, filtering/mapping data, combining
multiple inputs, or intentionally tracking a potentially replaced value.

Review questions:

- Does new state belong in `view` or `data`?
- Is the component calling a command for behavior instead of mutating app state
  directly?
- Is a `$derived` value computing something meaningful, or only renaming a
  reactive context path?
- Could a plain context read or stable alias make this component simpler?

## Accessibility

Accessibility is part of keeping components simple and correct.

Check that:

- Interactive elements use native controls.
- Buttons have `type="button"` unless submitting a form.
- Inputs have labels.
- Decorative icons are hidden from screen readers.
- Icon-only buttons have accessible labels.
- Focus states are visible.
- Keyboard behavior works naturally.
- Dialogs, menus, and popovers use existing primitives where possible.

Avoid custom keyboard behavior unless necessary.

Review question:

- Would this still work well with keyboard navigation and a screen reader?

## Testing expectations

Not every component needs a heavy test, but complex behavior should be covered.

For this repository:

- Use Vitest for Svelte component tests and runtime tests that import
  rune-backed `.svelte.ts` modules.
- Use Node's built-in `node:test` for plain TypeScript helper tests and
  source-level assertion tests that do not rely on Svelte or Vite transforms.

Good test candidates:

- Components with non-trivial conditional rendering.
- Components with user interaction.
- Components with derived state.
- Components that encode permissions, modes, or workflow state.

Avoid tests that only assert implementation details or class strings unless the
styling behavior is critical.

Review question:

- Is there logic here that could regress silently? If yes, test it or simplify
  it until a test is unnecessary.

## Review checklist

Use this checklist during Svelte component review:

- [ ] Component has one clear responsibility.
- [ ] Component is in the correct folder: `ui/`, `ai/`, `app/`, or
      `app/parts/`.
- [ ] Markup is semantic and easy to read.
- [ ] There are no unnecessary wrapper elements.
- [ ] Styling uses Tailwind/design tokens instead of hard-coded visual values.
- [ ] JavaScript is minimal and mostly props, state, derived values, and
      handlers.
- [ ] The component uses Svelte 5 idioms: `$props`, `$state`, `$derived`, and
      modern event attributes.
- [ ] `$effect` is only used for real side effects.
- [ ] Props are minimal and understandable.
- [ ] Conditional rendering is simple and impossible states are avoided.
- [ ] Data flow is clear.
- [ ] App components follow the root-context/CQRS pattern.
- [ ] UI-only state is in `context.view`, not `context.data`.
- [ ] `$derived` aliases are avoided unless they compute or intentionally track
      a value.
- [ ] Accessibility basics are handled.
- [ ] Tests are added where behavior is non-trivial.

## High-signal review findings

Prioritize findings like:

- The component is mostly JavaScript when it could be declarative markup.
- State or `$effect` is used for something `$derived` or CSS can handle.
- `$derived` is used only to rename already-reactive context state.
- UI-only state is stored in `context.data` instead of `context.view`.
- A component mutates root context state directly instead of calling a command.
- A pure component consumes global app/session/thread context.
- A component is in the wrong component folder for its context usage.
- A condition tree hides the main UI or permits impossible states.
- The prop API has too many modes or boolean flags.
- A wrapper component does not simplify the call site.
- Custom interaction is introduced where native HTML would work.
- Styling bypasses design tokens or duplicates existing primitives.

Prefer review comments that suggest a simpler alternative instead of only saying
that something is too complex.
