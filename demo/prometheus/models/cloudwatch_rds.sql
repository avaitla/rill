-- Thin adapter over the narrow CloudWatch landing table: convert the epoch
-- timestamp and keep the label map and summary struct as-is. The metrics view
-- does the rest: map_column discovery expands the labels, and measures apply
-- the summary re-aggregation algebra (avg = sum(sum)/sum(count), never avg of avgs).
SELECT
  to_timestamp(timestamp) AS time,
  metric_name,
  namespace,
  dimensions,
  value
FROM cloudwatch_metrics_raw
