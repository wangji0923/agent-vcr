# v0.2.5 Behavior Visualization Review Pack

This folder contains generated v0.2.5 visualization artifacts after the GPT Pro review fixes.

## Open First

- `two-run-compare.html`: primary two-run swimlane comparison.
- `multi-run-compare.html`: primary three-run lane comparison.
- `single-run.html`: single-run behavior timeline.

PNG screenshots are included for quick visual review.

## What Changed After Review

- Legacy source files now count as source edits while still marking legacy-path usage.
- Missing validation after source edit now surfaces as `Skipped validation = yes`.
- File Access Compare now shows only real file read/edit access.
- Search scopes such as `src` and `tests` moved to Search Scope Compare.
- Multi-run HTML includes first divergence per compared run.
- Path Graph is labeled as an auxiliary view and points users back to Swimlane Timeline.

## Review Focus

- Swimlane Timeline is the main v0.2.5 visualization surface.
- First divergence should be highlighted in the summary and the swimlane row.
- File Access Compare should show per-run read/edit differences only for files.
- Search Scope Compare should show search scope differences such as `src` vs `src, tests`.
- Metrics Cards should not mislead unavailable or skipped-validation values.
- Path Graph is auxiliary, not the primary swimlane view.
