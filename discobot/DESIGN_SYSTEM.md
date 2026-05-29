# Discobot design system

`Discobot` follows a compact, modern VS Code-style workspace layout. It should
feel like an IDE shell rather than a dashboard: panes, borders, file trees,
toolbar controls, and a focused composer area do most of the visual work.

## Source of truth

- Use `static/themes.css` for semantic tokens and theme definitions,
  `static/reset.css` for the browser reset, and `static/app.css` for base
  rules, utilities, and DaisyUI-style component classes.
- Keep token names aligned with `../ui/src/app.css` when the concept is shared:
  `--background`, `--foreground`, `--card`, `--popover`, `--primary`,
  `--secondary`, `--muted`, `--accent`, `--border`, `--input`, `--ring`,
  `--sidebar`, `--tree-hover`, `--tree-selected`, `--turn-border`,
  `--terminal-*`, and `--diff-*`.
- `Discobot` is allowed to tune values for its dark, pixel-matched shell. Do not
  copy Svelte UI values blindly when they make this layout less IDE-like.
- `static/reset.css`, `static/themes.css`, and `static/app.css` are served
  directly and are the editable CSS source of truth.

## Layout model

The workspace uses a three-panel IDE structure:

```text
┌─────────────────────────────────────────────────────────────┐
│ Window chrome                                                │
├───────────────┬───────────────────────────┬─────────────────┤
│ Sessions      │ Composer/session canvas   │ Files/changes   │
│ sidebar       │                           │ panel           │
└───────────────┴───────────────────────────┴─────────────────┘
```

- Left sidebar: session and workspace navigation.
- Center panel: primary session canvas and composer.
- Right panel: contextual files, changes, and project state.
- Window chrome: lightweight desktop/IDE controls.

Default dimensions live in CSS custom properties:

| Token | Purpose |
| --- | --- |
| `--workspace-sidebar-width` | Left session sidebar width |
| `--workspace-files-width` | Right files panel width |
| `--workspace-gap` | Gap between center and right panels |
| `--workspace-panel-radius` | Major panel radius |
| `--workspace-shell-radius` | Outer shell radius |
| `--workspace-tree-row-height` | Dense file-tree row height |
| `--workspace-toolbar-height` | Files/changes panel toolbar height |

## Color and surfaces

Use a dark, low-contrast palette. Visual surface tokens are named by their
function, not by the component that happens to use them:

- `--surface-canvas`: the single continuous background behind the UI chrome,
  session navigation, and center canvas.
- `--surface-card`: stacked content surfaces such as the right files panel and
  the focused composer card.
- `--surface-canvas-muted`: a quieter canvas shade reserved for nested regions
  when the single-surface layout needs subtle depth.
- `--surface-control`: inputs and command/composer controls.
- `--surface-control-active`: selected tabs, active controls, hover fills.
- `--border` / `--border-subtle`: 1px separators.
- `--foreground`: primary readable text.
- `--muted-foreground`: secondary labels and metadata.
- `--text-muted`: placeholders and disabled controls.
- `--text-subtle`: quiet but still readable heading/support text.
- `--primary` / `--focus-accent`: restrained blue focus/accent.

Prefer borders and small background changes over shadows. Shadows are reserved
for the outer desktop shell or rare overlays.

## Themes

`Discobot` ports the semantic theme tokens from `../ui/src/app.css`. The active
theme is controlled on the root element with `data-theme` and the `dark` class,
matching the Svelte UI convention where dark-only themes use selectors like
`[data-theme="nord"].dark`.

The titlebar includes a temporary theme switcher for testing every ported
theme. It stores the selected value in `localStorage` under `discobot-theme` and
applies it before the stylesheet loads to avoid most theme flash. The app shell
components should reference semantic tokens such as `--foreground`, `--border`,
`--muted-foreground`, `--primary`, and functional shell tokens such as
`--surface-canvas`, `--surface-card`, `--surface-control`, and `--focus-accent`
rather than fixed dark colors.

## Spacing and density

The density should match developer tools:

- Major panel padding: `10px` to `12px`.
- Panel gap: `10px`.
- Toolbar height: about `32px` to `37px`.
- Tree rows: about `22px`.
- Icon buttons: `24px`.
- Inline icon size: `13px` to `16px`.
- Composer max width: about `752px`.

Use component classes for repeated shell structure and controls, and reserve
local utility classes for one-off layout adjustments. Preserve explicit pixel
values in CSS tokens/components when matching the IDE shell geometry.

## Typography

- Use the `--font-sans` stack from `static/themes.css`.
- Default application text is `13px`.
- Toolbars, metadata, and secondary controls are `11px` to `12px`.
- Composer heading text is larger, around `18px`, but still regular weight.
- Use monospace only for shortcuts, code, terminal output, and file/code
  details.

## Component rules

- CSS is organized as standard cascade layers:
  `reset`, `tokens`, `base`, `utilities`, then `components`.
- The `reset` layer is adapted from Tailwind CSS v4.3.0 Preflight. Keep the
  attribution comment in `static/reset.css`; the only intentional adaptation is
  mapping Tailwind's `--theme(...)` font declarations to Discobot's
  `--font-sans` and `--font-mono` tokens so the file remains plain CSS.
- Utilities are Tailwind-style, low-level classes such as `flex`, `items-center`,
  `p-6`, and `text-card-foreground`. Add them only when they are actually
  needed by templates.
- Reused components use DaisyUI-style semantic classes such as `ide-panel`,
  `panel-toolbar`, `btn`, `tab`, and `tree-row`. Prefer these for recurring UI
  patterns.
- Single-use component structure may keep its CSS inline in the owning templ
  file. Prefix those classes with the component name, such as `app-shell` and
  `app-shell--window`, so their ownership is obvious even though CSS selectors
  remain global.
- Panels use `1px` borders, subtle backgrounds, and `6px` radii.
- Buttons are compact and tool-like; avoid large marketing-style CTAs.
- Icon-only controls are quiet by default and brighten on hover/active states.
- File trees are dense, left-aligned, and use small chevrons/icons.
- Focus states should use `--ring` or `--focus-accent` without large browser
  default outlines.
- Prefer semantic component classes and tokens over raw color/spacing utilities
  in templates. Raw pixel colors are acceptable inside `static/themes.css` or
  `static/app.css` only when preserving an exact screenshot match, and should be
  given descriptive class names when the surrounding component is touched.

## Icons

Use typed templ icon helpers from `content/components/app/icon.templ`, with
design-system icon classes such as `@IconSearch("icon-sm")` or
`@IconChevronRight("icon-xs icon-muted")`. Do not add new stringly-typed icon
calls in app components. If a new app icon is needed, map it to a Lucide
component in `content/components/app/icon.templ`, create a typed helper, and
use that helper from templates.

Lucide icons are rendered as inline SVGs through the
`github.com/bryanvaz/go-templ-lucide-icons` templ package. `pnpm assets:build`
only rebuilds CSS; icon markup is generated by `pnpm generate`.

## Relationship to `./ui`

The Svelte UI defines the broader Discobot theme vocabulary in `../ui/src/app.css`
and includes light/dark theme variants. `Discobot` currently implements a
dark-first IDE shell with the same token names where possible, plus
`--workspace-*` tokens for layout details that are specific to this experimental
templ port. Unlike the Svelte UI, `Discobot` intentionally uses standard CSS
cascade layers instead of Tailwind.
