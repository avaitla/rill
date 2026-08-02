-- Simulated RED-metrics request log: three services with disjoint attribute
-- sets, unioned by name into one wide, sparse table (RawDuck-style schemaless).
WITH checkout AS (
  SELECT
    now() - INTERVAL (CAST(random() * 43200 AS INT)) MINUTE AS ts,
    'checkout' AS service,
    CAST(20 + random() * random() * 800 AS DECIMAL(9,1)) AS duration_ms,
    CASE WHEN random() < 0.06 THEN ['card_declined', 'gateway_timeout'][1 + CAST(floor(random() * 2) AS INT)] END AS error,
    ['GET', 'POST'][1 + CAST(floor(random() * 2) AS INT)] AS http_method,
    ['200', '200', '200', '402', '500'][1 + CAST(floor(random() * 5) AS INT)] AS http_status,
    ['stripe', 'paypal', 'adyen'][1 + CAST(floor(random() * 3) AS INT)] AS payment_provider,
    CAST(1 + random() * 6 AS INT) AS cart_size
  FROM range(6000)
), search AS (
  SELECT
    now() - INTERVAL (CAST(random() * 43200 AS INT)) MINUTE AS ts,
    'search' AS service,
    CAST(5 + random() * random() * 300 AS DECIMAL(9,1)) AS duration_ms,
    CASE WHEN random() < 0.02 THEN 'shard_unavailable' END AS error,
    'GET' AS http_method,
    ['200', '200', '200', '429'][1 + CAST(floor(random() * 4) AS INT)] AS http_status,
    CAST(1 + random() * 5 AS INT) AS query_terms,
    ['shard-a', 'shard-b', 'shard-c'][1 + CAST(floor(random() * 3) AS INT)] AS shard
  FROM range(8000)
), worker AS (
  SELECT
    now() - INTERVAL (CAST(random() * 43200 AS INT)) MINUTE AS ts,
    'worker' AS service,
    CAST(100 + random() * random() * 20000 AS DECIMAL(9,1)) AS duration_ms,
    CASE WHEN random() < 0.09 THEN ['oom_killed', 'deadline_exceeded'][1 + CAST(floor(random() * 2) AS INT)] END AS error,
    ['send_email', 'resize_image', 'sync_crm'][1 + CAST(floor(random() * 3) AS INT)] AS job_name,
    ['default', 'bulk'][1 + CAST(floor(random() * 2) AS INT)] AS job_queue,
    CAST(random() * 3 AS INT) AS retry_count
  FROM range(4000)
)
SELECT * FROM checkout
UNION ALL BY NAME SELECT * FROM search
UNION ALL BY NAME SELECT * FROM worker
