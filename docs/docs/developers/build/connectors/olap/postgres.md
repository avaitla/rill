---
title: PostgreSQL
description: Power Rill dashboards using PostgreSQL
sidebar_label: PostgreSQL
sidebar_position: 23
---

[PostgreSQL](https://www.postgresql.org/) is a powerful open-source relational database. Beyond PostgreSQL itself, many analytical systems speak the PostgreSQL wire protocol (e.g. TimescaleDB, AlloyDB, CrateDB, Materialize), so this connector lets Rill query them directly without ingesting data first.

:::info

Rill supports connecting to an existing PostgreSQL database via a read-only OLAP connector and using it to power Rill dashboards with [external tables](/developers/build/connectors/olap#external-olap-tables).

Row-oriented databases like PostgreSQL are generally slower for analytical queries than columnar OLAP engines. For large datasets, consider ingesting the data into an OLAP engine like DuckDB or ClickHouse instead. For small to medium datasets, or for PostgreSQL-compatible analytical systems, querying directly works well.

:::

## Connect to PostgreSQL

Create a `postgres.yaml` file in your `connectors` directory. You can connect via individual connection parameters or a DSN.

### Connection Parameters

```yaml
type: connector
driver: postgres

host: "localhost"
port: 5432
user: "postgres"
password: "{{ .env.connector.postgres.password }}"
dbname: "postgres"
sslmode: "require"
```

### DSN

```yaml
type: connector
driver: postgres
dsn: "{{ .env.connector.postgres.dsn }}"
```

Set the corresponding secret in your `.env` file:

```bash
connector.postgres.dsn="postgresql://user:password@localhost:5432/postgres"
```

## Set as the default OLAP connector

To use PostgreSQL as the project's default OLAP engine, set the following in `rill.yaml`:

```yaml
olap_connector: postgres
```

Alternatively, reference the connector directly from a specific metrics view via its `connector` property. See [using multiple OLAP engines](/developers/build/connectors/olap/multiple-olap) for details.

## Create a metrics view

Metrics views can be defined directly against tables in your PostgreSQL database:

```yaml
version: 1
type: metrics_view
connector: postgres
table: events
timeseries: occurred_at
dimensions:
  - column: country
measures:
  - name: total_events
    expression: count(*)
```

## Additional configuration

| Property            | Description                                                          | Default |
| ------------------- | -------------------------------------------------------------------- | ------- |
| `max_open_conns`    | Maximum number of open database connections (negative for unlimited) | `20`    |
| `conn_max_lifetime` | Maximum lifetime of a connection (Go duration string)                | `1m`    |
| `log_queries`       | Log raw SQL queries sent to the database                             | `false` |

:::note Session timezone

Rill pins the session timezone to UTC to make time-based aggregations deterministic. To override this, set the `timezone` parameter explicitly in your DSN.

:::
