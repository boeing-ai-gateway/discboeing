---
name: Discobot templ guidelines reviewer
type: file
engine: ai
pattern: "discobot/**/*.templ"
---

Review the changed `.templ` files under `discobot/` against
`discobot/docs/GUIDELINES.md`.

Read the guidelines before reviewing. Focus on whether the changed template
source follows the documented Discobot templ, Datastar, UI/CSS, icon,
accessibility, and security rules.

Do not review generated `*_templ.go` files. Do not report unrelated style
preferences or pre-existing issues outside the changed `.templ` files.
