-- Application logs exercising the Logs view's rendering features:
--   - ERROR rows carry a multiline Java stack trace: the row shows the first line
--     plus "(+N lines)", and the expanded detail renders it whitespace-pre-wrapped.
--   - INFO rows carry a nested JSON payload string: pretty-printed in the expanded
--     detail (rows stay compact), with per-field copy buttons.
--   - trace_id feeds a row_links template to a trace viewer.
WITH ticks AS (
  SELECT ts, row_number() OVER (ORDER BY ts) AS n
  FROM generate_series(now() - INTERVAL 24 HOUR, now(), INTERVAL 2 MINUTE) AS g(ts)
),
base AS (
  SELECT
    ts,
    n,
    CASE
      WHEN n % 17 = 0 THEN 'ERROR'
      WHEN n % 7 = 0 THEN 'WARN'
      ELSE 'INFO'
    END AS severity,
    CASE (hash(concat('svc', ts::VARCHAR)) % 3)
      WHEN 0 THEN 'checkout'
      WHEN 1 THEN 'catalog'
      ELSE 'search'
    END AS service,
    lpad((hash(concat('tr', ts::VARCHAR)) % 100000000)::VARCHAR, 8, '0') AS trace_id
  FROM ticks
)
SELECT
  ts AS time,
  severity,
  service,
  'trace-' || trace_id AS trace_id,
  CASE severity
    WHEN 'ERROR' THEN
      'java.sql.SQLTransientConnectionException: HikariPool-1 - Connection is not available, request timed out after 30000ms'
      || chr(10) || chr(9) || 'at com.zaxxer.hikari.pool.HikariPool.createTimeoutException(HikariPool.java:696)'
      || chr(10) || chr(9) || 'at com.zaxxer.hikari.pool.HikariPool.getConnection(HikariPool.java:181)'
      || chr(10) || chr(9) || 'at com.zaxxer.hikari.HikariDataSource.getConnection(HikariDataSource.java:100)'
      || chr(10) || chr(9) || 'at org.hibernate.engine.jdbc.connections.internal.DatasourceConnectionProviderImpl.getConnection(DatasourceConnectionProviderImpl.java:122)'
      || chr(10) || chr(9) || 'at org.hibernate.internal.NonContextualJdbcConnectionAccess.obtainConnection(NonContextualJdbcConnectionAccess.java:38)'
      || chr(10) || chr(9) || 'at com.nocd.' || service || '.repository.OrderRepository.findPending(OrderRepository.java:87)'
      || chr(10) || chr(9) || 'at com.nocd.' || service || '.service.' || upper(service[1]) || service[2:] || 'Service.process(' || upper(service[1]) || service[2:] || 'Service.java:142)'
      || chr(10) || chr(9) || 'at java.base/java.util.concurrent.ThreadPoolExecutor.runWorker(ThreadPoolExecutor.java:1136)'
      || chr(10) || 'Caused by: java.net.SocketTimeoutException: connect timed out'
      || chr(10) || chr(9) || 'at java.base/sun.nio.ch.NioSocketImpl.timedFinishConnect(NioSocketImpl.java:546)'
    WHEN 'WARN' THEN 'Slow query detected: SELECT * FROM orders WHERE status = $1 took ' || (500 + hash(concat('q', ts::VARCHAR)) % 4500) || 'ms'
    ELSE 'Request completed'
  END AS message,
  CASE severity
    WHEN 'INFO' THEN
      '{"event":"http_request","method":"' || CASE WHEN n % 3 = 0 THEN 'POST' ELSE 'GET' END
      || '","path":"/api/' || service || '/' || CASE WHEN n % 2 = 0 THEN 'items' ELSE 'orders' END
      || '","status":200,"duration_ms":' || (10 + hash(concat('d', ts::VARCHAR)) % 240)
      || ',"user":{"id":' || (1000 + hash(concat('u', ts::VARCHAR)) % 9000)
      || ',"plan":"' || CASE WHEN n % 5 = 0 THEN 'premium' ELSE 'standard' END
      || '","region":"us-east-1"},"db":{"queries":' || (1 + hash(concat('dbq', ts::VARCHAR)) % 12)
      || ',"time_ms":' || (2 + hash(concat('dbt', ts::VARCHAR)) % 80) || '}}'
    ELSE NULL
  END AS payload
FROM base
