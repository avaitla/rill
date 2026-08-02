package executor

import (
	"context"
	"fmt"
	"strings"

	"github.com/rilldata/rill/runtime/drivers"
)

// EmptyDimensions returns the indices of dimensions whose values are NULL in every row of the underlying table.
// It supports schemaless data sources where the table schema is a union of all fields ever ingested,
// so a field not present in the current data shows up as an all-NULL column.
// It returns nil if the table has no rows, since in that case emptiness carries no signal.
// The time dimension and dimensions whose values cannot be resolved with a plain expression are never reported as empty.
func (e *Executor) EmptyDimensions(ctx context.Context) ([]int, error) {
	mv := e.metricsView
	dialect := e.olap.Dialect()

	exprs := []string{"count(*)"}
	var idxs []int
	for i, d := range mv.Dimensions {
		// Never hide the time dimension; dashboards require it.
		if strings.EqualFold(d.Name, mv.TimeDimension) {
			continue
		}
		expr, err := dialect.MetricsViewDimensionExpression(d)
		if err != nil {
			continue
		}
		// For unnest dimensions the expression is the underlying array value,
		// so the dimension is only considered empty when the arrays themselves are all NULL.
		exprs = append(exprs, fmt.Sprintf("count(%s)", expr))
		idxs = append(idxs, i)
	}
	if len(idxs) == 0 {
		return nil, nil
	}

	t, err := e.olap.InformationSchema().Lookup(ctx, mv.Database, mv.DatabaseSchema, mv.Table)
	if err != nil {
		return nil, fmt.Errorf("failed to look up table %q: %w", mv.Table, err)
	}

	rows, err := e.olap.Query(ctx, &drivers.Statement{
		Query:            fmt.Sprintf("SELECT %s FROM %s", strings.Join(exprs, ", "), dialect.EscapeTable(t.Database, t.DatabaseSchema, t.Name)),
		Priority:         e.priority,
		ExecutionTimeout: defaultExecutionTimeout,
		QueryAttributes:  e.queryAttributes,
	})
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, rows.Err()
	}
	counts := make([]int64, len(exprs))
	dests := make([]any, len(exprs))
	for i := range counts {
		dests[i] = &counts[i]
	}
	if err := rows.Scan(dests...); err != nil {
		return nil, err
	}

	if counts[0] == 0 {
		// The table has no rows; leave all dimensions visible.
		return nil, nil
	}
	var empty []int
	for i, idx := range idxs {
		if counts[i+1] == 0 {
			empty = append(empty, idx)
		}
	}
	return empty, nil
}
