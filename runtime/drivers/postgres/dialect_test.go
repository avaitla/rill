package postgres

import (
	"testing"
	"time"

	runtimev1 "github.com/rilldata/rill/proto/gen/rill/runtime/v1"
	"github.com/stretchr/testify/require"
)

func TestDateTruncExpr(t *testing.T) {
	dim := &runtimev1.MetricsViewSpec_Dimension{Column: "occurred_at"}

	tests := []struct {
		name       string
		grain      runtimev1.TimeGrain
		tz         string
		firstDay   int
		firstMonth int
		want       string
	}{
		{
			name:  "day in UTC",
			grain: runtimev1.TimeGrain_TIME_GRAIN_DAY,
			tz:    "UTC",
			want:  `date_trunc('DAY', "occurred_at"::TIMESTAMP)::TIMESTAMP`,
		},
		{
			name:  "hour in timezone",
			grain: runtimev1.TimeGrain_TIME_GRAIN_HOUR,
			tz:    "America/New_York",
			want:  `timezone('America/New_York', date_trunc('HOUR', timezone('America/New_York', "occurred_at"::TIMESTAMPTZ)))::TIMESTAMP`,
		},
		{
			name:     "week with first day sunday",
			grain:    runtimev1.TimeGrain_TIME_GRAIN_WEEK,
			tz:       "UTC",
			firstDay: 7,
			want:     `date_trunc('WEEK', "occurred_at"::TIMESTAMP + INTERVAL '1 DAY')::TIMESTAMP - INTERVAL '1 DAY'`,
		},
		{
			name:       "year with first month april",
			grain:      runtimev1.TimeGrain_TIME_GRAIN_YEAR,
			tz:         "Asia/Kolkata",
			firstMonth: 4,
			want:       `timezone('Asia/Kolkata', date_trunc('YEAR', timezone('Asia/Kolkata', "occurred_at"::TIMESTAMPTZ) + INTERVAL '9 MONTH') - INTERVAL '9 MONTH')::TIMESTAMP`,
		},
		{
			name:  "millisecond uses plural specifier",
			grain: runtimev1.TimeGrain_TIME_GRAIN_MILLISECOND,
			tz:    "UTC",
			want:  `date_trunc('MILLISECONDS', "occurred_at"::TIMESTAMP)::TIMESTAMP`,
		},
	}

	d := DialectPostgres
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := d.DateTruncExpr(dim, tt.grain, tt.tz, tt.firstDay, tt.firstMonth)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}

	t.Run("invalid timezone", func(t *testing.T) {
		_, err := d.DateTruncExpr(dim, runtimev1.TimeGrain_TIME_GRAIN_DAY, "Not/AZone", 1, 1)
		require.Error(t, err)
	})

	t.Run("expression instead of column", func(t *testing.T) {
		got, err := d.DateTruncExpr(&runtimev1.MetricsViewSpec_Dimension{Expression: "created_at + INTERVAL '1 hour'"}, runtimev1.TimeGrain_TIME_GRAIN_DAY, "UTC", 1, 1)
		require.NoError(t, err)
		require.Equal(t, `date_trunc('DAY', (created_at + INTERVAL '1 hour')::TIMESTAMP)::TIMESTAMP`, got)
	})
}

func TestDateDiff(t *testing.T) {
	d := DialectPostgres

	tests := []struct {
		name  string
		grain runtimev1.TimeGrain
		t1    time.Time
		t2    time.Time
		want  string
	}{
		{
			name:  "days",
			grain: runtimev1.TimeGrain_TIME_GRAIN_DAY,
			t1:    time.Date(2024, 1, 1, 23, 0, 0, 0, time.UTC),
			t2:    time.Date(2024, 1, 8, 1, 0, 0, 0, time.UTC),
			want:  "7",
		},
		{
			name:  "months across year boundary",
			grain: runtimev1.TimeGrain_TIME_GRAIN_MONTH,
			t1:    time.Date(2023, 11, 15, 0, 0, 0, 0, time.UTC),
			t2:    time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC),
			want:  "3",
		},
		{
			name:  "weeks",
			grain: runtimev1.TimeGrain_TIME_GRAIN_WEEK,
			t1:    time.Date(2024, 1, 7, 0, 0, 0, 0, time.UTC), // Sunday
			t2:    time.Date(2024, 1, 8, 0, 0, 0, 0, time.UTC), // Monday: crosses a week boundary
			want:  "1",
		},
		{
			name:  "quarters",
			grain: runtimev1.TimeGrain_TIME_GRAIN_QUARTER,
			t1:    time.Date(2023, 12, 31, 0, 0, 0, 0, time.UTC),
			t2:    time.Date(2024, 4, 1, 0, 0, 0, 0, time.UTC),
			want:  "2",
		},
		{
			name:  "years negative",
			grain: runtimev1.TimeGrain_TIME_GRAIN_YEAR,
			t1:    time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			t2:    time.Date(2023, 12, 31, 0, 0, 0, 0, time.UTC),
			want:  "-2",
		},
		{
			name:  "hours",
			grain: runtimev1.TimeGrain_TIME_GRAIN_HOUR,
			t1:    time.Date(2024, 1, 1, 10, 59, 0, 0, time.UTC),
			t2:    time.Date(2024, 1, 1, 12, 1, 0, 0, time.UTC),
			want:  "2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := d.DateDiff(tt.grain, tt.t1, tt.t2)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestIntervalSubtract(t *testing.T) {
	got, err := DialectPostgres.IntervalSubtract(`"ts"`, "3", runtimev1.TimeGrain_TIME_GRAIN_DAY)
	require.NoError(t, err)
	require.Equal(t, `("ts" - (3) * INTERVAL '1 DAY')`, got)
}

func TestSelectTimeRangeBins(t *testing.T) {
	tz, err := time.LoadLocation("America/New_York")
	require.NoError(t, err)

	sql, args, err := DialectPostgres.SelectTimeRangeBins(
		time.Date(2024, 3, 10, 5, 0, 0, 0, time.UTC), // midnight in New York
		time.Date(2024, 3, 13, 4, 0, 0, 0, time.UTC),
		runtimev1.TimeGrain_TIME_GRAIN_DAY,
		"timestamp",
		tz,
		1, 1,
	)
	require.NoError(t, err)
	require.Nil(t, args)
	require.Equal(t,
		`SELECT timezone('America/New_York', tbin)::TIMESTAMP AS "timestamp" FROM generate_series(timezone('America/New_York', '2024-03-10T05:00:00Z'::TIMESTAMPTZ), timezone('America/New_York', '2024-03-13T04:00:00Z'::TIMESTAMPTZ), INTERVAL '1 DAY') AS tbin WHERE tbin < timezone('America/New_York', '2024-03-13T04:00:00Z'::TIMESTAMPTZ)`,
		sql,
	)
}

func TestMiscExpressions(t *testing.T) {
	d := DialectPostgres
	require.Equal(t, `(array_agg("dim"))[1]`, d.AnyValueExpression(`"dim"`))
	require.Equal(t, `(a)/NULLIF(CAST(b AS DOUBLE PRECISION), 0)`, d.SafeDivideExpression("a", "b"))
	require.Equal(t, `::TEXT`, d.GetCastExprForLike())
	require.Equal(t, `"col" DESC NULLS LAST`, d.OrderByExpression("col", true))
	require.Equal(t, `"col" NULLS LAST`, d.OrderByAliasExpression("col", false))
	require.True(t, d.SupportsILike())
	require.False(t, d.CanPivot())
}
