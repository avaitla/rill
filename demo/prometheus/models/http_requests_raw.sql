-- Synthetic equivalent of Prometheus `http_requests_total{job, instance, handler, status}`:
-- a CUMULATIVE counter sampled once a minute for 24 hours, per series.
-- Two things are deliberately baked in:
--   1. apiserver-02's counter RESETS 6 hours ago (process restart) — naive deltas go negative there.
--   2. apiserver has an ongoing 500s incident in the last 45 minutes — the "after" error-rate
--      measure should cross its critical threshold for the current period.
WITH ticks AS (
  SELECT ts
  FROM generate_series(now() - INTERVAL 24 HOUR, now(), INTERVAL 1 MINUTE) AS g(ts)
),
series AS (
  SELECT job, instance, handler, status
  FROM (VALUES
    ('apiserver', 'apiserver-01'),
    ('apiserver', 'apiserver-02'),
    ('webserver', 'webserver-01'),
    ('webserver', 'webserver-02')
  ) jobs(job, instance)
  CROSS JOIN (VALUES ('/api/comments'), ('/api/users'), ('/healthz')) handlers(handler)
  CROSS JOIN (VALUES ('200'), ('404'), ('500')) statuses(status)
),
increments AS (
  SELECT
    ts,
    job,
    instance,
    handler,
    status,
    CASE status
      -- healthy traffic: diurnal wave + deterministic jitter
      WHEN '200' THEN 40
        + CAST(20 * sin(epoch(ts) / 7200.0) AS BIGINT)
        + CAST(hash(concat(ts::VARCHAR, job, instance, handler)) % 15 AS BIGINT)
      WHEN '404' THEN 1 + CAST(hash(concat(ts::VARCHAR, job, instance, handler, '4')) % 3 AS BIGINT)
      -- 500s: near-zero noise, except the ongoing apiserver incident
      ELSE CASE
        WHEN job = 'apiserver' AND ts > now() - INTERVAL 45 MINUTE
          THEN 12 + CAST(hash(concat(ts::VARCHAR, instance)) % 6 AS BIGINT)
        ELSE CAST(hash(concat(ts::VARCHAR, job, instance, handler, '5')) % 2 AS BIGINT)
      END
    END AS inc,
    -- apiserver-02 restarted 6h ago: its counter starts over from zero
    (instance = 'apiserver-02' AND ts >= now() - INTERVAL 6 HOUR) AS counter_epoch
  FROM ticks
  CROSS JOIN series
)
SELECT
  ts AS time,
  job,
  instance,
  handler,
  status,
  sum(inc) OVER (
    PARTITION BY job, instance, handler, status, counter_epoch
    ORDER BY ts
  ) AS value
FROM increments
