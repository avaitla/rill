-- Synthetic equivalent of the Prometheus examples' gauge metrics
-- `instance_memory_usage_bytes` / `instance_memory_limit_bytes` / CPU, labeled by (app, proc, instance).
--
-- This table also answers "do CPU and memory need to be unioned into one table?":
-- they share the same grain (time, instance) and the same labels, so they are joined
-- into ONE wide table at the model layer, and one metrics view covers both.
-- (In real life this would be `FROM cloudwatch_cpu JOIN cloudwatch_memory USING (time, instance)`.)
--
-- Baked in: checkout-worker-02 leaks memory toward its limit (unused MiB goes critical),
-- and search-api-01 runs hot on CPU at the end of the window (crosses 90%).
WITH ticks AS (
  SELECT ts
  FROM generate_series(now() - INTERVAL 24 HOUR, now(), INTERVAL 5 MINUTE) AS g(ts)
),
inst AS (
  SELECT app, proc, instance
  FROM (VALUES
    ('catalog',  'web',    'catalog-web-01'),
    ('catalog',  'web',    'catalog-web-02'),
    ('checkout', 'worker', 'checkout-worker-01'),
    ('checkout', 'worker', 'checkout-worker-02'),
    ('search',   'api',    'search-api-01'),
    ('search',   'api',    'search-api-02')
  ) t(app, proc, instance)
),
base AS (
  SELECT
    ts,
    app,
    proc,
    instance,
    -- 0..1 ramp over the 24h window (1 = "now"), used to build end-of-window incidents
    greatest(0, 1 - date_diff('minute', ts, now()) / 1440.0) AS ramp,
    (hash(concat(ts::VARCHAR, instance)) % 10) / 100.0 AS jitter,
    (hash(instance) % 628) / 100.0 AS phase
  FROM ticks
  CROSS JOIN inst
)
SELECT
  ts AS time,
  app,
  proc,
  instance,
  2147483648 AS memory_limit_bytes, -- 2 GiB per instance
  CAST(
    (0.40
      + 0.10 * sin(epoch(ts) / 9000.0 + phase)
      + jitter
      + CASE WHEN instance = 'checkout-worker-02' THEN 0.45 * ramp * ramp ELSE 0 END
    ) * 2147483648 AS BIGINT
  ) AS memory_usage_bytes,
  round(
    25
      + 15 * sin(epoch(ts) / 5400.0 + phase)
      + jitter * 80
      + CASE WHEN instance = 'search-api-01' THEN 55 * ramp * ramp ELSE 0 END,
    1
  ) AS cpu_pct
FROM base
