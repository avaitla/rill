-- Counter normalization: the one Prometheus-specific transform, done ONCE at the model layer.
-- Converts the cumulative `http_requests_raw.value` into a per-sample `increase`,
-- computed per series (job, instance, handler, status) with reset detection:
-- when the counter goes backwards (process restart), the new value IS the increase.
--
-- After this step, plain SUM(increase) is correct under every slice, grain, filter,
-- and comparison — the property PromQL's `sum by (x) (rate(...))` makes every panel
-- author manage by hand.
SELECT
  time,
  job,
  instance,
  handler,
  status,
  CASE
    WHEN value - lag(value) OVER w < 0 THEN value
    ELSE coalesce(value - lag(value) OVER w, 0)
  END AS increase
FROM http_requests_raw
WINDOW w AS (PARTITION BY job, instance, handler, status ORDER BY time)
