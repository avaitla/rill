-- The AUTHENTIC CloudWatch Metric Streams landing format, as it arrives in
-- S3/Athena (see feature-docs/10-grafana-monitoring-draft.md §8):
--
--   metric_name string, namespace string,
--   dimensions map<string,string>,                        -- dynamic labels
--   timestamp bigint,
--   value struct<max,min,sum,count>                       -- pre-aggregated period summary
--
-- Synthesized here by narrowing the wide database_metrics model: each wide row
-- becomes one narrow row per metric, with a plausible {max,min,sum,count} summary
-- (4 samples per 1-minute period).
WITH narrow AS (
  SELECT time, database, 'CPUUtilization' AS metric_name, cpu_pct AS v FROM database_metrics
  UNION ALL
  SELECT time, database, 'FreeStorageSpace', free_storage_gb FROM database_metrics
  UNION ALL
  SELECT time, database, 'DatabaseConnections', connections::DOUBLE FROM database_metrics
)
SELECT
  metric_name,
  'AWS/RDS' AS namespace,
  MAP {
    'DBInstanceIdentifier': database,
    'env': 'prod',
    'region': CASE WHEN database = 'ext-prod-db' THEN 'us-west-2' ELSE 'us-east-1' END
  } AS dimensions,
  epoch(time)::BIGINT AS timestamp,
  {
    'max': CASE WHEN metric_name = 'CPUUtilization' THEN least(v * 1.08, 100) ELSE v * 1.08 END,
    'min': v * 0.92,
    'sum': v * 4,
    'count': 4.0
  } AS value
FROM narrow
