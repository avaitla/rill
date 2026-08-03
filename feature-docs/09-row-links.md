# 9. Row Links & View Rows

**Branch:** `avaitla/row-links` (stacked on `avaitla/logs-view`)

## What it does

1. **`row_links`** on a metrics view attach external links to **every raw row** in the
   Logs view — e.g. a `trace_id` linking to Jaeger/Tempo/Datadog. Each `url` is a
   template where `{{ <column> }}` placeholders resolve to the URL-encoded value of
   that column in the clicked row.
2. **View rows**: leaderboard dimension values gain a "View rows" action that jumps to
   the explore's Logs view with that value applied as a filter (carrying the current
   filters, time range and timezone). Click a bucket → read its events.

```yaml
type: metrics_view
row_links:
  - label: Trace in Datadog
    url: "https://app.datadoghq.com/apm/traces?query=service%3A{{ service }}"
```

## How it works

- Proto: `MetricsViewSpec.RowLink {label, url}`, `row_links = 42`; parser validates
  absolute http(s) URLs (placeholders substituted before parsing), label defaults to
  the hostname
- `logs-view/row-links.ts` — `resolveRowLink(template, row)`; rendered as a leading
  icon column in the Logs table
- "View rows" navigates via `generateExploreLink` with `activePage: LOGS` (on the
  all-features branch it is an entry in the dimension value links menu; on this
  standalone branch it is a hover icon)

## Demo

All-features demo: the logs metrics view links each row to Grafana
(`service={{ ServiceName }}&severity={{ SeverityText }}`); RED rows link to a Datadog
trace search. Hover `worker` in a Service leaderboard → "View rows" → Logs view
filtered to worker.
