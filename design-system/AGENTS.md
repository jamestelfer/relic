# Agent Instructions — design-system

> These instructions target AI coding agents. They are not intended for human
> readers.

## Role of this directory

`design-system/` is **design reference material** — it is not production code
and is not part of the build or test pipeline. Nothing here is embedded into the
rendered HTML output.

| Path | Purpose |
|---|---|
| `colors_and_type.css` | Canonical design tokens (color, type, scale). Source of truth copied into `internal/renderer/assets/`. |
| `preview/` | Static HTML previews for visual QA. Opened directly in a browser during design iteration. |
| `uploads/` | Spec documents and reference material uploaded during design conversations. |
| `SKILL.md` | Skill definition for design-focused agent sessions. |
| `README.md` | Human-readable overview of the design system. |

## What this means for review and validation

- **External resources are expected.** Preview files use relative `<link>` and
  `<img src>` references to local files. This is intentional — they are
  development aids, not self-contained artifacts.
- **Google Fonts `@import` is intentional** in `colors_and_type.css`. The font
  definitions here are the source of truth; the same `@import` appears in the
  production CSS at `internal/renderer/assets/colors_and_type.css`. Google Fonts
  are an allowed external dependency (see root `AGENTS.md`).
- **No tests cover this directory.** Changes here do not require `just verify`.
  They may indicate that corresponding changes are needed in
  `internal/renderer/assets/` or `internal/renderer/renderer.templ`.
- **Do not inline, base64-encode, or bundle** resources referenced by preview
  files. The previews are meant to be lightweight and editable.
