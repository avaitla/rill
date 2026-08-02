-- Old version of the events table: no http_method or region columns.
SELECT
  now() - INTERVAL (CAST(random() * 43200 AS INT)) MINUTE AS ts,
  ['checkout', 'search', 'worker'][1 + CAST(floor(random() * 3) AS INT)] AS service,
  CAST(10 + random() * 400 AS DECIMAL(8,1)) AS duration_ms
FROM range(3000)
