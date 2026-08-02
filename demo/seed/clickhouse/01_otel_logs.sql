-- ClickStack/OTel-style structured logs: fixed columns + dynamic attributes in Map columns.
CREATE TABLE otel_logs (
  Timestamp DateTime64(9),
  ServiceName LowCardinality(String),
  SeverityText LowCardinality(String),
  Body String,
  LogAttributes Map(LowCardinality(String), String),
  ResourceAttributes Map(LowCardinality(String), String)
) ENGINE = MergeTree ORDER BY Timestamp;

INSERT INTO otel_logs
SELECT
  now() - toIntervalMinute(rand() % 1440),
  ['checkout','worker','api'][1 + rand() % 3],
  ['INFO','INFO','INFO','WARN','ERROR'][1 + rand() % 5],
  'log body',
  map(
    'http.method', ['GET','POST','PUT'][1 + rand() % 3],
    'http.status_code', ['200','200','404','500'][1 + rand() % 4],
    'user.id', concat('u', toString(rand() % 50))
  ),
  map('k8s.namespace.name', ['prod','batch'][1 + rand() % 2])
FROM numbers(20000);
