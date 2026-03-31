# Documentation Language Layout

This directory follows an English-first strategy.

- English docs live in `docs/*.md`.
- Chinese docs live in `docs/zh/*.md`.
- For the same topic, use the same filename in both locations.

## Language Rule

- `docs/*.md`: English primary pages (full content or migration placeholders).
- `docs/zh/*.md`: Chinese complete pages.
- Avoid creating `*_zh.md` files directly under `docs/`.

## Current Chinese Set

- `docs/zh/USAGE.md`
- `docs/zh/ARGS_YAML_TYPES.md`
- `docs/zh/TEMPLATE_REPO_PROTOCOL.md`
- `docs/zh/V1.0_STATUS_ANALYSIS.md`
- `docs/zh/Roadmap.md`
- `docs/zh/IMPLEMENTATION_PLAN.md`
- `docs/zh/FEATURE_STATUS.md`

## Migration Note

Some English pages in `docs/` are currently placeholders linking to `docs/zh/`.
Replace placeholders with full English content gradually, but keep filenames aligned.
