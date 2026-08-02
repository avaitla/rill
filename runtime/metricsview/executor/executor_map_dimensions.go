package executor

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode"

	runtimev1 "github.com/rilldata/rill/proto/gen/rill/runtime/v1"
	"github.com/rilldata/rill/runtime/drivers"
	"google.golang.org/protobuf/proto"
)

// defaultMapDimensionDiscoverLimit is the number of map keys discovered for a map_column dimension when discover.limit is not set.
const defaultMapDimensionDiscoverLimit = 100

// ExpandMapDimensions returns a copy of the executor's metrics view spec where each dimension that declares a map_column
// is replaced by concrete dimensions for the keys discovered in the data, most frequent keys first.
// It supports schemaless data sources where variable fields are ingested into a single map-typed column.
// It returns a nil spec if the metrics view has no map dimensions or the underlying table does not exist
// (the latter so validation can produce its standard error).
// If the metrics view has skip_invalid_dimensions enabled, a map dimension whose keys cannot be discovered
// (e.g. because the map column is missing from the current schema) is dropped with a warning instead of failing.
func (e *Executor) ExpandMapDimensions(ctx context.Context) (*runtimev1.MetricsViewSpec, []string, error) {
	mv := e.metricsView
	hasMap := false
	for _, d := range mv.Dimensions {
		if d.MapColumn != "" {
			hasMap = true
			break
		}
	}
	if !hasMap {
		return nil, nil, nil
	}

	t, err := e.olap.InformationSchema().Lookup(ctx, mv.Database, mv.DatabaseSchema, mv.Table)
	if err != nil {
		if errors.Is(err, drivers.ErrNotFound) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("failed to look up table %q: %w", mv.Table, err)
	}

	// Names already taken by declared dimensions and measures; discovered keys must not collide with them.
	taken := make(map[string]bool, len(mv.Dimensions)+len(mv.Measures))
	for _, d := range mv.Dimensions {
		taken[strings.ToLower(d.Name)] = true
	}
	for _, m := range mv.Measures {
		taken[strings.ToLower(m.Name)] = true
	}

	expanded := proto.Clone(mv).(*runtimev1.MetricsViewSpec)
	dims := make([]*runtimev1.MetricsViewSpec_Dimension, 0, len(expanded.Dimensions))
	var warnings []string
	for _, d := range expanded.Dimensions {
		if d.MapColumn == "" {
			dims = append(dims, d)
			continue
		}

		limit := int(d.DiscoverLimit)
		if limit == 0 {
			limit = defaultMapDimensionDiscoverLimit
		}
		keys, err := e.discoverMapKeys(ctx, t, d.MapColumn, d.DiscoverPattern, limit)
		if err != nil {
			if ctx.Err() != nil {
				return nil, nil, ctx.Err()
			}
			if mv.SkipInvalidDimensions {
				warnings = append(warnings, fmt.Sprintf("skipped dimension %q: failed to discover keys of map column %q: %s", d.Name, d.MapColumn, err.Error()))
				continue
			}
			return nil, nil, fmt.Errorf("failed to discover keys of map column %q for dimension %q: %w", d.MapColumn, d.Name, err)
		}
		if len(keys) >= limit {
			warnings = append(warnings, fmt.Sprintf("dimension %q: hit the discover limit of %d keys for map column %q; additional keys are not shown", d.Name, limit, d.MapColumn))
		}

		for _, key := range keys {
			name := safeFieldName(key)
			if name == "" || taken[strings.ToLower(name)] {
				// Fall back to a name prefixed with the map dimension's name to resolve collisions.
				name = safeFieldName(fmt.Sprintf("%s_%s", d.Name, key))
				if name == "" || taken[strings.ToLower(name)] {
					warnings = append(warnings, fmt.Sprintf("dimension %q: skipped key %q of map column %q: conflicts with an existing dimension or measure name", d.Name, key, d.MapColumn))
					continue
				}
			}
			taken[strings.ToLower(name)] = true
			dims = append(dims, &runtimev1.MetricsViewSpec_Dimension{
				Name:        name,
				DisplayName: key,
				Description: d.Description,
				Expression:  e.olap.Dialect().MapAccessExpr(d.MapColumn, key),
				Tags:        d.Tags,
			})
		}
	}
	expanded.Dimensions = dims
	return expanded, warnings, nil
}

// discoverMapKeys returns the distinct keys of a map-typed column, most frequent first.
func (e *Executor) discoverMapKeys(ctx context.Context, t *drivers.OlapTable, column, pattern string, limit int) ([]string, error) {
	query, err := e.olap.Dialect().SelectMapKeys(t.Database, t.DatabaseSchema, t.Name, column, pattern, limit)
	if err != nil {
		return nil, err
	}
	rows, err := e.olap.Query(ctx, &drivers.Statement{
		Query:            query,
		Priority:         e.priority,
		ExecutionTimeout: defaultExecutionTimeout,
		QueryAttributes:  e.queryAttributes,
	})
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

// safeFieldName converts a discovered map key to a name that is safe to use as a dimension name,
// replacing any character that is not a letter, digit or underscore.
// It returns an empty string if the key contains no usable characters.
func safeFieldName(key string) string {
	var b strings.Builder
	for _, r := range key {
		if r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	if strings.Trim(b.String(), "_") == "" {
		return ""
	}
	return b.String()
}
