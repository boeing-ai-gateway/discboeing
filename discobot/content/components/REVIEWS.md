# Review Guidelines

- Do not create a secondary helper file unless it contains helpers shared across multiple files.
- Keep related helpers in the `.templ` file when they only support that component.
- Avoid one-line or two-line helpers for simple templ expressions.
- Do not use `__` in `data-class:` or `data-style:` attribute names.
- Use one helper for a full `class` string and one for a full `style` string when markup would get noisy.
- Review component styling against `discobot/docs/GUIDELINES.md`; prefer inline Tailwind for one-off styles and reserve custom classes for reusable, stateful, or complex CSS.
- Prefer `data-on:*` handlers on the element that owns an interaction, calling small functions exposed under `window.discobot.<island>`. Avoid document-level delegated listeners unless the behavior is genuinely global, such as outside-click or Escape handling.
