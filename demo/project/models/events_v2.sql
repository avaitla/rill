-- Current version of the events table: adds http_method and region.
SELECT
  now() - INTERVAL (CAST(random() * 43200 AS INT)) MINUTE AS ts,
  ['checkout', 'search', 'worker'][1 + CAST(floor(random() * 3) AS INT)] AS service,
  CAST(10 + random() * 400 AS DECIMAL(8,1)) AS duration_ms,
  ['GET', 'POST', 'PUT'][1 + CAST(floor(random() * 3) AS INT)] AS http_method,
  ['eu-west', 'us-east', 'ap-south'][1 + CAST(floor(random() * 3) AS INT)] AS region
FROM range(5000)
