# Roadmap and Backlog Audit

GitHub issues are the source of truth for roadmap tracking:

- Epic index: https://github.com/MB3R-Lab/coroot-graft/issues/21
- [MVP] coroot-graft bootstrap + delivery: https://github.com/MB3R-Lab/coroot-graft/issues/1
- [MVP] Coroot connector + topology normalization: https://github.com/MB3R-Lab/coroot-graft/issues/4
- [MVP] Upstream toolchain orchestration + artifact lifecycle: https://github.com/MB3R-Lab/coroot-graft/issues/8
- [MVP] Coroot publishing surfaces + install packaging: https://github.com/MB3R-Lab/coroot-graft/issues/11
- Runtime signal separation and topology freshness: https://github.com/MB3R-Lab/coroot-graft/issues/14
- Discovery enrichment and explainability follow-ups: https://github.com/MB3R-Lab/coroot-graft/issues/18

This file captures the repository-side tracker bootstrap performed on 2026-03-18 and aligns the repo with the GitHub issue structure.

## Milestone Tracking State

- Historical shipped milestone: `MVP`
- Active backlog milestone: `Post-MVP hardening`
- Pinned delivery index: [#21](https://github.com/MB3R-Lab/coroot-graft/issues/21)

## MVP Summary

The MVP milestone is closed and represented by closed GitHub issues:

- [#1](https://github.com/MB3R-Lab/coroot-graft/issues/1) EPIC: [MVP] coroot-graft bootstrap + delivery
- [#2](https://github.com/MB3R-Lab/coroot-graft/issues/2) TASK: Repo scaffold + governance + versioning baseline
- [#3](https://github.com/MB3R-Lab/coroot-graft/issues/3) TASK: Add CI, smoke, release E2E, and delivery automation
- [#4](https://github.com/MB3R-Lab/coroot-graft/issues/4) EPIC: [MVP] Coroot connector + topology normalization
- [#5](https://github.com/MB3R-Lab/coroot-graft/issues/5) TASK: Implement Coroot auth client + project resolution + snapshot extraction
- [#6](https://github.com/MB3R-Lab/coroot-graft/issues/6) TASK: Build topology_api normalizer + filtering + edge overrides
- [#7](https://github.com/MB3R-Lab/coroot-graft/issues/7) TASK: Add endpoint extraction modes and synthetic entry fallback
- [#8](https://github.com/MB3R-Lab/coroot-graft/issues/8) EPIC: [MVP] Upstream toolchain orchestration + artifact lifecycle
- [#9](https://github.com/MB3R-Lab/coroot-graft/issues/9) TASK: Run upstream Bering and Sheaft as external toolchain
- [#10](https://github.com/MB3R-Lab/coroot-graft/issues/10) TASK: Persist per-project runs, latest artifacts, and HTTP artifact access
- [#11](https://github.com/MB3R-Lab/coroot-graft/issues/11) EPIC: [MVP] Coroot publishing surfaces + install packaging
- [#12](https://github.com/MB3R-Lab/coroot-graft/issues/12) TASK: Export metrics, webhook triggers, and managed Coroot dashboard
- [#13](https://github.com/MB3R-Lab/coroot-graft/issues/13) TASK: Add local Docker harness, production image/chart, compatibility baseline, and install docs

## Active Backlog

### Runtime signal separation and topology freshness

- [#14](https://github.com/MB3R-Lab/coroot-graft/issues/14) EPIC: Runtime signal separation and topology freshness
- [#15](https://github.com/MB3R-Lab/coroot-graft/issues/15) TASK: Add topology presence TTL and stale-membership semantics for empty windows
- [#16](https://github.com/MB3R-Lab/coroot-graft/issues/16) TASK: Add runtime degradation overlay so service-down state affects published gate context
- [#17](https://github.com/MB3R-Lab/coroot-graft/issues/17) TASK: Distinguish posture score and live health score in dashboard, API, and metrics

### Discovery enrichment and explainability follow-ups

- [#18](https://github.com/MB3R-Lab/coroot-graft/issues/18) EPIC: Discovery enrichment and explainability follow-ups
- [#19](https://github.com/MB3R-Lab/coroot-graft/issues/19) TASK: Enrich Coroot-derived topology with richer edge and endpoint evidence without inventing a second discovery engine
- [#20](https://github.com/MB3R-Lab/coroot-graft/issues/20) TASK: Add operator-facing explanation docs and detector heuristics for weak or missing topology signals

## Maintenance Rule

- Keep [#21](https://github.com/MB3R-Lab/coroot-graft/issues/21) pinned.
- Keep this file aligned with the GitHub tracker.
- Add new work as concrete task issues first, then update the index and roadmap.
