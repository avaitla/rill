-- Disaster-recovery replica fleet in us-west-2 — third entry in the `table_options`
-- `table_options` selector. Same schema, near-idle load, nonzero replica lag
-- (it replicates from prod cross-region).
WITH ticks AS (
  SELECT ts
  FROM generate_series(now() - INTERVAL 24 HOUR, now(), INTERVAL 1 MINUTE) AS g(ts)
),
dbs AS (
  SELECT db, base_free_gb, base_conn, engine, region, workload
  FROM (VALUES
    ('dr-db',          480, 6, 'aurora-postgresql', 'us-west-2', 'oltp'),
    ('dr-db-cronjobs', 510, 2, 'aurora-postgresql', 'us-west-2', 'batch'),
    ('dr-db-metabase', 470, 1, 'postgres',          'us-west-2', 'analytics')
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
  CAST(2 + jitter * 6 AS INTEGER) AS replica_lag_s
FROM base
