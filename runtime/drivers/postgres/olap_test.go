package postgres_test

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	runtimev1 "github.com/rilldata/rill/proto/gen/rill/runtime/v1"
	"github.com/rilldata/rill/runtime/drivers"
	"github.com/rilldata/rill/runtime/pkg/activity"
	"github.com/rilldata/rill/runtime/storage"
	"github.com/rilldata/rill/runtime/testruntime"
	"github.com/rilldata/rill/runtime/testruntime/testmode"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestPgxOLAP(t *testing.T) {
	testmode.Expensive(t)
	_, olap := acquireTestPostgres(t)

	t.Run("test map scan", func(t *testing.T) {
		testOLAP(t, olap)
	})

	t.Run("test empty rows", func(t *testing.T) {
		testEmptyRows(t, olap)
	})

	t.Run("test complex types", func(t *testing.T) {
		testComplexTypes(t, olap)
	})

	t.Run("test null values", func(t *testing.T) {
		testNullValues(t, olap)
	})

	t.Run("test timestamp with time zone", func(t *testing.T) {
		testTimestampWithTimeZone(t, olap)
	})

	t.Run("test numeric types", func(t *testing.T) {
		testNumericTypes(t, olap)
	})

	t.Run("test string types", func(t *testing.T) {
		testStringTypes(t, olap)
	})

	t.Run("test enum type", func(t *testing.T) {
		testEnumType(t, olap)
	})

	t.Run("test dry run", func(t *testing.T) {
		testDryRun(t, olap)
	})

	t.Run("test information schema", func(t *testing.T) {
		testInformationSchema(t, olap)
	})

	t.Run("test exec", func(t *testing.T) {
		testExec(t, olap)
	})

	t.Run("test LoadDDL", func(t *testing.T) {
		testLoadDDL(t, olap)
	})

	t.Run("test query args rebinding", func(t *testing.T) {
		testQueryArgsRebinding(t, olap)
	})

	t.Run("test query schema", func(t *testing.T) {
		testQuerySchema(t, olap)
	})

	t.Run("test session timezone", func(t *testing.T) {
		testSessionTimezone(t, olap)
	})

	t.Run("test dialect queries", func(t *testing.T) {
		testDialectQueries(t, olap)
	})
}

func testOLAP(t *testing.T, olap drivers.OLAPStore) {
	tests := []struct {
		query  string
		args   []any
		result map[string]any
	}{
		{
			"SELECT TRUE AS bool",
			nil,
			map[string]any{"bool": true},
		},
		{
			"SELECT '2021-01-01'::DATE AS date",
			nil,
			map[string]any{"date": time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)},
		},
		{
			"SELECT '2025-01-31 23:59:59.999999'::TIMESTAMP AS datetime",
			nil,
			map[string]any{"datetime": time.Date(2025, 1, 31, 23, 59, 59, 999999000, time.UTC)},
		},
		{
			"SELECT 123::int2 AS smallint",
			nil,
			map[string]any{"smallint": int64(123)},
		},
		{
			"SELECT 123456789 AS integer",
			nil,
			map[string]any{"integer": int64(123456789)},
		},
		{
			"SELECT 99999999999999999999999999999.999999999::NUMERIC(38,9) AS number",
			nil,
			map[string]any{"number": "99999999999999999999999999999.999999999"},
		},
		{
			"SELECT 0.1::NUMERIC(10,1) AS number",
			nil,
			map[string]any{"number": "0.1"},
		},
		{
			"SELECT 3.14::FLOAT AS number",
			nil,
			map[string]any{"number": float64(3.14)},
		},
		{
			"SELECT ARRAY[1, 2, 3] AS arr",
			nil,
			map[string]any{"arr": "{1,2,3}"},
		},
		{
			"SELECT '23:59:59.999999'::TIME AS t",
			nil,
			map[string]any{"t": "23:59:59.999999"}, // TIME is returned as string
		},
		{
			"SELECT weight FROM all_datatypes WHERE age = $1",
			[]any{30},
			map[string]any{"weight": float64(75.4000015258789)}, // FLOAT4 returned as FLOAT64
		},
		{
			"SELECT name FROM all_datatypes WHERE uuid = $1",
			[]any{"8a25ac46-8ad6-4415-9a2e-12aa3962c144"},
			map[string]any{"name": "John Doe"},
		},
	}
	for _, test := range tests {
		t.Run(test.query, func(t *testing.T) {
			rows, err := olap.Query(t.Context(), &drivers.Statement{Query: test.query, Args: test.args})
			require.NoError(t, err)
			defer rows.Close()
			for rows.Next() {
				res := make(map[string]any)
				err = rows.MapScan(res)
				require.NoError(t, err)
				require.Equal(t, test.result, res)
			}
			require.NoError(t, rows.Err())
		})
	}
}

func testEmptyRows(t *testing.T, olap drivers.OLAPStore) {
	rows, err := olap.Query(t.Context(), &drivers.Statement{Query: "SELECT age, weight FROM all_datatypes LIMIT 0"})
	require.NoError(t, err)
	defer rows.Close()

	sc := rows.Schema
	require.Len(t, sc.Fields, 2)
	require.Equal(t, "age", sc.Fields[0].Name)
	require.Equal(t, "weight", sc.Fields[1].Name)
	require.False(t, rows.Next())
	require.Nil(t, rows.Err())
}

func testComplexTypes(t *testing.T, olap drivers.OLAPStore) {
	// Test complex data types (json, jsonb, array)
	rows, err := olap.Query(t.Context(), &drivers.Statement{
		Query: "SELECT personal_info, personal_info2, salary_history FROM all_datatypes WHERE id = 1",
	})
	require.NoError(t, err)
	defer rows.Close()

	require.True(t, rows.Next())
	res := make(map[string]any)
	err = rows.MapScan(res)
	require.NoError(t, err)

	// Verify JSON values (returned as []uint8 byte slices)
	var jsonCol map[string]string
	jsonBytes, ok := res["personal_info"].([]uint8)
	require.True(t, ok, "personal_info should be []uint8")
	err = json.Unmarshal(jsonBytes, &jsonCol)
	require.NoError(t, err)
	require.Equal(t, map[string]string{"hobbies": "Travel, Tech"}, jsonCol)

	var jsonbCol map[string]string
	jsonbBytes, ok := res["personal_info2"].([]uint8)
	require.True(t, ok, "personal_info2 should be []uint8")
	err = json.Unmarshal(jsonbBytes, &jsonbCol)
	require.NoError(t, err)
	require.Equal(t, map[string]string{"job": "Software Engineer"}, jsonbCol)

	// Verify array value (Postgres returns arrays as strings in the format "{val1,val2}")
	require.Equal(t, "{1234567,7654312}", res["salary_history"])

	require.False(t, rows.Next())
	require.NoError(t, rows.Err())
}

func testNullValues(t *testing.T, olap drivers.OLAPStore) {
	// Test NULL handling
	rows, err := olap.Query(t.Context(), &drivers.Statement{
		Query: "SELECT is_married, emp_salary FROM all_datatypes WHERE id = 3",
	})
	require.NoError(t, err)
	defer rows.Close()

	require.True(t, rows.Next())
	res := make(map[string]any)
	err = rows.MapScan(res)
	require.NoError(t, err)

	// Verify NULL values
	require.Nil(t, res["is_married"])
	require.Nil(t, res["emp_salary"])

	require.False(t, rows.Next())
	require.NoError(t, rows.Err())
}

func testTimestampWithTimeZone(t *testing.T, olap drivers.OLAPStore) {
	// Test timestamp with time zone
	rows, err := olap.Query(t.Context(), &drivers.Statement{
		Query: "SELECT last_login FROM all_datatypes WHERE id = 1",
	})
	require.NoError(t, err)
	defer rows.Close()

	require.True(t, rows.Next())
	res := make(map[string]any)
	err = rows.MapScan(res)
	require.NoError(t, err)

	// Verify the timestamp value exists (exact value may vary based on timezone)
	require.NotNil(t, res["last_login"])
	_, ok := res["last_login"].(time.Time)
	require.True(t, ok, "last_login should be a time.Time value")

	require.False(t, rows.Next())
	require.NoError(t, rows.Err())
}

func testNumericTypes(t *testing.T, olap drivers.OLAPStore) {
	// Test various numeric types
	rows, err := olap.Query(t.Context(), &drivers.Statement{
		Query: "SELECT num_of_dependents, age, net_worth, weight, height FROM all_datatypes WHERE id = 1",
	})
	require.NoError(t, err)
	defer rows.Close()

	require.True(t, rows.Next())
	res := make(map[string]any)
	err = rows.MapScan(res)
	require.NoError(t, err)

	// Verify numeric types (all integers return as int64)
	require.Equal(t, int64(2), res["num_of_dependents"])    // smallint -> int64
	require.Equal(t, int64(30), res["age"])                 // integer -> int64
	require.Equal(t, int64(1234567), res["net_worth"])      // bigint -> int64
	require.InDelta(t, float64(75.4), res["weight"], 0.1)   // float4 -> float64
	require.InDelta(t, float64(180.5), res["height"], 0.01) // float8 -> float64

	require.False(t, rows.Next())
	require.NoError(t, rows.Err())
}

func testStringTypes(t *testing.T, olap drivers.OLAPStore) {
	// Test various string/char types
	rows, err := olap.Query(t.Context(), &drivers.Statement{
		Query: "SELECT name, gender, gender_full, nickname, biography FROM all_datatypes WHERE id = 1",
	})
	require.NoError(t, err)
	defer rows.Close()

	require.True(t, rows.Next())
	res := make(map[string]any)
	err = rows.MapScan(res)
	require.NoError(t, err)

	// Verify string types
	require.Equal(t, "John Doe", res["name"])                                                                     // text
	require.Equal(t, "M", res["gender"])                                                                          // character
	require.Equal(t, "Male", res["gender_full"])                                                                  // character varying
	require.Equal(t, "abcd      ", res["nickname"])                                                               // bpchar(10) - padded with spaces
	require.Equal(t, "John is a software engineer who loves to travel and explore new places.", res["biography"]) // text

	require.False(t, rows.Next())
	require.NoError(t, rows.Err())
}

func testEnumType(t *testing.T, olap drivers.OLAPStore) {
	// Test ENUM type
	rows, err := olap.Query(t.Context(), &drivers.Statement{
		Query: "SELECT country FROM all_datatypes WHERE id = 1",
	})
	require.NoError(t, err)
	defer rows.Close()

	require.True(t, rows.Next())
	res := make(map[string]any)
	err = rows.MapScan(res)
	require.NoError(t, err)

	// Verify enum value (should be returned as string)
	require.Equal(t, "IND", res["country"])

	require.False(t, rows.Next())
	require.NoError(t, rows.Err())
}

func testDryRun(t *testing.T, olap drivers.OLAPStore) {
	// Dry run query
	_, err := olap.Query(t.Context(), &drivers.Statement{
		Query:  "SELECT * FROM all_datatypes WHERE age = $1",
		Args:   []any{30},
		DryRun: true,
	})
	require.NoError(t, err)
}

func testInformationSchema(t *testing.T, olap drivers.OLAPStore) {
	// Test All() method to list tables
	tables, _, err := olap.InformationSchema().All(t.Context(), "", 100, "")
	require.NoError(t, err)
	require.NotEmpty(t, tables)

	// Find our test table
	var foundTable *drivers.OlapTable
	for _, table := range tables {
		if table.Name == "all_datatypes" {
			foundTable = table
			break
		}
	}
	require.NotNil(t, foundTable, "all_datatypes table should be in the list")
	require.False(t, foundTable.View)

	// Test Lookup() method
	table, err := olap.InformationSchema().Lookup(t.Context(), foundTable.Database, foundTable.DatabaseSchema, "all_datatypes")
	require.NoError(t, err)
	require.NotNil(t, table)
	require.Equal(t, "all_datatypes", table.Name)
	require.NotNil(t, table.Schema)
	require.NotEmpty(t, table.Schema.Fields)

	// Verify some fields exist
	fieldNames := make(map[string]bool)
	for _, field := range table.Schema.Fields {
		fieldNames[field.Name] = true
	}
	require.True(t, fieldNames["id"])
	require.True(t, fieldNames["name"])
	require.True(t, fieldNames["age"])
	require.True(t, fieldNames["uuid"])
}

func testExec(t *testing.T, olap drivers.OLAPStore) {
	// Test Exec method - create a regular table instead of temp table
	// (temp tables are session-specific and won't work with connection pooling)
	tableName := "test_exec_" + time.Now().Format("20060102150405")

	err := olap.Exec(t.Context(), &drivers.Statement{
		Query: fmt.Sprintf("CREATE TABLE %s (id INT, name TEXT)", tableName),
	})
	require.NoError(t, err)

	// Clean up at the end
	defer func() {
		_ = olap.Exec(t.Context(), &drivers.Statement{
			Query: fmt.Sprintf("DROP TABLE IF EXISTS %s", tableName),
		})
	}()

	// Insert data
	err = olap.Exec(t.Context(), &drivers.Statement{
		Query: fmt.Sprintf("INSERT INTO %s (id, name) VALUES ($1, $2)", tableName),
		Args:  []any{1, "test"},
	})
	require.NoError(t, err)

	// Verify data was inserted
	rows, err := olap.Query(t.Context(), &drivers.Statement{
		Query: fmt.Sprintf("SELECT id, name FROM %s WHERE id = $1", tableName),
		Args:  []any{1},
	})
	require.NoError(t, err)
	defer rows.Close()

	require.True(t, rows.Next())
	res := make(map[string]any)
	err = rows.MapScan(res)
	require.NoError(t, err)
	require.Equal(t, int64(1), res["id"]) // INT returns as int64
	require.Equal(t, "test", res["name"])
}

func testLoadDDL(t *testing.T, olap drivers.OLAPStore) {
	// Test DDL for a table
	table, err := olap.InformationSchema().Lookup(t.Context(), "", "", "all_datatypes")
	require.NoError(t, err)
	err = olap.InformationSchema().LoadDDL(t.Context(), table)
	require.NoError(t, err)
	require.Contains(t, table.DDL, "CREATE TABLE")
	require.Contains(t, table.DDL, "all_datatypes")

	// Create a view and test DDL for it
	tableName := fmt.Sprintf("test_ddl_view_%d", time.Now().UnixNano())
	err = olap.Exec(t.Context(), &drivers.Statement{Query: fmt.Sprintf("CREATE VIEW %s AS SELECT id, name FROM all_datatypes", tableName)})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = olap.Exec(t.Context(), &drivers.Statement{Query: fmt.Sprintf("DROP VIEW IF EXISTS %s", tableName)})
	})

	view, err := olap.InformationSchema().Lookup(t.Context(), "", "", tableName)
	require.NoError(t, err)
	err = olap.InformationSchema().LoadDDL(t.Context(), view)
	require.NoError(t, err)
	require.Contains(t, view.DDL, "CREATE VIEW")
	require.Contains(t, view.DDL, tableName)
}

func testQueryArgsRebinding(t *testing.T, olap drivers.OLAPStore) {
	// The runtime generates SQL with '?' placeholders that the driver must rebind to Postgres '$N' style.
	res, err := olap.Query(t.Context(), &drivers.Statement{
		Query: "SELECT name, age FROM all_datatypes WHERE age > ? AND name ILIKE ? ORDER BY age",
		Args:  []any{26, "%o%"},
	})
	require.NoError(t, err)
	var names []string
	for res.Next() {
		var name string
		var age int
		require.NoError(t, res.Scan(&name, &age))
		names = append(names, name)
	}
	require.NoError(t, res.Err())
	require.NoError(t, res.Close())
	require.Equal(t, []string{"John Doe", "Sophia Davis", "Bob Brown"}, names)

	// Time range filters are passed as time.Time args (see AST.sqlForTimeRange).
	res, err = olap.Query(t.Context(), &drivers.Statement{
		Query: "SELECT count(*) FROM all_datatypes WHERE created_at >= ? AND created_at < ?",
		Args: []any{
			time.Date(2023, 7, 1, 0, 0, 0, 0, time.UTC),
			time.Date(2023, 10, 1, 0, 0, 0, 0, time.UTC),
		},
	})
	require.NoError(t, err)
	require.True(t, res.Next())
	var count int
	require.NoError(t, res.Scan(&count))
	require.NoError(t, res.Close())
	require.Equal(t, 3, count)
}

func testQuerySchema(t *testing.T, olap drivers.OLAPStore) {
	schema, err := olap.QuerySchema(t.Context(), "SELECT id, name, created_at, last_login FROM all_datatypes WHERE id = ?", []any{1})
	require.NoError(t, err)
	require.Len(t, schema.Fields, 4)
	require.Equal(t, "id", schema.Fields[0].Name)
	require.Equal(t, runtimev1.Type_CODE_INT64, schema.Fields[0].Type.Code)
	require.Equal(t, "name", schema.Fields[1].Name)
	require.Equal(t, runtimev1.Type_CODE_STRING, schema.Fields[1].Type.Code)
	require.Equal(t, "created_at", schema.Fields[2].Name)
	require.Equal(t, runtimev1.Type_CODE_TIMESTAMP, schema.Fields[2].Type.Code)
	require.Equal(t, "last_login", schema.Fields[3].Name)
	require.Equal(t, runtimev1.Type_CODE_TIMESTAMP, schema.Fields[3].Type.Code)
}

func testSessionTimezone(t *testing.T, olap drivers.OLAPStore) {
	// The driver pins the session timezone to UTC so dialect SQL can rely on deterministic timestamp casts.
	res, err := olap.Query(t.Context(), &drivers.Statement{Query: "SELECT current_setting('TimeZone') AS tz"})
	require.NoError(t, err)
	require.True(t, res.Next())
	var tz string
	require.NoError(t, res.Scan(&tz))
	require.NoError(t, res.Close())
	require.Equal(t, "UTC", tz)
}

func testDialectQueries(t *testing.T, olap drivers.OLAPStore) {
	d := olap.Dialect()

	grains := []runtimev1.TimeGrain{
		runtimev1.TimeGrain_TIME_GRAIN_MILLISECOND,
		runtimev1.TimeGrain_TIME_GRAIN_SECOND,
		runtimev1.TimeGrain_TIME_GRAIN_MINUTE,
		runtimev1.TimeGrain_TIME_GRAIN_HOUR,
		runtimev1.TimeGrain_TIME_GRAIN_DAY,
		runtimev1.TimeGrain_TIME_GRAIN_WEEK,
		runtimev1.TimeGrain_TIME_GRAIN_MONTH,
		runtimev1.TimeGrain_TIME_GRAIN_QUARTER,
		runtimev1.TimeGrain_TIME_GRAIN_YEAR,
	}

	// DateTruncExpr must produce executable SQL for every grain, timezone and first day/month shift,
	// on both timestamptz (last_login) and timestamp (created_at) columns.
	for _, tz := range []string{"UTC", "America/New_York", "Asia/Kolkata"} {
		for _, col := range []string{"last_login", "created_at"} {
			for _, grain := range grains {
				expr, err := d.DateTruncExpr(&runtimev1.MetricsViewSpec_Dimension{Column: col}, grain, tz, 7, 4)
				require.NoError(t, err)
				res, err := olap.Query(t.Context(), &drivers.Statement{
					Query: fmt.Sprintf("SELECT %s AS t FROM all_datatypes GROUP BY 1 ORDER BY 1 NULLS LAST", expr),
				})
				require.NoError(t, err, "grain %s tz %s col %s", grain, tz, col)
				for res.Next() {
				}
				require.NoError(t, res.Err())
				require.NoError(t, res.Close())
			}
		}
	}

	// SelectTimeRangeBins across a DST transition (America/New_York springs forward on 2024-03-10).
	loc, err := time.LoadLocation("America/New_York")
	require.NoError(t, err)
	sql, args, err := d.SelectTimeRangeBins(
		time.Date(2024, 3, 10, 5, 0, 0, 0, time.UTC), // midnight in New York
		time.Date(2024, 3, 13, 4, 0, 0, 0, time.UTC),
		runtimev1.TimeGrain_TIME_GRAIN_DAY,
		"timestamp", loc, 1, 1,
	)
	require.NoError(t, err)
	res, err := olap.Query(t.Context(), &drivers.Statement{Query: sql, Args: args})
	require.NoError(t, err)
	var bins []time.Time
	for res.Next() {
		var bin time.Time
		require.NoError(t, res.Scan(&bin))
		bins = append(bins, bin)
	}
	require.NoError(t, res.Err())
	require.NoError(t, res.Close())
	// Bins are UTC wall times of New York midnights; the second one reflects the DST shift (EST -> EDT).
	require.Equal(t, []time.Time{
		time.Date(2024, 3, 10, 5, 0, 0, 0, time.UTC),
		time.Date(2024, 3, 11, 4, 0, 0, 0, time.UTC),
		time.Date(2024, 3, 12, 4, 0, 0, 0, time.UTC),
	}, bins)

	// SelectTimeRangeBins must produce executable SQL for every grain.
	for _, grain := range grains {
		sql, args, err := d.SelectTimeRangeBins(
			time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			time.Date(2023, 1, 1, 0, 0, 2, 0, time.UTC),
			grain, "timestamp", loc, 1, 1,
		)
		require.NoError(t, err)
		err = olap.Exec(t.Context(), &drivers.Statement{Query: sql, Args: args})
		require.NoError(t, err, "grain %s", grain)
	}

	// DateDiff + IntervalSubtract compose into the comparison time-shift expression.
	for _, grain := range grains {
		diff, err := d.DateDiff(grain, time.Date(2023, 5, 1, 0, 0, 0, 0, time.UTC), time.Date(2023, 9, 1, 0, 0, 0, 0, time.UTC))
		require.NoError(t, err)
		expr, err := d.IntervalSubtract("created_at", diff, grain)
		require.NoError(t, err)
		err = olap.Exec(t.Context(), &drivers.Statement{
			Query: fmt.Sprintf("SELECT %s FROM all_datatypes LIMIT 1", expr),
		})
		require.NoError(t, err, "grain %s", grain)
	}

	// AnyValueExpression and SafeDivideExpression are used in comparison queries; SafeDivide must return NULL on division by zero.
	res, err = olap.Query(t.Context(), &drivers.Statement{
		Query: fmt.Sprintf(
			"SELECT %s AS name, %s AS ratio, %s AS null_ratio FROM all_datatypes",
			d.AnyValueExpression(`"name"`),
			d.SafeDivideExpression("sum(age)", "count(*)"),
			d.SafeDivideExpression("sum(age)", "sum(0)"),
		),
	})
	require.NoError(t, err)
	require.True(t, res.Next())
	var name string
	var ratio *float64
	var nullRatio *float64
	require.NoError(t, res.Scan(&name, &ratio, &nullRatio))
	require.NoError(t, res.Close())
	require.NotEmpty(t, name)
	require.NotNil(t, ratio)
	require.Nil(t, nullRatio)
}

func acquireTestPostgres(t *testing.T) (drivers.Handle, drivers.OLAPStore) {
	cfg := testruntime.AcquireConnector(t, "postgres")
	conn, err := drivers.Open("postgres", "", "default", cfg, storage.MustNew(t.TempDir(), nil), activity.NewNoopClient(), zap.NewNop())
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	olap, ok := conn.AsOLAP("default")
	require.True(t, ok)

	return conn, olap
}
