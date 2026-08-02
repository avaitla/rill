package executor

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	runtimev1 "github.com/rilldata/rill/proto/gen/rill/runtime/v1"
	"github.com/rilldata/rill/runtime/drivers"
	"google.golang.org/protobuf/proto"
)

// defaultMapDimensionDiscoverLimit is the number of map keys discovered for a map_column dimension when discover.limit is not set.
const defaultMapDimensionDiscoverLimit = 100

// ExpandDynamicDimensions returns a copy of the executor's metrics view spec where dimensions that declare
// a map_column or columns wildcard are replaced by concrete dimensions discovered in the data:
// one dimension per map key (most frequent keys first) or per table column respectively.
// It supports schemaless data sources where the schema is a union of all fields ever ingested,
// whether the variable fields land in a map-typed column or as individual columns.
// It returns a nil spec if the metrics view has no dynamic dimensions or the underlying table does not exist
// (the latter so validation can produce its standard error).
// If the metrics view has skip_invalid_dimensions enabled, a map dimension whose keys cannot be discovered
// (e.g. because the map column is missing from the current schema) is dropped with a warning instead of failing.
func (e *Executor) ExpandDynamicDimensions(ctx context.Context) (*runtimev1.MetricsViewSpec, []string, error) {
	mv := e.metricsView
	hasDynamic := false
	for _, d := range mv.Dimensions {
		if d.MapColumn != "" || d.AllColumns {
			hasDynamic = true
			break
		}
	}
	if !hasDynamic {
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
		if d.MapColumn == "" && !d.AllColumns {
			dims = append(dims, d)
			continue
		}

		if d.AllColumns {
			expandedDims, dimWarnings := expandTableColumns(t, d, taken)
			dims = append(dims, expandedDims...)
			warnings = append(warnings, dimWarnings...)
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

// nonGroupableTypeCodes are column types that are not expanded into dimensions by a columns wildcard,
// since grouping and filtering by them is not meaningful.
var nonGroupableTypeCodes = map[runtimev1.Type_Code]bool{
	runtimev1.Type_CODE_UNSPECIFIED: true,
	runtimev1.Type_CODE_ARRAY:       true,
	runtimev1.Type_CODE_MAP:         true,
	runtimev1.Type_CODE_STRUCT:      true,
	runtimev1.Type_CODE_JSON:        true,
	runtimev1.Type_CODE_BYTES:       true,
}

// expandTableColumns expands a columns wildcard dimension into one dimension per groupable column
// of the underlying table, in schema order.
// Column names are sanitized for use as dimension names, since schemaless ingestion may produce
// column names that are unsafe in identifiers elsewhere (e.g. flattened nested fields like "user.name");
// the display name and the referenced column keep the original name.
func expandTableColumns(t *drivers.OlapTable, d *runtimev1.MetricsViewSpec_Dimension, taken map[string]bool) ([]*runtimev1.MetricsViewSpec_Dimension, []string) {
	var pattern *regexp.Regexp
	if d.DiscoverPattern != "" {
		// The pattern is validated at parse time.
		pattern = regexp.MustCompile(d.DiscoverPattern)
	}

	limit := int(d.DiscoverLimit)
	if limit == 0 {
		limit = defaultMapDimensionDiscoverLimit
	}

	var dims []*runtimev1.MetricsViewSpec_Dimension
	var warnings []string
	for _, f := range t.Schema.Fields {
		if nonGroupableTypeCodes[f.Type.Code] {
			continue
		}
		if pattern != nil && !pattern.MatchString(f.Name) {
			continue
		}
		if taken[strings.ToLower(f.Name)] {
			// Explicitly declared dimensions and measures win; not a conflict worth warning about.
			continue
		}
		if len(dims) >= limit {
			warnings = append(warnings, fmt.Sprintf("dimension %q: hit the discover limit of %d columns; additional columns are not shown", d.Name, limit))
			break
		}
		name := safeFieldName(f.Name)
		if name == "" || taken[strings.ToLower(name)] {
			name = safeFieldName(fmt.Sprintf("%s_%s", d.Name, f.Name))
			if name == "" || taken[strings.ToLower(name)] {
				warnings = append(warnings, fmt.Sprintf("dimension %q: skipped column %q: conflicts with an existing dimension or measure name", d.Name, f.Name))
				continue
			}
		}
		taken[strings.ToLower(f.Name)] = true
		taken[strings.ToLower(name)] = true
		dims = append(dims, &runtimev1.MetricsViewSpec_Dimension{
			Name:        name,
			DisplayName: f.Name,
			Description: d.Description,
			Column:      f.Name,
			Tags:        d.Tags,
		})
	}
	return dims, warnings
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
