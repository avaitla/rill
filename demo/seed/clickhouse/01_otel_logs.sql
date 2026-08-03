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

-- A handful of ERROR logs with a large multi-line stack trace, to exercise the
-- Logs view's multiline rendering (first line in the row, full trace in detail).
INSERT INTO otel_logs
SELECT
  now() - toIntervalMinute(number * 7),
  'checkout',
  'ERROR',
  'java.lang.NullPointerException: Cannot invoke "com.shop.cart.CartService.getItems()" because "this.cartService" is null
	at com.shop.checkout.CheckoutController.processOrder(CheckoutController.java:142)
	at com.shop.checkout.CheckoutController.lambda$submit$3(CheckoutController.java:98)
	at java.base/java.util.Optional.map(Optional.java:260)
	at com.shop.checkout.CheckoutController.submit(CheckoutController.java:97)
	at org.springframework.web.method.support.InvocableHandlerMethod.doInvoke(InvocableHandlerMethod.java:205)
	at org.springframework.web.servlet.DispatcherServlet.doDispatch(DispatcherServlet.java:1089)
	at org.apache.catalina.core.ApplicationFilterChain.internalDoFilter(ApplicationFilterChain.java:174)
	at java.base/java.util.concurrent.ThreadPoolExecutor.runWorker(ThreadPoolExecutor.java:1136)
	at java.base/java.lang.Thread.run(Thread.java:840)
Caused by: com.shop.common.BeanInitializationException: circular dependency detected in cart module
	at com.shop.common.di.Injector.resolve(Injector.java:77)
	... 19 more',
  map('http.method', 'POST', 'http.status_code', '500', 'user.id', concat('u', toString(number % 5)), 'exception.type', 'NullPointerException'),
  map('k8s.namespace.name', 'prod')
FROM numbers(8);

-- A few INFO logs whose body is a large nested JSON payload, to exercise the
-- Logs view's pretty-printed JSON rendering in the expanded row detail.
INSERT INTO otel_logs
SELECT
  now() - toIntervalMinute(number * 11),
  'api',
  'INFO',
  '{"event":"order.created","order":{"id":"ord_8842","total":249.99,"currency":"USD","items":[{"sku":"ELEC-1042","name":"Electronics Pro","qty":1,"price":199.99},{"sku":"HOME-0311","name":"Home Basic","qty":2,"price":25.0}],"shipping":{"method":"express","address":{"city":"Berlin","country":"DE","postal_code":"10115"}}},"customer":{"id":"cus_1187","segment":"pro","consents":{"marketing":true,"analytics":false}},"context":{"trace_id":"a1b2c3d4e5f6","span_id":"0011223344","feature_flags":["new_checkout","fast_shipping"]}}',
  map('http.method', 'POST', 'http.status_code', '201', 'user.id', concat('u', toString(number))),
  map('k8s.namespace.name', 'prod')
FROM numbers(5);
