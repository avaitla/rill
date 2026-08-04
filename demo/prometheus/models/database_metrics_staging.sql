-- Staging fleet with the SAME schema as database_metrics, powering the
-- `table_options` feature: one metrics view, a table selector in the dashboard,
-- and a `table=` URL param switching between prod and staging.
WITH ticks AS (
  SELECT ts
  FROM generate_series(now() - INTERVAL 24 HOUR, now(), INTERVAL 1 MINUTE) AS g(ts)
),
dbs AS (
  SELECT db, base_free_gb, base_conn, engine, region, workload
  FROM (VALUES
    ('staging-db',          480, 6, 'aurora-postgresql', 'us-east-1', 'oltp'),
    ('staging-db-cronjobs', 510, 2, 'aurora-postgresql', 'us-east-1', 'batch'),
    ('staging-db-metabase', 470, 1, 'postgres',          'us-east-1', 'analytics')
  ) t(db, base_free_gb, base_conn, engine, region, workload)
),
base AS (
  SELECT
    ts,
    db,
    base_free_gb,
    base_conn,
    engine,
    region,
    workload,
    (hash(concat(ts::VARCHAR, db)) % 100) / 100.0 AS jitter,
    (hash(db) % 628) / 100.0 AS phase
  FROM ticks
  CROSS JOIN dbs
)
SELECT
  ts AS time,
  db AS database,
  engine,
  region,
  workload,
  round(4 + 3 * sin(epoch(ts) / 7200.0 + phase) + jitter * 6, 1) AS cpu_pct,
  round(35 + 8 * sin(epoch(ts) / 14400.0 + phase) + jitter * 4, 1) AS memory_used_pct,
  round(1 + 1.5 * sin(epoch(ts) / 7200.0 + phase) + jitter * 2, 2) AS network_mbps,
  round(base_free_gb + jitter, 1) AS free_storage_gb,
  CAST(base_conn + jitter * 3 AS INTEGER) AS connections,
  0 AS replica_lag_s
FROM base
