# 6. Metrics View Table Options

**Branch:** `avaitla/metrics-view-table-options` (1 commit, stacked on
`avaitla/skip-invalid-dimensions` — merge that first)

## What it does

One metrics view backed by **multiple selectable tables**, with a dropdown in the
dashboard header ("Table events_v2 ▾") and the selection persisted in the **`table` URL
param**. The headline use case is table versions: an old table may lack columns the new
one has — combined with `skip_invalid_dimensions`, selecting the old version simply shows
fewer dimensions instead of erroring.

```yaml
type: metrics_view
table: events_v2                       # default table
table_options: [events_v1, events_v2]  # user-selectable tables
skip_invalid_dimensions: true
```

## How it works

**Backend** — the parser emits one **variant metrics view per additional table**
(`<name>__<sanitized-table>`) sharing the primary's full spec but with `table` swapped,
and records the option→variant mapping on the primary spec
(`MetricsViewSpec.TableOption {table, metrics_view}`, field 40; `ExplorePreset.table_option = 41`
for URL state). Variants are inserted alongside the primary (collision-prechecked, since
errors are forbidden after the first `insertResource`), get the same refs, and reconcile
**independently** — which is exactly where `skip_invalid_dimensions` prunes per table.
All existing query APIs work against variants unchanged; zero query-path changes.

**Frontend** — one substitution point: the state managers
(`web-common/src/features/dashboards/state-managers/state-managers.ts`) expose the
*effective* metrics view name as a **pure derived** store (variant name when a table
option is selected, else primary), and the merged `validSpecStore` swaps in the variant's
valid spec (fetched via `useResource`) while carrying the primary's `tableOptions` forward
so the dropdown stays populated. `Dashboard.svelte` threads the effective name to all
children; dimensions absent from the selected variant disappear from the dashboard.
`TableSelector.svelte` renders the dropdown in the header; `setTableOption` +
`selectedTableOption` + the `table` URL param follow the standard url-state pipeline
(cleaned when equal to the default table).

### Hard-won implementation notes (read before touching this code)

- **Do not imperatively `set()` a writable from inside another store's `derived`
  callback**: Svelte component auto-subscriptions (`$store`) did not reliably observe such
  writes (plain `derived` chains did). The fix was a pure derived for the effective name,
  plus **explicit `.subscribe()`** in `Dashboard.svelte` for both the name and the
  time-controls store.
- **`useResource`'s `select` receives the raw `V1GetResourceResponse`** — the path must
  start with `data.resource.` (a missing `.resource` made the variant spec silently
  `undefined`, falling back to the primary forever).
- The composite store keeps `Writable` shape (`{subscribe: derived, set/update: base}`)
  because `StateManagersProvider` writes the primary name during visual editing.

## Known follow-up

Named time ranges ("All time", presets) are resolved against the **default** table's time
bounds at dashboard load and are not re-resolved per table option
(`DashboardStateDataLoader` resolves via the primary before the selection applies). This
only matters when option tables have **disjoint** time coverage; real table versions
overlap, where it's invisible. Fix would move the loader's time resolution onto the
effective metrics view.

## Tests

`TestMetricsViewTableOptions` in `runtime/reconcilers/metrics_view_test.go`: primary
carries the mapping (`[{events_v2, mv}, {events_v1, mv__events_v1}]`); the variant is
backed by the old table with missing columns pruned and 2 warnings.

## Demo runbook (port 9014)

```bash
# Two versions of a table with overlapping time coverage; v1 lacks http_method/region
duckdb /tmp/versions.db -c "
CREATE TABLE events_v1 AS
SELECT TIMESTAMP '2026-07-15' + INTERVAL (CAST(random()*25920 AS INT)) MINUTE AS ts,
       ['checkout','search','worker'][1 + CAST(random()*2.99 AS INT)] AS service,
       CAST(10 + random()*400 AS DECIMAL(8,1)) AS duration_ms
FROM range(3000);
CREATE TABLE events_v2 AS
SELECT TIMESTAMP '2026-07-15' + INTERVAL (CAST(random()*25920 AS INT)) MINUTE AS ts,
       ['checkout','search','worker'][1 + CAST(random()*2.99 AS INT)] AS service,
       CAST(10 + random()*400 AS DECIMAL(8,1)) AS duration_ms,
       ['GET','POST','PUT'][1 + CAST(random()*2.99 AS INT)] AS http_method,
       ['eu-west','us-east','ap-south'][1 + CAST(random()*2.99 AS INT)] AS region
FROM range(5000);"

mkdir -p /tmp/verdemo/connectors /tmp/verdemo/metrics
cat > /tmp/verdemo/rill.yaml <<'EOF'
compiler: rillv1
display_name: Table Versions Demo
olap_connector: versions
EOF
cat > /tmp/verdemo/connectors/versions.yaml <<'EOF'
type: connector
driver: duckdb
path: "/tmp/versions.db"
mode: read
EOF
cat > /tmp/verdemo/metrics/events.yaml <<'EOF'
version: 1
type: metrics_view
display_name: Events
connector: versions
table: events_v2
table_options: [events_v1, events_v2]
timeseries: ts
skip_invalid_dimensions: true
dimensions:
  - column: service
  - column: http_method
  - column: region
measures:
  - name: events
    expression: count(*)
  - name: p95_duration
    expression: quantile_cont(duration_ms, 0.95)
explore:
  name: events_explore
  display_name: Events
EOF
cd /tmp/verdemo && /path/to/rill/rill start --no-open --port 9014 --port-grpc 49014
```

**What to show** at http://localhost:9014/explore/events_explore:

1. Resources API shows both metrics views: `events` (4 dims + the options mapping) and
   `events__events_v1` (2 dims — `http_method`/`region` pruned, with warnings)
2. Header dropdown reads "Table events_v2"; three leaderboards, 5,000 events
3. Switch to `events_v1` → URL gains `?table=events_v1`; only the `service` leaderboard
   remains; totals become 3,000; footer row count follows
4. Deep-link `?table=events_v1` directly → same state; switch back to v2 → the param
   cleans away (default table)
