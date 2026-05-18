---
name: datastar
description: Use when building, changing, or reviewing Datastar UIs, signals, data-* attributes, backend actions, SSE responses, or Rocket components.
---

# Datastar

Use this skill for current Datastar work. If exact behavior matters, prefer the
live docs at `https://data-star.dev/docs.md`; do not rely on old repo branches
or historical docs as current API truth.

## What Datastar is

Datastar is a small hypermedia-first frontend framework. The backend drives the
UI by patching HTML and signals, while frontend reactivity is declared with
standard `data-*` attributes. It combines htmx-like backend reactivity with
Alpine-like frontend reactivity and does not require npm.

## The Datastar way

- Keep authoritative state on the backend; treat frontend state as exposed and
  user-controlled.
- Start with defaults. Datastar's default request, morphing, and SSE behavior is
  the intended path.
- Prefer backend patches: send elements and signals from the server instead of
  rebuilding a client-side state machine.
- Use signals sparingly: UI toggles, input values, and request payload state are
  good uses; broad application state usually belongs on the backend.
- Trust morphing. Send larger server-rendered chunks with stable IDs instead of
  hand-targeting tiny fragments. Use `data-ignore-morph` only for true escape
  hatches.
- Prefer `text/event-stream` responses. SSE can send 0..n element and signal
  patches in one response; scripts can be executed by patching script tags, and
  SDKs may provide helpers.
- Keep HTML DRY with backend templates/components.
- Use normal page navigation: anchors for navigation, redirects from backend
  when needed. Do not manage browser history unless there is a strong reason.
- For realtime apps, consider CQRS: one long-lived read stream plus short-lived
  command requests.
- Use loading indicators instead of optimistic UI unless deception/failure modes
  are explicitly acceptable.
- Accessibility is your responsibility: semantic HTML, ARIA where appropriate,
  keyboard and screen-reader behavior.

## Core syntax rules

- Attributes are evaluated depth-first and in DOM attribute order. Put
  `data-indicator` before `data-init` if the init request should use it.
- Data attributes are reapplied after DOM patches only when added, removed, or
  changed; morphing preserves unchanged attributes.
- Signal keys in `data-bind:*`, `data-signals:*`, `data-computed:*`, etc. are
  camel-cased by default: `data-signals:foo-bar` creates `$fooBar`.
- Non-signal keys default to kebab-case: `data-class:text-blue-700`,
  `data-on:my-event`.
- Use `__case.camel|kebab|snake|pascal` to override key casing.
- Signal paths may be nested with dots or object syntax:
  `data-signals:foo.bar="1"` or `data-signals="{foo: {bar: 1}}"`.
- `null` or `undefined` removes a signal.
- Signals beginning with `_` are excluded from backend requests by default.
- Signal names cannot begin with or contain `__` because that delimiter is used
  for modifiers.
- Datastar expressions are JavaScript-like. Use `$signal` to read/write signals;
  `el` is always the current element. In `data-on`, `evt` is the event object.
- Separate multiple statements with semicolons. Datastar does not await async
  expressions; dispatch events or update signals when async work completes.
- Keep expression logic small. Move complex JS to modules or web components;
  for web components prefer props down, events up.

## Attribute cheat sheet

| Attribute | Use |
| --- | --- |
| `data-signals[:path]` | Patch/create/remove signals; object syntax supported; `__ifmissing` sets defaults only. |
| `data-bind[:signal]` | Two-way bind inputs, selects, textareas, checkboxes/radios, file inputs, and web components. Predefined signal types are preserved. |
| `data-computed[:signal]` | Read-only computed signal; do not perform side effects here. |
| `data-text` | Set text content from an expression. |
| `data-show` | Toggle visibility; add initial `style="display: none"` to prevent flicker. |
| `data-class[:class]` | Toggle one or many classes. |
| `data-attr[:attr]` | Bind one or many attributes, including ARIA attributes. |
| `data-style[:prop]` | Bind one or many inline styles; falsy values restore/remove dynamic styles. |
| `data-on:event` | Attach event listener and run an expression. `data-on:submit` prevents native form submission. |
| `data-effect` | Run an expression on load and whenever referenced signals change. |
| `data-init` | Run on initialization, insertion, or attribute change. |
| `data-indicator[:signal]` | Set a signal true while a fetch from that element is in flight, otherwise false. |
| `data-ignore` | Ignore this element and descendants; `__self` ignores only the element. |
| `data-ignore-morph` | Prevent patch-element morphing for an element and children. |
| `data-preserve-attr` | Preserve listed attributes during morphing, e.g. `open class`. |
| `data-ref[:signal]` | Store an element reference in a signal. |
| `data-json-signals` | Debug current signals; accepts `{include, exclude}` regex filters; `__terse`. |
| `data-on-intersect` | Run when element intersects viewport. |
| `data-on-interval` | Run periodically; use `__duration.500ms` / `.1s` and optional `.leading`. |
| `data-on-signal-patch` | Run when signals are patched; `patch` contains details. |
| `data-on-signal-patch-filter` | Filter signal-patch listeners with `{include, exclude}` regexes. |

Common modifiers:

- `data-bind`: `__prop.name`, `__event.input.change`, `__case.*`.
- `data-on`: `__once`, `__passive`, `__capture`, `__delay.*`,
  `__debounce.*`, `__throttle.*`, `__viewtransition`, `__window`,
  `__document`, `__outside`, `__prevent`, `__stop`, `__case.*`.
- `data-init`: `__delay.*`, `__viewtransition`.
- `data-on-intersect`: `__once`, `__exit`, `__half`, `__full`,
  `__threshold.*`, plus delay/debounce/throttle/view-transition modifiers.
- `data-signals`, `data-computed`, `data-indicator`, `data-ref`: `__case.*`.

## Actions and backend requests

Actions are expression helpers using `@name(...)`.

- Local signal helpers: `@peek(fn)`, `@setAll(value, filter?)`,
  `@toggleAll(filter?)`.
- Backend actions: `@get(uri, options?)`, `@post`, `@put`, `@patch`,
  `@delete`.
- Requests include `Datastar-Request: true`.
- By default, every backend request sends all non-underscore signals.
  - `GET`: JSON-encoded signals in the `datastar` query param.
  - Other methods: signals in a JSON body.
- Avoid partial signal payloads unless necessary; if needed, use
  `filterSignals: {include: /.../, exclude: /.../}`.
- `contentType: 'json'` is default. `contentType: 'form'` sends the closest or
  selected form, validates fields, and sends no signals. For file upload forms,
  use `enctype="multipart/form-data"`.
- Useful request options: `contentType`, `filterSignals`, `selector`, `headers`,
  `openWhenHidden`, `payload`, `retry`, `retryInterval`, `retryScaler`,
  `retryMaxWait`, `retryMaxCount`, `requestCancellation`.
- `requestCancellation` defaults to `auto` and cancels prior requests from the
  same element. Other values: `cleanup`, `disabled`, or an `AbortController`.
- `datastar-fetch` events fire with `evt.detail.type`: `started`, `finished`,
  `error`, `retrying`, `retries-failed`.

Response handling by content type:

- `text/event-stream`: Datastar SSE events.
- `text/html`: patch elements. Optional headers: `datastar-selector`,
  `datastar-mode`, `datastar-use-view-transition`.
- `application/json`: patch signals. Optional header:
  `datastar-only-if-missing: true`.
- `text/javascript`: execute script. Optional header:
  `datastar-script-attributes` JSON.

## SSE events

Prefer SDK helpers when available. Manual SSE events must end with a blank line.

```text
event: datastar-patch-elements
data: selector #target
data: mode outer
data: elements <div id="target">Updated</div>

```

`datastar-patch-elements` patches HTML. Top-level elements should have stable
IDs for morphing. Options include `selector`, `mode`, `namespace`,
`useViewTransition`, and `elements`. Modes: `outer` (default/recommended),
`inner`, `replace`, `prepend`, `append`, `before`, `after`, `remove`.

```text
event: datastar-patch-signals
data: signals {answer: 'bread'}

```

`datastar-patch-signals` patches signals. `data: onlyIfMissing true` only writes
missing signals. Set a signal to `null` to remove it.

## Minimal pattern

```html
<div id="quiz" data-signals="{response: '', answer: ''}">
  <button data-indicator:_loading data-on:click="@get('/quiz')">
    Fetch question
  </button>
  <span data-show="$_loading" style="display: none">Loading...</span>
  <p data-text="$response"></p>
</div>
```

Backend responds with HTML, JSON, JS, or preferably SSE that patches elements
and/or signals. Keep the backend authoritative and patch the next valid UI.

## Pro features and Rocket

Clearly mark Pro-only APIs when suggesting them.

Pro attributes: `data-animate`, `data-custom-validity`, `data-match-media`,
`data-on-raf`, `data-on-resize`, `data-persist`, `data-query-string`,
`data-replace-url`, `data-scroll-into-view`, `data-view-transition`.

Pro actions: `@clipboard()`, `@fit()`, `@intl()`.

Rocket is Datastar Pro's beta web-component API:

- Define components with `rocket(tagName, { ... })`; `tagName` must contain a
  hyphen.
- Import explicitly from the Pro bundle, e.g.
  `import { rocket } from '/bundles/datastar-pro.js'`.
- `mode`: `light`, `open`, or `closed`; omitted defaults to open shadow DOM.
- `props` defines the public API with codecs and maps JS prop names to kebab
  attributes. Prefer props for parent/page configuration.
- Codecs include `string`, `number`, `bool`, `date`, `json`, `js`, `bin`,
  `array`, `object`, `oneOf`; codecs are fluent immutable builders with
  defaults/normalization.
- Use local Rocket signals (`$$`) for internal state; `$` is the global Datastar
  signal root.
- `setup` runs once per connected instance for local state, timers, effects,
  actions, prop observers, cleanup, and imperative integration that does not
  require rendered refs.
- `onFirstRender` runs once after initial render/apply/ref population; use it
  for DOM measurements, focus, third-party widgets, or `data-ref` work.
- `render` returns `html`/`svg` fragments, primitives, iterables, or nothing.
- `renderOnPropChange` controls rerendering; use `observeProps` for prop change
  side effects and Datastar signals/effects for reactive local state.
- Rocket supports `<template data-if>`, `data-else-if`, `data-else`, and
  `<template data-for>`. Local `$$name` is scoped under `$._rocket`.

## Security checklist

- Escape user input before sending HTML patches.
- Never trust frontend signals for authorization, ownership, pricing, roles, or
  other sensitive decisions.
- Do not expose secrets or sensitive data in signals; remember non-underscore
  signals are sent on requests by default.
- Ignore unsafe/untrusted DOM with `data-ignore` when escaping is not possible.
- Use CSP and avoid `text/javascript` responses unless they are deliberate and
  reviewed.
- Validate backend requests normally; Datastar improves interaction, not trust.
