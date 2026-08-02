# 1. Postgres OLAP Connector

**Branch:** `avaitla/postgres-olap-connector` (1 commit, off `main`)

## What it does

Makes Postgres usable as a first-class OLAP engine: a Rill project can set
`olap_connector: postgres` and build metrics views + explore dashboards directly on
Postgres tables, with no ingestion step. Because many systems speak the Postgres wire
protocol (TimescaleDB, AlloyDB, CrateDB, Materialize, …), this opens all of them up.

The driver already implemented Rill's `OLAPStore` interface; this branch completes the
missing pieces:

- **SQL dialect** (`runtime/drivers/postgres/dialect.go`): timezone-aware `date_trunc`
  (with first-day-of-week / first-month-of-year shifts), time-range bins via
  `generate_series` (DST-correct: generated in local wall time), `DateDiff` computed in Go
  (Postgres has no `DATEDIFF`; inputs are constants), `NULLIF`-based safe division
  (Postgres errors on float ÷ 0), `array_agg(...)[1]` any-value (works pre-PG16),
  `::TEXT` casts so ILIKE works on non-text columns
- **Placeholder rebinding**: the runtime generates `?` placeholders; the driver rebinds to
  `$N` via `sqlx.Rebind` when args are present
- **`QuerySchema`** via a `LIMIT 0` wrapper (subqueries need an alias in Postgres)
- **Session pinned to UTC** via pgx runtime params, so timestamp↔timestamptz casts are
  deterministic (overridable by setting `timezone` in the DSN)
- **`pgx/v5` explicit driver name**: binaries that also link pgx/v4 (the admin service)
  register the unversioned `"pgx"` name first, and v4 can't resolve v5's conn configs
- `ImplementsOLAP` on the postgres + supabase specs; default `max_open_conns` 1 → 20
- Runtime integration: postgres in the metricsview executor's time-range resolution, and
  in the SQL resolver's subquery-alias + export paths

Gotchas discovered live: `INTERVAL '1 QUARTER'` is invalid (quarter is a `date_trunc`
field, not an interval unit → use `3 MONTH`); `date_trunc` needs plural `milliseconds`.

## Usage

```yaml
# connectors/postgres.yaml
type: connector
driver: postgres
dsn: "postgresql://user:pass@localhost:5432/mydb"

# rill.yaml
olap_connector: postgres
```

Metrics views then use `table: <postgres table>` directly. Docs page added at
`docs/docs/developers/build/connectors/olap/postgres.md`.

## Tests

- `runtime/drivers/postgres/dialect_test.go` — generated-SQL unit tests (no DB needed)
- `runtime/drivers/postgres/olap_test.go` — testcontainer integration suite incl.
  rebinding, `QuerySchema`, session timezone, and executing every dialect SQL shape.
  Run with Docker up: `RILL_RUNTIME_TEST_MODE=expensive go test ./runtime/drivers/postgres/`

## Demo runbook (port 9009)

Requires local Postgres 16 on 5432 (peer auth for the OS user is fine).

```bash
# 1. Sample data: 120k orders over ~6 months, weighted toward recent dates
psql -h localhost -d postgres -c "DROP DATABASE IF EXISTS rill_demo" -c "CREATE DATABASE rill_demo"
psql -h localhost -d rill_demo <<'EOF'
CREATE TABLE orders (
  order_id BIGSERIAL PRIMARY KEY, ordered_at TIMESTAMPTZ NOT NULL, customer_id INT NOT NULL,
  country TEXT NOT NULL, category TEXT NOT NULL, product TEXT NOT NULL, channel TEXT NOT NULL,
  status TEXT NOT NULL, quantity INT NOT NULL, unit_price NUMERIC(10,2) NOT NULL,
  discount_pct NUMERIC(4,2) NOT NULL, revenue NUMERIC(12,2) NOT NULL);
INSERT INTO orders (ordered_at, customer_id, country, category, product, channel, status, quantity, unit_price, discount_pct, revenue)
SELECT ts, (random()*20000)::int + 1,
  (ARRAY['United States','United States','United States','Germany','Germany','India','India','Brazil','Japan','France','Canada','Australia'])[1 + (random()*11)::int],
  cat, cat || ' ' || (ARRAY['Basic','Plus','Pro','Max','Mini'])[1 + (random()*4)::int],
  (ARRAY['web','web','web','mobile','mobile','store'])[1 + (random()*5)::int],
  CASE WHEN random() < 0.9 THEN 'completed' WHEN random() < 0.6 THEN 'returned' ELSE 'cancelled' END,
  qty, price, disc, round(qty * price * (1 - disc/100), 2)
FROM (SELECT now() - (random() * random() * 180 || ' days')::interval - (random() * 24 || ' hours')::interval AS ts,
  (ARRAY['Electronics','Electronics','Apparel','Apparel','Home','Sports','Beauty','Toys'])[1 + (random()*7)::int] AS cat,
  1 + (random()*4)::int AS qty, round((5 + random() * random() * 495)::numeric, 2) AS price,
  (ARRAY[0,0,0,0,5,10,15,20])[1 + (random()*7)::int]::numeric AS disc
  FROM generate_series(1, 120000)) t;
CREATE INDEX orders_ordered_at_idx ON orders (ordered_at);
EOF

# 2. Project
mkdir -p /tmp/pgdemo/connectors /tmp/pgdemo/metrics
cat > /tmp/pgdemo/rill.yaml <<'EOF'
compiler: rillv1
display_name: Postgres OLAP Demo
olap_connector: postgres
EOF
cat > /tmp/pgdemo/connectors/postgres.yaml <<'EOF'
type: connector
driver: postgres
dsn: "postgres://localhost:5432/rill_demo"
EOF
cat > /tmp/pgdemo/metrics/orders_metrics.yaml <<'EOF'
version: 1
type: metrics_view
display_name: Orders
table: orders
timeseries: ordered_at
dimensions:
  - column: country
  - column: category
  - column: product
  - column: channel
  - column: status
measures:
  - name: total_orders
    expression: count(*)
    format_preset: humanize
  - name: total_revenue
    expression: sum(revenue)
    format_preset: currency_usd
  - name: return_rate
    expression: "count(*) FILTER (WHERE status = 'returned') * 1.0 / count(*)"
    format_preset: percentage
explore:
  name: orders_explore
  display_name: Orders Explore
EOF

# 3. Run (from a build of this branch)
cd /tmp/pgdemo && /path/to/rill/rill start --no-open --port 9009 --port-grpc 49009
```

**What to show** at http://localhost:9009/explore/orders_explore:

- Dashboards query Postgres live (add `log_queries: true` to the connector yaml to tail SQL)
- Time grain switching (hour→year: exercises the `date_trunc` dialect), timezone changes
  (e.g. Asia/Kolkata: month bins land at 18:30 UTC = midnight IST), period comparison
  (comparison joins + safe division), and the Postgres-specific `FILTER (WHERE ...)` measure
- Insert rows with psql and refresh — no ingestion pipeline in between
