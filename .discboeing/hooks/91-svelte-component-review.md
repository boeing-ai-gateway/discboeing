---
name: Svelte component review
type: file
engine: ai
pattern: "**/*.svelte"
description: Review changed Svelte components for simplicity and idiomatic Svelte 5
phase: review
---
Review the changed Svelte files for simplicity, clean HTML/CSS, minimal
JavaScript, accessibility, and idiomatic Svelte 5.

Before reviewing, read and apply `docs/ui/COMPONENT_REVIEW.md`. Also apply the
component placement and context rules in `docs/ui/ARCHITECTURE.md` and any more
specific repository guidance that applies to the changed files.

Focus on high-signal, actionable feedback. Prefer suggestions that simplify the
component, reduce JavaScript, improve semantic markup, use existing design
tokens, or align the file with Svelte 5 and this repository's component folder
rules. Do not flag purely subjective preferences unless they are supported by
the documented guidelines or project patterns.
