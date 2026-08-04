-- RDS-style fleet metrics: one row per (minute, database instance), all gauges —
-- the shape CloudWatch Metric Streams lands in after export. Mirrors a real
-- production dashboard: Free Storage Space, CPU Utilization, connections, replica lag,
-- plus memory and network for the full cpu/memory/network/disk quartet.
--
-- Baked-in stories (so thresholds visibly fire on the current period):
--   - prod-db-cronjobs is low on disk (~179 GB free) and runs warm CPU at the end.
--   - prod-db-metabase had a storage step-drop 10 hours ago (big table load).
--   - prod-db-01 has a connection storm right now.
--   - prod-db-cronjobs had a replica-lag spike ~12h ago; everyone else is ~0.
WITH ticks AS (
  SELECT ts
  FROM generate_series(now() - INTERVAL 24 HOUR, now(), INTERVAL 1 MINUTE) AS g(ts)
),
dbs AS (
  SELECT db, base_free_gb, base_conn
  FROM (VALUES
    ('prod-db',            242, 40),
    ('prod-db-01',         235, 65),
    ('prod-db-cronjobs',   181, 9),
    ('prod-db-fivetran',   250, 3),
    ('prod-db-metabase',   452, 4),
    ('prod-db-offline-02', 795, 6),
    ('ext-prod-db',        608, 4),
    ('prod-reports-v3',    435, 3)
  ) t(db, base_free_gb, base_conn)
),
base AS (
  SELECT
    ts,
    db,
    base_free_gb,
    base_conn,
    greatest(0, 1 - date_diff('minute', ts, now()) / 1440.0) AS ramp,
    (hash(concat(ts::VARCHAR, db)) % 100) / 100.0 AS jitter,
    (hash(db) % 628) / 100.0 AS phase
  FROM ticks
  CROSS JOIN dbs
)
SELECT
  ts AS time,
  db AS database,
  -- CPU %: diurnal wave; cronjobs is bursty and runs warm at the end of the window
  round(
    CASE WHEN db = 'prod-db-cronjobs'
      THEN 25 + 20 * sin(epoch(ts) / 1800.0 + phase) + jitter * 25 + 35 * ramp * ramp
      ELSE 8 + 6 * sin(epoch(ts) / 7200.0 + phase) + jitter * 10
    END, 1
  ) AS cpu_pct,
  -- Memory used %: slow wave per instance; fivetran creeps high at the end
  round(
    55 + 12 * sin(epoch(ts) / 14400.0 + phase) + jitter * 5
      + CASE WHEN db = 'prod-db-fivetran' THEN 25 * ramp * ramp ELSE 0 END, 1
  ) AS memory_used_pct,
  -- Network MB/s (rx+tx): correlated with CPU activity
  round(
    CASE WHEN db = 'prod-db-cronjobs'
      THEN 20 + 15 * sin(epoch(ts) / 1800.0 + phase) + jitter * 12
      ELSE 5 + 4 * sin(epoch(ts) / 7200.0 + phase) + jitter * 6
    END, 2
  ) AS network_mbps,
  -- Free storage GB: slow decline; metabase steps down 10h ago (big table load)
  round(
    base_free_gb
      - 2.0 * (1 - ramp)  -- slow burn over the day (rendered right-to-left: full burn at window start)
      - CASE WHEN db = 'prod-db-metabase' AND ts > now() - INTERVAL 10 HOUR THEN 175 ELSE 0 END
      + jitter, 1
  ) AS free_storage_gb,
  -- Connections: steady baseline; prod-db-01 has a storm in the last 20 minutes
  CAST(
    base_conn + jitter * 6
      + CASE WHEN db = 'prod-db-01' AND ts > now() - INTERVAL 20 MINUTE THEN 1400 * ramp ELSE 0 END
    AS INTEGER
  ) AS connections,
  -- Replica lag seconds: ~0 normally; cronjobs spiked ~12h ago
  CASE
    WHEN db = 'prod-db-cronjobs'
      AND ts BETWEEN now() - INTERVAL 720 MINUTE AND now() - INTERVAL 705 MINUTE
      THEN 300 + CAST(jitter * 300 AS INTEGER)
    WHEN db = 'prod-db-offline-02' THEN CAST(jitter * 2 AS INTEGER)
    ELSE 0
  END AS replica_lag_s
FROM base
