# Prometheus/Grafana examples — before/after demo

Self-contained DuckDB project (no Docker needed): `../../rill start` from this directory.
Mirrors the queries on https://prometheus.io/docs/prometheus/latest/querying/examples/ and an
RDS-fleet Grafana dashboard, to demonstrate two ideas from
`feature-docs/10-grafana-monitoring-draft.md`:

1. **Counter normalization at the model layer** — `models/http_requests_raw.sql` is a cumulative
   `http_requests_total{job, instance, handler, status}` counter (with a counter reset and an
   ongoing 500s incident baked in); `models/http_requests.sql` converts it once into per-series,
   reset-safe increases, after which plain `SUM` is correct for every slice.
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
