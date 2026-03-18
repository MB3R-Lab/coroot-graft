# Upstream Summary

## Bering

Источники:

- https://github.com/MB3R-Lab/Bering
- https://raw.githubusercontent.com/MB3R-Lab/Bering/main/docs/topology-input-format.md

Ключевой контракт:

- `Bering v0.1.0` является discovery/publishing слоем.
- Стабильные выходные артефакты:
  - `io.mb3r.bering.model@1.0.0`
  - `io.mb3r.bering.snapshot@1.0.0`
- Открытый batch CLI:
  - `bering discover --input ... --out ... [--snapshot-out ...] [--overlay ...] [--discovered-at ...]`
  - `bering validate --input ...`
- Для `coroot-graft` важен batch-путь через explicit `topology_api`, а не runtime OTLP ingest.
- `topology_api` уже предназначен для случаев, когда topology приходит из внешнего inventory/topology source. Это точно соответствует сценарию `Coroot snapshot -> topology_api -> Bering discover`.
- `topology_api` поддерживает:
  - `services[]` с `id`, `name`, `replicas`
  - `edges[]` с `from`, `to`, `kind`, `blocking`
  - `endpoints[]` с `entry_service`, `method/path` или explicit `id`, плюс `predicate_ref`
- Overlay в Bering additive, то есть это корректный механизм для edge/endpoint-тюнинга поверх snapshot-derived topology, но не замена самого discovery engine.

Вывод для `coroot-graft`:

- `coroot-graft` не должен дублировать discovery.
- Его задача: собрать explicit topology input из Coroot и вызвать upstream `bering discover`.
- Рабочий handoff дальше должен идти через стабильный Bering model/snapshot artifact, а не через внутренние Go API Bering.

## Sheaft

Источники:

- https://github.com/MB3R-Lab/Sheaft
- https://raw.githubusercontent.com/MB3R-Lab/Sheaft/main/docs/configuration.md
- https://raw.githubusercontent.com/MB3R-Lab/Sheaft/main/api/schema/report.schema.json

Ключевой контракт:

- `Sheaft v0.1.1` является downstream resilience posture engine и gate.
- Sheaft принимает только whitelist upstream-контрактов:
  - `io.mb3r.bering.model@1.0.0`
  - `io.mb3r.bering.snapshot@1.0.0`
- Открытый batch CLI:
  - `sheaft simulate`
  - `sheaft gate`
  - `sheaft run`
- `sheaft run` пишет как минимум:
  - `model.json`
  - `report.json`
  - `summary.md`
- Rich analysis config живёт отдельно от legacy policy:
  - `--policy` подходит для простого single-profile gate
  - `--analysis` нужен для профилей, baselines, endpoint weights, predicate overlays, contract pinning
- Report schema стабилизирует минимально необходимые поля:
  - `summary`
  - `endpoint_results`
  - `policy_evaluation`
  - опционально `profiles`, `contract_policy`, `provenance`, `parameters`, `diffs`

Поведенческий вывод:

- Sheaft не владеет discovery и не должен быть использован как discovery engine внутри `coroot-graft`.
- Sheaft нужно запускать на Bering artifact, предпочтительно на snapshot artifact, чтобы не терять provenance и snapshot metadata.
- Для `coroot-graft` открытый контракт Sheaft тоже проходит по CLI + JSON artifacts, а не по internal Go packages.

## Coroot

Источники:

- https://raw.githubusercontent.com/coroot/coroot/main/main.go
- https://docs.coroot.com/metrics/custom-metrics
- https://docs.coroot.com/dashboards/overview/
- https://docs.coroot.com/alerting/webhook
- https://docs.coroot.com/alerting/alerts/

Подтверждённые surface area:

- В `main.go` нет отдельного extension SDK, но есть пригодные API/routes:
  - `/api/project/{project}/overview/{view}`
  - `/api/project/{project}/app/{app}`
  - `/api/project/{project}/dashboards`
  - `/api/project/{project}/dashboards/{dashboard}`
  - `/api/project/{project}/panel/data`
  - `/api/project/{project}/integrations/{type}`
- UI dashboards в Coroot работают поверх PromQL и custom metrics.
- Coroot custom metrics документированно забираются scrape-ом через `coroot-cluster-agent` по pod annotations:
  - `coroot.com/scrape-metrics: "true"`
  - `coroot.com/metrics-port`
  - `coroot.com/metrics-path`
  - `coroot.com/metrics-scheme`
- Webhook integration документированно умеет отправлять JSON через template `{{ json . }}`.
- Alerting в Coroot является event source: alerts/deployments/incidents могут служить trigger signal для внешнего recalculation pipeline.

Неформально подтверждённые, но полезные контракты из исходников:

- `/api/login` принимает JSON `email/password` и устанавливает cookie `coroot_session`.
- `overview/map` отдаёт service-map view с приложениями и связями.
- `app/{app}` отдаёт instance/dependency context, достаточный для сборки `services[]` и `edges[]`.
- `dashboards` API хранит dashboard config как JSON с panel groups и PromQL queries.

Вывод для `coroot-graft`:

- Coroot в этой архитектуре не заменяет Bering и не заменяет Sheaft.
- Coroot должен использоваться как:
  1. источник topology/operational context,
  2. источник trigger events,
  3. UI-host для результатов через custom metrics + dashboards.

## Reference Repo: mb3r-stack

Источник:

- https://github.com/MB3R-Lab/mb3r-stack

Что важно для дизайна:

- `mb3r-stack` прямо позиционируется как bundle/integration layer над upstream Bering и Sheaft.
- Он не vendor'ит исходники и не претендует на ownership core logic.
- Это подтверждает правильный архитектурный ход для `coroot-graft`: packaging/orchestration/adaptation layer, а не третий core engine.

## Итоговые архитектурные решения для coroot-graft

- `coroot-graft` должен оркестрировать upstream binaries/images `bering` и `sheaft` как внешний toolchain.
- Discovery path:
  - Coroot API snapshot
  - transform в Bering `topology_api`
  - upstream `bering discover`
- Analysis/gate path:
  - upstream `sheaft run` на Bering snapshot/model artifact
- Публикация обратно в Coroot:
  - Prometheus metrics endpoint для custom metrics
  - managed/custom dashboard в Coroot
  - webhook endpoint для event-driven rerun
- Собственная логика `coroot-graft` должна ограничиваться:
  - Coroot-specific data extraction
  - normalization to Bering input
  - toolchain invocation
  - artifact lifecycle
  - reporting/export to Coroot

Не должно быть внутри `coroot-graft`:

- собственного service discovery engine вместо Bering
- собственного resilience evaluator/gate вместо Sheaft
