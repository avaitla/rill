package postgres

import (
	"fmt"
	"time"

	runtimev1 "github.com/rilldata/rill/proto/gen/rill/runtime/v1"
	"github.com/rilldata/rill/runtime/drivers"
	"github.com/rilldata/rill/runtime/pkg/timeutil"
)

type dialect struct {
	drivers.BaseDialect
}

var DialectPostgres drivers.Dialect = func() drivers.Dialect {
	d := &dialect{}
	d.BaseDialect = drivers.NewBaseDialect(drivers.DialectNamePostgres, drivers.DoubleQuotesEscapeIdentifier, drivers.DoubleQuotesEscapeIdentifier)
	return d
}()

// ConvertToDateTruncSpecifier returns the date_trunc field name for the given grain.
// Postgres only recognizes the plural form for sub-second fields.
func (d *dialect) ConvertToDateTruncSpecifier(grain runtimev1.TimeGrain) string {
	if grain == runtimev1.TimeGrain_TIME_GRAIN_MILLISECOND {
		return "MILLISECONDS"
	}
	return d.BaseDialect.ConvertToDateTruncSpecifier(grain)
}

// GetCastExprForLike casts operands to text so ILIKE also works on non-text columns (e.g. numbers, UUIDs).
func (d *dialect) GetCastExprForLike() string {
	return "::TEXT"
}

func (d *dialect) OrderByExpression(name string, desc bool) string {
	res := d.EscapeIdentifier(name)
	if desc {
		res += " DESC"
	}
	res += " NULLS LAST"
	return res
}

func (d *dialect) OrderByAliasExpression(name string, desc bool) string {
	res := d.EscapeAlias(name)
	if desc {
		res += " DESC"
	}
	res += " NULLS LAST"
	return res
}

// AnyValueExpression picks an arbitrary value from the group.
// Postgres only gained any_value() in v16, so use array_agg for compatibility with older versions
// and Postgres-compatible systems; it works for values of any type.
func (d *dialect) AnyValueExpression(expr string) string {
	return fmt.Sprintf("(array_agg(%s))[1]", expr)
}

// SafeDivideExpression returns a division expression that evaluates to NULL instead of erroring when the denominator is zero.
// Unlike DuckDB, Postgres raises a division_by_zero error even for float division.
func (d *dialect) SafeDivideExpression(numExpr, denExpr string) string {
	return fmt.Sprintf("(%s)/NULLIF(CAST(%s AS DOUBLE PRECISION), 0)", numExpr, denExpr)
}

// DateTruncExpr truncates a time expression to the given grain, optionally in a target timezone,
// and returns the result as a timestamp in UTC wall time.
// It relies on the connection's session timezone being pinned to UTC (see getDB),
// so casts between timestamp and timestamptz are deterministic.
func (d *dialect) DateTruncExpr(dim *runtimev1.MetricsViewSpec_Dimension, grain runtimev1.TimeGrain, tz string, firstDayOfWeek, firstMonthOfYear int) (string, error) {
	if tz == "UTC" || tz == "Etc/UTC" {
		tz = ""
	}
	if tz != "" {
		_, err := time.LoadLocation(tz)
		if err != nil {
			return "", fmt.Errorf("invalid time zone %q: %w", tz, err)
		}
	}

	specifier := d.ConvertToDateTruncSpecifier(grain)

	var expr string
	if dim.Expression != "" {
		expr = fmt.Sprintf("(%s)", dim.Expression)
	} else {
		expr = d.EscapeIdentifier(dim.Column)
	}

	var shift string
	if grain == runtimev1.TimeGrain_TIME_GRAIN_WEEK && firstDayOfWeek > 1 {
		offset := 8 - firstDayOfWeek
		shift = fmt.Sprintf("%d DAY", offset)
	} else if grain == runtimev1.TimeGrain_TIME_GRAIN_YEAR && firstMonthOfYear > 1 {
		offset := 13 - firstMonthOfYear
		shift = fmt.Sprintf("%d MONTH", offset)
	}

	if tz == "" {
		if shift == "" {
			return fmt.Sprintf("date_trunc('%s', %s::TIMESTAMP)::TIMESTAMP", specifier, expr), nil
		}
		return fmt.Sprintf("date_trunc('%s', %s::TIMESTAMP + INTERVAL '%s')::TIMESTAMP - INTERVAL '%s'", specifier, expr, shift, shift), nil
	}

	// timezone('<tz>', <timestamptz>) converts to wall time in <tz>; truncate there, then convert back and cast to UTC wall time.
	if shift == "" {
		return fmt.Sprintf("timezone('%s', date_trunc('%s', timezone('%s', %s::TIMESTAMPTZ)))::TIMESTAMP", tz, specifier, tz, expr), nil
	}
	return fmt.Sprintf("timezone('%s', date_trunc('%s', timezone('%s', %s::TIMESTAMPTZ) + INTERVAL '%s') - INTERVAL '%s')::TIMESTAMP", tz, specifier, tz, expr, shift, shift), nil
}

// DateDiff returns the number of grain boundaries crossed between t1 and t2 (in UTC).
// Postgres has no DATEDIFF function, but since both timestamps are known constants,
// the diff is computed in Go and emitted as an integer literal.
func (d *dialect) DateDiff(grain runtimev1.TimeGrain, t1, t2 time.Time) (string, error) {
	t1 = t1.UTC()
	t2 = t2.UTC()

	var n int64
	switch grain {
	case runtimev1.TimeGrain_TIME_GRAIN_MILLISECOND:
		n = t2.Truncate(time.Millisecond).UnixMilli() - t1.Truncate(time.Millisecond).UnixMilli()
	case runtimev1.TimeGrain_TIME_GRAIN_SECOND:
		n = t2.Truncate(time.Second).Unix() - t1.Truncate(time.Second).Unix()
	case runtimev1.TimeGrain_TIME_GRAIN_MINUTE:
		n = int64(t2.Truncate(time.Minute).Sub(t1.Truncate(time.Minute)) / time.Minute)
	case runtimev1.TimeGrain_TIME_GRAIN_HOUR:
		n = int64(t2.Truncate(time.Hour).Sub(t1.Truncate(time.Hour)) / time.Hour)
	case runtimev1.TimeGrain_TIME_GRAIN_DAY:
		n = int64(t2.Truncate(24*time.Hour).Sub(t1.Truncate(24*time.Hour)) / (24 * time.Hour))
	case runtimev1.TimeGrain_TIME_GRAIN_WEEK:
		w1 := timeutil.TruncateTime(t1, timeutil.TimeGrainWeek, time.UTC, 1, 1)
		w2 := timeutil.TruncateTime(t2, timeutil.TimeGrainWeek, time.UTC, 1, 1)
		n = int64(w2.Sub(w1) / (7 * 24 * time.Hour))
	case runtimev1.TimeGrain_TIME_GRAIN_MONTH:
		n = int64(t2.Year()-t1.Year())*12 + int64(t2.Month()-t1.Month())
	case runtimev1.TimeGrain_TIME_GRAIN_QUARTER:
		q1 := (int(t1.Month()) - 1) / 3
		q2 := (int(t2.Month()) - 1) / 3
		n = int64(t2.Year()-t1.Year())*4 + int64(q2-q1)
	case runtimev1.TimeGrain_TIME_GRAIN_YEAR:
		n = int64(t2.Year() - t1.Year())
	default:
		return "", fmt.Errorf("unsupported time grain %q for DateDiff in %s dialect", grain.String(), d.String())
	}
	return fmt.Sprintf("%d", n), nil
}

func (d *dialect) IntervalSubtract(tsExpr, unitExpr string, grain runtimev1.TimeGrain) (string, error) {
	// Postgres does not support INTERVAL (n) DAY with a dynamic count, but multiplying an integer with a unit interval works.
	return fmt.Sprintf("(%s - (%s) * INTERVAL '%s')", tsExpr, unitExpr, d.intervalForGrain(grain)), nil
}

// intervalForGrain returns a Postgres interval literal spanning one unit of the given grain.
// QUARTER is a valid date_trunc field but not a valid interval unit in Postgres, so it maps to 3 months.
func (d *dialect) intervalForGrain(grain runtimev1.TimeGrain) string {
	if grain == runtimev1.TimeGrain_TIME_GRAIN_QUARTER {
		return "3 MONTH"
	}
	return "1 " + d.ConvertToDateTruncSpecifier(grain)
}

// SelectTimeRangeBins generates one row per grain-sized bin between start and end (end exclusive).
// The series is generated in the target timezone's wall time so bins align with local calendar boundaries (including DST),
// then converted back to UTC wall time to match the output of DateTruncExpr.
func (d *dialect) SelectTimeRangeBins(start, end time.Time, grain runtimev1.TimeGrain, alias string, tz *time.Location, firstDay, firstMonth int) (string, []any, error) {
	g := timeutil.TimeGrainFromAPI(grain)
	start = timeutil.TruncateTime(start, g, tz, firstDay, firstMonth)
	return fmt.Sprintf(
		"SELECT timezone('%[1]s', tbin)::TIMESTAMP AS %[2]s FROM generate_series(timezone('%[1]s', '%[3]s'::TIMESTAMPTZ), timezone('%[1]s', '%[4]s'::TIMESTAMPTZ), INTERVAL '%[5]s') AS tbin WHERE tbin < timezone('%[1]s', '%[4]s'::TIMESTAMPTZ)",
		tz.String(),
		d.EscapeAlias(alias),
		start.Format(time.RFC3339),
		end.Format(time.RFC3339),
		d.intervalForGrain(grain),
	), nil, nil
}
