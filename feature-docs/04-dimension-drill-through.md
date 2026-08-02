# 4. Dimension Drill-Through

**Branch:** `avaitla/dimension-drill-through` (1 commit, off `main`)

## What it does

A per-dimension `drill_through` property naming another explore dashboard. Dimension
values in the **leaderboard** and the **expanded dimension table** get a hover-revealed
drill icon; clicking it navigates to the target explore with the clicked value applied as
a filter — e.g. `/explore/orders_detail?f=country+IN+('Japan')`.

Use case: everyone starts on a high-level overview; clicking a specific dimension value
takes them to a deeper, more detailed dashboard scoped to that value.

```yaml
type: metrics_view
dimensions:
  - column: country
    drill_through: orders_detail    # name of the target explore
```

## How it works

- Proto: `MetricsViewSpec.Dimension.drill_through = 17`; parser passes it through
  (`runtime/parser/parse_metrics_view.go`) — no query-path work needed since the target is
  static config (unlike `uri`, which is evaluated SQL per row)
- `web-common/src/features/dashboards/drill-through.ts` — `gotoDrillThroughExplore()`
  builds the link via the existing explore-mappers (`generateExploreLink` +
  `createInExpression`), so URLs resolve correctly in Rill Developer, Rill Cloud
  (org/project paths from `$page.params`), and embeds
- Leaderboard: `Leaderboard.svelte` (handler) + `LeaderboardRow.svelte` (hover icon,
  styled exactly like the existing `uri` external-link affordance; both icons compose,
  with the uri link shifting left when both are present)
- Dimension table: `DimensionValueHeader.svelte` builds an `onDrill` per row (it has
  state-managers access) → generic `virtualized-table/core/Cell.svelte` renders the icon
- i18n: `dashboard_drill_through` ("Drill through to {name}") in en/es

## Tests

Parser: `TestMetricsViewDimensionDrillThrough` in `runtime/parser/parse_metrics_view_test.go`.
Full dashboard vitest suites pass (912 tests).

## Demo runbook (port 9009)

DuckDB-only:

```bash
mkdir -p /tmp/drilldemo/models /tmp/drilldemo/metrics /tmp/drilldemo/dashboards
cat > /tmp/drilldemo/rill.yaml <<'EOF'
compiler: rillv1
EOF
cat > /tmp/drilldemo/models/orders.sql <<'EOF'
SELECT TIMESTAMP '2026-05-01' + INTERVAL (i % 2200) HOUR AS ordered_at,
  ['United States','Germany','India','Brazil','Japan'][1 + i % 5] AS country,
  ['Electronics','Apparel','Home'][1 + i % 3] AS category,
  10 + (i % 37) * 3.5 AS revenue
FROM range(0, 50000) t(i)
EOF
cat > /tmp/drilldemo/metrics/orders_metrics.yaml <<'EOF'
version: 1
type: metrics_view
display_name: Orders
model: orders
timeseries: ordered_at
dimensions:
  - column: country
    drill_through: orders_detail
  - column: category
measures:
  - name: total_orders
    expression: count(*)
  - name: total_revenue
    expression: sum(revenue)
EOF
cat > /tmp/drilldemo/dashboards/orders_detail.yaml <<'EOF'
type: explore
display_name: Orders Detail
metrics_view: orders_metrics
dimensions: '*'
measures: '*'
defaults:
  time_range: P4W
EOF
cd /tmp/drilldemo && /path/to/rill/rill start --no-open --port 9009 --port-grpc 49009
```

(The metrics view auto-emits `orders_explore`; `orders_detail` is the drill target — in a
real setup it would be a different, more detailed dashboard, possibly on another metrics
view sharing the dimension name.)

**What to show:**

1. On `/explore/orders_explore`, hover a country row (e.g. Japan) in the Country
   leaderboard → a drill icon appears at the row's right edge (aria-label
   "Drill through to orders_detail")
2. Click it → lands on `/explore/orders_detail?f=country+IN+('Japan')` with the filter
   chip applied and all numbers scoped to Japan
3. Expand Country to the dimension table (`?expand_dim=country`) → the same drill icon on
   each row cell
4. Playwright note: use the exact accessible name
   `getByRole("button", { name: "Drill through to orders_detail", exact: true })` —
   the row container also has role=button and will collide with a regex match

**Known follow-up:** only the clicked value carries into the target; carrying the current
time range / other active filters is a small extension of `gotoDrillThroughExplore`.
