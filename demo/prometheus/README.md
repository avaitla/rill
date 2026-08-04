# Prometheus/Grafana examples — before/after demo

Self-contained DuckDB project (no Docker needed): `../../rill start` from this directory.
Mirrors the queries on https://prometheus.io/docs/prometheus/latest/querying/examples/ and an
RDS-fleet Grafana dashboard, to demonstrate two ideas from
`feature-docs/10-grafana-monitoring-draft.md`:

1. **Generated counter normalization via `kind: counter`** — `models/http_requests_raw.sql` is a
   cumulative `http_requests_total{job, instance, handler, status}` counter (with a counter reset
   and an ongoing 500s incident baked in). The metrics view declares `kind: counter` on the value
   column, and the parser generates the reset-safe per-series delta model
   (`http_requests_after__normalized`) automatically — no hand-written window SQL anywhere in this
   project. After normalization, plain `SUM` is correct for every slice.
2. **Measure thresholds** (implemented on this branch) — `thresholds:` on metrics-view measures
   (compact steps `- warn: X` / `- critical: Y`, or `below: true` + `steps:` for
   bad-when-low measures like free disk); rendered in explore big numbers, leaderboards, and
   canvas KPIs from the single declaration.

## Dashboards (in intended viewing order)

| # | Dashboard | What to look at |
|---|---|---|
| 1 | HTTP — BEFORE | Naive `sum(value)` over cumulative counters: meaningless totals, a "traffic dip" that's actually a counter reset, an error rate the incident barely moves. |
| 2 | HTTP — AFTER | Same data normalized: zoom to the last hour — the 5xx error-rate big number turns **red** (ongoing incident ≥ 5%). |
| 3 | Instance resources | Thresholds in both directions: CPU (bad high) vs unused memory (bad low, `below: true`). CPU+memory share grain and labels, so they're **one wide table and one metrics view** — no union gymnastics. |
| 5 | Database fleet | RDS-style: 8 databases × cpu/memory/network/disk/connections/replica lag. Free storage shows **orange** (179 GB < 250 warn), connections **red** (storm ≥ 1000). |
| 4, 6 | Canvases | Grouped sections (markdown headers = Grafana rows) composing MULTIPLE metrics views from different tables — the answer to "cpu and disk come from different CloudWatch tables". |
| 5b | Database inspect | Drill-through target: click a Database value's "Inspect database" link on dashboard 5 — the value arrives filtered with time range and grain carried along. |
| 8 | Binlog pipeline | Logs tab (raw events newest-first with per-row `row_links`), freshness measures (`minutes_since_last_archive` with thresholds), auto-refresh default 1m, and `skip_empty_dimensions` pruning the all-NULL `error_reason`. |

## All-features branch features exercised here

Dashboard 5 additionally demonstrates (all declared in `metrics/database_metrics.yaml`):

- **`annotations:`** — deploy markers from `models/deploy_events.sql` on every measure chart;
  the latest deploy lands right before prod-db-01's connection storm.
- **`table_options:`** — the header's Table selector switches the whole metrics view to the
  staging fleet (`models/database_metrics_staging.sql`, same schema), with a `table=` URL param.
- **Dimension `links:`** — the Database dimension carries a Runbook URL template
  (`{{ value }}`) and an "Inspect database" drill-through to dashboard 5b.
- **`map_column` discovery** — dashboard 7's labels (DBInstanceIdentifier/env/region) are
  discovered from the CloudWatch `dimensions` map, not declared.

## Threshold YAML shapes

```yaml
measures:
  - name: error_rate
    thresholds:          # bad HIGH (default)
      - warn: 0.02
      - critical: 0.05
  - name: free_storage_gb
    thresholds:          # bad LOW
      below: true
      steps:
        - warn: 250
        - critical: 150
```
