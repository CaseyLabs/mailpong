# Remaining dependency issue

## Scope
Resolve D2 when stable frontend tools support TypeScript 7. Other ISSUES.md findings have been fixed, reviewed, squash merged, and pushed.

## Milestones
- D2: upgrade TypeScript when both published peer ranges accept 7. Acceptance: supported dependency graph; frontend lint, check, and build pass.

## Progress
- [ ] D2 — blocked upstream as of 2026-10-12. `npm view` reports typescript-eslint 8.71.1 requires `<6.1.0`; svelte-check 4.7.6 accepts only 5/6. No supported upgrade is available.

## Surprises & Discoveries
- The latest lint/check tools still exclude the published TypeScript 7.0.2 release.

## Decision Log
- Keep TypeScript ~6.0.3 until both tools support 7.

## Outcomes & Retrospective
All actionable audit findings and recurring tooling guidance are resolved. Each implementation branch passed CLI review before landing. Full Go tests, lint, frontend check/build, and affected headless UI checks passed; native desktop window chrome was not inspected. Review logs and UI artifacts are in `/tmp/mailven-issues-review/`.
