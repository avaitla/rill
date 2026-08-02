# 5. Dynamic Map Dimensions & Schemaless Support

**Branch:** `avaitla/dynamic-map-dimensions` (4 commits, stacked on
`avaitla/skip-invalid-dimensions` — merge that first)

## What it does

Complete schemaless-observability support, covering both ways variable event fields land
in OLAP tables:

1. **`map_column` dimensions** (ClickStack/OTel style — attributes in a ClickHouse `Map`
   column): keys are discovered in the data at reconcile time and expanded into one
   concrete dimension per key
2. **`columns: '*'` wildcard dimensions** (RawDuck / union-by-name style — attributes as
   individual columns): one dimension per groupable table column, discovered at reconcile
3. **`skip_empty_dimensions`** (metrics view): dimensions whose values are NULL in every
   row are hidden with a warning (fields absent from current data)
4. **`hide_empty_dimensions`** (explore): leaderboards whose values are all NULL/empty
   *under the current filters* hide themselves at display time — filter to one service and
   the other services' attribute leaderboards vanish

```yaml
type: metrics_view
table: otel_logs
timeseries: Timestamp
skip_invalid_dimensions: true
skip_empty_dimensions: true
dimensions:
  - column: ServiceName
  - map_column: LogAttributes          # ClickHouse Map / DuckDB MAP column
    discover: { limit: 50, pattern: '^http\.' }   # both optional (default limit 100)
  - columns: '*'                        # or: every groupable table column
    discover: { pattern: '^(service|http_)' }
measures:
  - name: events
    expression: count(*)
explore:
  hide_empty_dimensions: true
```

## How it works

- Proto: `Dimension.map_column = 18`, `discover_limit = 19`, `discover_pattern = 20`,
  `all_columns = 21`; `MetricsViewSpec.skip_empty_dimensions = 39`;
  `ExploreSpec.hide_empty_dimensions = 23`
- **Discovery/expansion** lives in the executor
  (`runtime/metricsview/executor/executor_dynamic_dimensions.go`,
  `ExpandDynamicDimensions`): map keys via a new dialect method `SelectMapKeys`
  (ClickHouse `arrayJoin(mapKeys(...))`, DuckDB `unnest(map_keys(...))`, frequency-ordered,
  regex-filterable) and value access via `MapAccessExpr` (`col['key']`); table columns via
  the information schema, skipping non-groupable types (array/map/struct/json/bytes).
  Names are sanitized (`user.name` → `user_name`) with the original as display name and
  column — critical for RawDuck's dotted flattened columns. Declared dimensions/measures
  win on collisions. Recomputed every reconcile → new keys/columns appear as data arrives.
- **Empty pruning** in `executor_empty_dimensions.go` (`EmptyDimensions`): one
  `SELECT count(*), count(dim1), ...` scan; the time dimension is never hidden; empty
  tables leave everything visible. Reconciler wires both via
  `pruneEmptyDimensions`/`expandDynamicDimensions` in `metrics_view.go`.
- **`hide_empty_dimensions`** is frontend-only cost-free: each leaderboard reuses its own
  top-values query result and hides (`display:none`) when every value is null/empty-string
  — ClickHouse's `''` missing-key bucket counts as empty. Dimensions with an active filter
  never hide. (`Leaderboard.svelte`, `LeaderboardDisplay.svelte`.)
- Composes with `skip_invalid_dimensions`: a map column missing from the current schema
  degrades to a warning.

## Tests

`runtime/reconcilers/metrics_view_test.go`: `TestMetricsViewMapDimensions` (DuckDB MAP,
frequency order, pattern, spec-vs-validspec), `TestMetricsViewSkipEmptyDimensions`,
`TestMetricsViewAllColumnsDimensions` (union-by-name table, dotted `user.name` column,
empty-column pruning). Parser: `TestMetricsViewMapColumnParsing`.

## Demo A — ClickStack logs on ClickHouse (port 9011)

```bash
docker run -d --rm --name rill-ch -p 18123:8123 \
  -e CLICKHOUSE_USER=rill -e CLICKHOUSE_PASSWORD=rillpass clickhouse/clickhouse-server:24.8
sleep 10
docker exec rill-ch clickhouse-client -u rill --password rillpass -n -q "
CREATE TABLE otel_logs (
  Timestamp DateTime64(9), ServiceName LowCardinality(String), SeverityText LowCardinality(String),
  Body String, LogAttributes Map(LowCardinality(String), String),
  ResourceAttributes Map(LowCardinality(String), String)
) ENGINE = MergeTree ORDER BY Timestamp;
INSERT INTO otel_logs SELECT now() - toIntervalMinute(rand() % 1440),
  ['checkout','worker','api'][1 + rand() % 3], ['INFO','INFO','INFO','WARN','ERROR'][1 + rand() % 5],
  'log body',
  map('http.method', ['GET','POST','PUT'][1 + rand() % 3], 'http.status_code', ['200','200','404','500'][1 + rand() % 4], 'user.id', concat('u', toString(rand() % 50))),
  map('k8s.namespace.name', ['prod','batch'][1 + rand() % 2])
FROM numbers(5000);"

mkdir -p /tmp/chlogs/connectors /tmp/chlogs/metrics
cat > /tmp/chlogs/rill.yaml <<'EOF'
compiler: rillv1
olap_connector: clickhouse
EOF
cat > /tmp/chlogs/connectors/clickhouse.yaml <<'EOF'
type: connector
driver: clickhouse
dsn: "http://rill:rillpass@localhost:18123"
EOF
cat > /tmp/chlogs/metrics/logs.yaml <<'EOF'
version: 1
type: metrics_view
display_name: Log Events
table: otel_logs
timeseries: Timestamp
skip_invalid_dimensions: true
skip_empty_dimensions: true
dimensions:
  - column: ServiceName
  - column: SeverityText
  - map_column: LogAttributes
  - map_column: ResourceAttributes
measures:
  - name: events
    expression: count(*)
explore:
  name: logs_explore
  hide_empty_dimensions: true
EOF
cd /tmp/chlogs && /path/to/rill/rill start --no-open --port 9011 --port-grpc 49011
```

**Show:** auto-discovered leaderboards titled `http.method`, `http.status_code`,
`user.id`, `k8s.namespace.name` next to the fixed columns. Insert a log with a
never-seen attribute key, re-reconcile (touch the yaml) → new leaderboard appears.
Note: the default ClickHouse docker user rejects remote connections — the
`CLICKHOUSE_USER/PASSWORD` env vars are required.

## Demo B — RawDuck wide table (port 9012)

RawDuck (github.com/quackscience/rawduck) shreds raw JSON into typed, dotted columns with
schema evolution. Its stores are plain DuckDB files readable **without** the extension.

```bash
# Get the release (bundles a matching duckdb binary; release targets a specific duckdb version)
curl -sL -o /tmp/rawduck.tar.gz \
  https://github.com/quackscience/rawduck/releases/download/v0.0.2/rawduck-macos-arm64-v0.0.2.tar.gz
tar xzf /tmp/rawduck.tar.gz -C /tmp && chmod +x /tmp/rawduck-macos-arm64/duckdb

/tmp/rawduck-macos-arm64/duckdb -unsigned -c "
LOAD '/tmp/rawduck-macos-arm64/extension/rawduck/rawduck.duckdb_extension';
ATTACH 'rawduck:/tmp/rawstore.db' AS raw;
INSERT INTO raw.ingest.events VALUES
  ('{\"action\": \"click\", \"ts\": \"2026-08-02T10:30:00\", \"user\": {\"name\": \"alice\", \"plan\": \"pro\"}}'),
  ('{\"action\": \"view\",  \"ts\": \"2026-08-02T11:31:00\", \"user\": {\"name\": \"bob\"}}'),
  ('{\"action\": \"purchase\", \"ts\": \"2026-08-02T13:00:00\", \"user\": {\"name\": \"carol\", \"plan\": \"free\"}, \"amount\": 42.5}');"

mkdir -p /tmp/rawdemo/connectors /tmp/rawdemo/metrics
cat > /tmp/rawdemo/rill.yaml <<'EOF'
compiler: rillv1
olap_connector: rawstore
EOF
cat > /tmp/rawdemo/connectors/rawstore.yaml <<'EOF'
type: connector
driver: duckdb
path: "/tmp/rawstore.db"
mode: read
EOF
cat > /tmp/rawdemo/metrics/events.yaml <<'EOF'
version: 1
type: metrics_view
display_name: Raw Events
connector: rawstore
table: events
timeseries: ts
skip_invalid_dimensions: true
skip_empty_dimensions: true
dimensions:
  - columns: '*'
measures:
  - name: events
    expression: count(*)
explore:
  name: events_explore
  hide_empty_dimensions: true
EOF
cd /tmp/rawdemo && /path/to/rill/rill start --no-open --port 9012 --port-grpc 49012
```

**Show:** dimensions for `action`, `user.name`, `user.plan`, `amount` — never declared
anywhere. Then the evolution kicker: stop rill, ingest an event with a new field
(`"region": "eu-west"`) via the rawduck duckdb, restart → a `region` dimension appears
with zero YAML changes. (DuckDB is single-writer: stop rill before ingesting.)

## Demo C — RED services with per-service attributes (port 9013)

The showcase for `hide_empty_dimensions`: RED metrics (rate/errors/duration) over three
services with disjoint attribute sets (checkout: http+payment+cart; search: http+query+
shard; worker: job+queue+retry, no http). Generate heterogeneous NDJSON (script in the
session history, or any generator), ingest via `CALL raw_ingest_file('raw.requests', ...)`,
metrics view with `columns: '*'` + a discover pattern excluding `duration_ms`, RED
measures (`count(*)`, `count(error)`, `count(error)*1.0/count(*)` as percentage,
`quantile_cont(duration_ms, 0.5/0.95)`), explore with `hide_empty_dimensions: true`.

**The money shot:** unfiltered, ~11 attribute leaderboards across all services; click
`worker` in the service leaderboard → the six checkout/search leaderboards vanish, leaving
only `job.name`, `job.queue`, `retry_count`, `error`, while the RED numbers track the
filtered service.

Multiple-OLAP note: demos A and B/C can run in ONE project (per-metrics-view `connector:`),
proving the same YAML feature set works across engines simultaneously.
