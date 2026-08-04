-- Binlog-pipeline events: one row per archived binlog file (the case-study-2 shape —
-- a table that is ALREADY the ideal metrics view input, no scraping involved).
-- Powers the Logs view (row-level recent events), row_links (log_name URL templates),
-- freshness measures (minutes since last archive), and skip_empty_dimensions
-- (error_reason is all-NULL and gets pruned with a reconcile warning).
WITH seq AS (
  SELECT ts, row_number() OVER (ORDER BY ts) AS n
  FROM generate_series(now() - INTERVAL 24 HOUR, now(), INTERVAL 5 MINUTE) AS g(ts)
)
SELECT
  ts AS archived_at,
  'mysql-bin-changelog.' || (675000 + n) AS log_name,
  round(20 + (hash(concat('sz', ts::VARCHAR)) % 1100) / 10.0, 1) AS size_mb,
  CASE WHEN ts >= now() - INTERVAL 5 MINUTE
    THEN NULL
    ELSE ts + INTERVAL 1 MINUTE * (1 + hash(concat('lag', ts::VARCHAR)) % 3)
  END AS processed_at,
  CASE WHEN ts >= now() - INTERVAL 5 MINUTE THEN NULL
    ELSE 1 + hash(concat('lag', ts::VARCHAR)) % 3
  END AS lag_min,
  CASE WHEN ts >= now() - INTERVAL 5 MINUTE THEN 'pending' ELSE 'processed' END AS status,
  CAST(NULL AS VARCHAR) AS error_reason
FROM seq
