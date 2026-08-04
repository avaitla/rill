-- Deploy events for chart ANNOTATIONS (metrics view `annotations:` — an upstream feature).
-- Contract: a `time` column, optional `description` / `time_end` / `duration`.
-- The most recent deploy lands right before prod-db-01's connection storm,
-- so the marker on the Connections chart tells the incident story.
SELECT * FROM (VALUES
  (now() - INTERVAL 22 HOUR,   'Deploy api v2.13.0 (batch writer refactor)'),
  (now() - INTERVAL 12 HOUR,   'Deploy cron v1.8.2 (replica failover test)'),
  (now() - INTERVAL 6 HOUR,    'Maintenance: apiserver-02 restart (counter reset)'),
  (now() - INTERVAL 25 MINUTE, 'Deploy api v2.14.1 (connection pool change)')
) t(time, description)
