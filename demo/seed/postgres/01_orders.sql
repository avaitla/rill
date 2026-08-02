-- E-commerce orders queried live by the Postgres OLAP connector.
CREATE TABLE orders (
  order_id BIGSERIAL PRIMARY KEY,
  ordered_at TIMESTAMPTZ NOT NULL,
  customer_id INT NOT NULL,
  country TEXT NOT NULL,
  category TEXT NOT NULL,
  product TEXT NOT NULL,
  channel TEXT NOT NULL,
  status TEXT NOT NULL,
  quantity INT NOT NULL,
  unit_price NUMERIC(10,2) NOT NULL,
  revenue NUMERIC(12,2) NOT NULL
);

INSERT INTO orders (ordered_at, customer_id, country, category, product, channel, status, quantity, unit_price, revenue)
SELECT
  ts, (random()*20000)::int + 1,
  (ARRAY['United States','United States','Germany','India','Brazil','Japan','France','Canada'])[1 + (random()*7)::int],
  cat,
  cat || ' ' || (ARRAY['Basic','Plus','Pro','Max'])[1 + (random()*3)::int],
  (ARRAY['web','web','mobile','store'])[1 + (random()*3)::int],
  CASE WHEN random() < 0.9 THEN 'completed' WHEN random() < 0.6 THEN 'returned' ELSE 'cancelled' END,
  qty, price, round(qty * price, 2)
FROM (
  SELECT
    now() - random() * random() * 180 * interval '1 day' - random() * interval '24 hours' AS ts,
    (ARRAY['Electronics','Electronics','Apparel','Home','Sports','Beauty'])[1 + (random()*5)::int] AS cat,
    1 + (random()*4)::int AS qty,
    round((5 + random() * random() * 495)::numeric, 2) AS price
  FROM generate_series(1, 50000)
) t;

CREATE INDEX orders_ordered_at_idx ON orders (ordered_at);
