package reconcilers_test

import (
	"testing"

	runtimev1 "github.com/rilldata/rill/proto/gen/rill/runtime/v1"
	"github.com/rilldata/rill/runtime"
	"github.com/rilldata/rill/runtime/testruntime"
	"github.com/stretchr/testify/require"
)

func TestMetricsViewTimeCaseInsensitive(t *testing.T) {
	rt, id := testruntime.NewInstanceWithOptions(t, testruntime.InstanceOptions{
		Files: map[string]string{
			"m1.sql": `SELECT '2024-01-01T00:00:00Z'::TIMESTAMP AS TiMe, 1 AS num`,
			"mv1.yaml": `
type: metrics_view
model: m1
timeseries: TiMe
measures:
- name: num
  expression: sum(num)
explore:
  skip: true
`,
			"mv2.yaml": `
type: metrics_view
model: m1
timeseries: TiMe
dimensions:
- column: TiMe
measures:
- name: num
  expression: sum(num)
explore:
  skip: true
`,
			"mv3.yaml": `
type: metrics_view
model: m1
timeseries: time
dimensions:
- name: time
  column: TiMe
measures:
- name: num
  expression: sum(num)
explore:
  skip: true
`,
			"mv4.yaml": `
type: metrics_view
model: m1
timeseries: time
dimensions:
- column: TiMe
measures:
- name: num
  expression: sum(num)
explore:
  skip: true
`,
			"mv5.yaml": `
type: metrics_view
model: m1
timeseries: TiMe
dimensions:
- name: time
  column: TiMe
measures:
- name: num
  expression: sum(num)
explore:
  skip: true
`,
		},
	})
	testruntime.RequireReconcileState(t, rt, id, 5, 1, 2)

	r := testruntime.GetResource(t, rt, id, runtime.ResourceKindMetricsView, "mv1")
	require.Empty(t, r.Meta.ReconcileError)
	d := r.GetMetricsView().State.ValidSpec.Dimensions[0]
	require.Equal(t, runtimev1.MetricsViewSpec_DIMENSION_TYPE_TIME, d.Type)
	require.Equal(t, runtimev1.Type_CODE_TIMESTAMP, d.DataType.Code)

	r = testruntime.GetResource(t, rt, id, runtime.ResourceKindMetricsView, "mv2")
	require.Empty(t, r.Meta.ReconcileError)
	d = r.GetMetricsView().State.ValidSpec.Dimensions[0]
	require.Equal(t, runtimev1.MetricsViewSpec_DIMENSION_TYPE_TIME, d.Type)
	require.Equal(t, runtimev1.Type_CODE_TIMESTAMP, d.DataType.Code)

	r = testruntime.GetResource(t, rt, id, runtime.ResourceKindMetricsView, "mv3")
	require.Empty(t, r.Meta.ReconcileError)
	d = r.GetMetricsView().State.ValidSpec.Dimensions[0]
	require.Equal(t, runtimev1.MetricsViewSpec_DIMENSION_TYPE_TIME, d.Type)
	require.Equal(t, runtimev1.Type_CODE_TIMESTAMP, d.DataType.Code)

	testruntime.RequireParseErrors(t, rt, id, map[string]string{
		"/mv4.yaml": "does not match the case of time dimension",
		"/mv5.yaml": "does not match the case of time dimension",
	})
}

func TestMetricsViewTimeTypes(t *testing.T) {
	rt, id := testruntime.NewInstanceWithOptions(t, testruntime.InstanceOptions{
		Files: map[string]string{
			"m1.sql": `SELECT '2024-01-01'::DATE AS date, '2024-01-01T00:00:00Z'::TIMESTAMP AS time, 'foo' AS str, 1 AS num`,
			"mv_none.yaml": `
type: metrics_view
model: m1
dimensions:
- column: time
- column: date
measures:
- name: num
  expression: sum(num)
explore:
  skip: true
`,
			"mv_time.yaml": `
type: metrics_view
model: m1
timeseries: time
dimensions:
- column: time
- column: date
measures:
- name: num
  expression: sum(num)
explore:
  skip: true
`,
			"mv_date.yaml": `
type: metrics_view
model: m1
timeseries: date
dimensions:
- column: time
- column: date
measures:
- name: num
  expression: sum(num)
explore:
  skip: true
`,
			"mv_time_legacy.yaml": `
type: metrics_view
model: m1
timeseries: time
dimensions:
- column: date
measures:
- name: num
  expression: sum(num)
explore:
  skip: true
`,
			"mv_date_legacy.yaml": `
type: metrics_view
model: m1
timeseries: date
dimensions:
- column: time
measures:
- name: num
  expression: sum(num)
explore:
  skip: true
`,
		},
	})
	testruntime.RequireReconcileState(t, rt, id, 7, 0, 0)

	// Expectations
	cases := []struct {
		metricsView string
		dimension   string
		typ         runtimev1.MetricsViewSpec_DimensionType
		dataTyp     runtimev1.Type_Code
	}{
		{"mv_none", "time", runtimev1.MetricsViewSpec_DIMENSION_TYPE_TIME, runtimev1.Type_CODE_TIMESTAMP},
		{"mv_none", "date", runtimev1.MetricsViewSpec_DIMENSION_TYPE_CATEGORICAL, runtimev1.Type_CODE_DATE},
		{"mv_time", "time", runtimev1.MetricsViewSpec_DIMENSION_TYPE_TIME, runtimev1.Type_CODE_TIMESTAMP},
		{"mv_time", "date", runtimev1.MetricsViewSpec_DIMENSION_TYPE_CATEGORICAL, runtimev1.Type_CODE_DATE},
		{"mv_date", "time", runtimev1.MetricsViewSpec_DIMENSION_TYPE_TIME, runtimev1.Type_CODE_TIMESTAMP},
		{"mv_date", "date", runtimev1.MetricsViewSpec_DIMENSION_TYPE_TIME, runtimev1.Type_CODE_DATE},
		{"mv_time_legacy", "time", runtimev1.MetricsViewSpec_DIMENSION_TYPE_TIME, runtimev1.Type_CODE_TIMESTAMP},
		{"mv_time_legacy", "date", runtimev1.MetricsViewSpec_DIMENSION_TYPE_CATEGORICAL, runtimev1.Type_CODE_DATE},
		{"mv_date_legacy", "time", runtimev1.MetricsViewSpec_DIMENSION_TYPE_TIME, runtimev1.Type_CODE_TIMESTAMP},
		{"mv_date_legacy", "date", runtimev1.MetricsViewSpec_DIMENSION_TYPE_TIME, runtimev1.Type_CODE_DATE},
	}
	for _, c := range cases {
		t.Run(c.metricsView+"_"+c.dimension, func(t *testing.T) {
			ctrl, err := rt.Controller(t.Context(), id)
			require.NoError(t, err)
			mv, err := ctrl.Get(t.Context(), &runtimev1.ResourceName{Kind: runtime.ResourceKindMetricsView, Name: c.metricsView}, false)
			require.NoError(t, err)
			validSpec := mv.GetMetricsView().State.ValidSpec
			require.NotNil(t, validSpec)

			var found bool
			for _, d := range validSpec.Dimensions {
				if d.Name == c.dimension {
					found = true
					require.Equal(t, c.typ, d.Type)
					require.Equal(t, c.dataTyp, d.DataType.Code)
				}
			}
			require.True(t, found, "dimension %s not found in metrics view %s", c.dimension, c.metricsView)
		})
	}
}

func TestMetricsViewSkipInvalidDimensions(t *testing.T) {
	rt, id := testruntime.NewInstanceWithOptions(t, testruntime.InstanceOptions{
		Files: map[string]string{
			"m1.sql": `SELECT '2024-01-01T00:00:00Z'::TIMESTAMP AS time, 'svc' AS service, 1 AS num`,
			// Without the flag, a dimension referencing a missing column fails the whole metrics view.
			"mv_strict.yaml": `
type: metrics_view
model: m1
timeseries: time
dimensions:
- column: service
- column: status_code
measures:
- name: num
  expression: sum(num)
explore:
  skip: true
`,
			// With the flag, invalid dimensions are excluded from the valid spec with a warning.
			"mv_skip.yaml": `
type: metrics_view
model: m1
timeseries: time
skip_invalid_dimensions: true
dimensions:
- column: service
- column: status_code
- column: log_level
measures:
- name: num
  expression: sum(num)
explore:
  skip: true
`,
		},
	})
	testruntime.RequireReconcileState(t, rt, id, 4, 1, 0)

	strict := testruntime.GetResource(t, rt, id, runtime.ResourceKindMetricsView, "mv_strict")
	require.Nil(t, strict.GetMetricsView().State.ValidSpec)
	require.Contains(t, strict.Meta.ReconcileError, "status_code")

	skip := testruntime.GetResource(t, rt, id, runtime.ResourceKindMetricsView, "mv_skip")
	require.Empty(t, skip.Meta.ReconcileError)
	validSpec := skip.GetMetricsView().State.ValidSpec
	require.NotNil(t, validSpec)
	var dims []string
	for _, d := range validSpec.Dimensions {
		dims = append(dims, d.Name)
	}
	require.Equal(t, []string{"time", "service"}, dims)
	// The full spec still contains all declared dimensions.
	require.Len(t, skip.GetMetricsView().Spec.Dimensions, 4)
	// The skipped dimensions are surfaced as reconcile warnings.
	require.Len(t, skip.Meta.ReconcileWarnings, 2)
	require.Contains(t, skip.Meta.ReconcileWarnings[0], "status_code")
	require.Contains(t, skip.Meta.ReconcileWarnings[1], "log_level")
}

func TestMetricsViewSkipEmptyDimensions(t *testing.T) {
	rt, id := testruntime.NewInstanceWithOptions(t, testruntime.InstanceOptions{
		Files: map[string]string{
			"m1.sql":      `SELECT '2024-01-01T00:00:00Z'::TIMESTAMP AS time, 'svc' AS service, CAST(NULL AS VARCHAR) AS status_code, 1 AS num`,
			"m_empty.sql": `SELECT '2024-01-01T00:00:00Z'::TIMESTAMP AS time, 'svc' AS service, 1 AS num WHERE 1=0`,
			// With the flag, dimensions whose values are all NULL are hidden from the valid spec with a warning.
			"mv_hide.yaml": `
type: metrics_view
model: m1
timeseries: time
skip_empty_dimensions: true
dimensions:
- column: service
- column: status_code
measures:
- name: num
  expression: sum(num)
explore:
  skip: true
`,
			// Without the flag, all-NULL dimensions are kept.
			"mv_keep.yaml": `
type: metrics_view
model: m1
timeseries: time
dimensions:
- column: service
- column: status_code
measures:
- name: num
  expression: sum(num)
explore:
  skip: true
`,
			// On a table with no rows, emptiness carries no signal, so nothing is hidden.
			"mv_empty.yaml": `
type: metrics_view
model: m_empty
timeseries: time
skip_empty_dimensions: true
dimensions:
- column: service
func TestMetricsViewTableOptions(t *testing.T) {
	rt, id := testruntime.NewInstanceWithOptions(t, testruntime.InstanceOptions{
		Files: map[string]string{
			// Two versions of a table: v1 predates the http_method and region columns.
			"events_v1.sql": `SELECT '2024-01-01T00:00:00Z'::TIMESTAMP AS "time", 'checkout' AS service, 1 AS num`,
			"events_v2.sql": `SELECT '2024-02-01T00:00:00Z'::TIMESTAMP AS "time", 'checkout' AS service, 'GET' AS http_method, 'eu' AS region, 2 AS num`,
			"mv.yaml": `
type: metrics_view
table: events_v2
table_options: [events_v1, events_v2]
timeseries: time
skip_invalid_dimensions: true
dimensions:
- column: service
- column: http_method
- column: region
measures:
- name: num
  expression: sum(num)
explore:
  skip: true
`,
		},
	})
	testruntime.RequireReconcileState(t, rt, id, 6, 0, 0)

	dimNames := func(spec *runtimev1.MetricsViewSpec) []string {
		var names []string
		for _, d := range spec.Dimensions {
			names = append(names, d.Name)
		}
		return names
	}

	hide := testruntime.GetResource(t, rt, id, runtime.ResourceKindMetricsView, "mv_hide")
	require.NotNil(t, hide.GetMetricsView().State.ValidSpec)
	require.Equal(t, []string{"time", "service"}, dimNames(hide.GetMetricsView().State.ValidSpec))
	// The full spec still contains all declared dimensions.
	require.Equal(t, []string{"time", "service", "status_code"}, dimNames(hide.GetMetricsView().Spec))
	// The hidden dimension is surfaced as a reconcile warning.
	require.Len(t, hide.Meta.ReconcileWarnings, 1)
	require.Contains(t, hide.Meta.ReconcileWarnings[0], "status_code")

	keep := testruntime.GetResource(t, rt, id, runtime.ResourceKindMetricsView, "mv_keep")
	require.Equal(t, []string{"time", "service", "status_code"}, dimNames(keep.GetMetricsView().State.ValidSpec))
	require.Empty(t, keep.Meta.ReconcileWarnings)

	empty := testruntime.GetResource(t, rt, id, runtime.ResourceKindMetricsView, "mv_empty")
	require.Equal(t, []string{"time", "service"}, dimNames(empty.GetMetricsView().State.ValidSpec))
	require.Empty(t, empty.Meta.ReconcileWarnings)

	// Once the column carries data, the dimension comes back automatically.
	testruntime.PutFiles(t, rt, id, map[string]string{
		"m1.sql": `SELECT '2024-01-01T00:00:00Z'::TIMESTAMP AS time, 'svc' AS service, '200' AS status_code, 1 AS num`,
	})
	testruntime.ReconcileParserAndWait(t, rt, id)
	testruntime.RequireReconcileState(t, rt, id, 6, 0, 0)
	hide = testruntime.GetResource(t, rt, id, runtime.ResourceKindMetricsView, "mv_hide")
	require.Equal(t, []string{"time", "service", "status_code"}, dimNames(hide.GetMetricsView().State.ValidSpec))
	require.Empty(t, hide.Meta.ReconcileWarnings)
}

func TestMetricsViewMapDimensions(t *testing.T) {
	rt, id := testruntime.NewInstanceWithOptions(t, testruntime.InstanceOptions{
		Files: map[string]string{
			// Structured-log-style data: fixed columns plus a map of dynamic attributes.
			// legacy_field is all-NULL, as when a field stops being ingested.
			"m1.sql": `
SELECT * FROM (VALUES
  ('2024-01-01T00:00:00Z'::TIMESTAMP, MAP {'http.method': 'GET', 'level': 'info'}, 'svc-a', NULL::VARCHAR, 1),
  ('2024-01-02T00:00:00Z'::TIMESTAMP, MAP {'http.method': 'POST', 'k8s.pod': 'p1'}, 'svc-b', NULL::VARCHAR, 2),
  ('2024-01-03T00:00:00Z'::TIMESTAMP, MAP {'http.method': 'GET'}, 'svc-a', NULL::VARCHAR, 3)
) t("time", attrs, service, legacy_field, num)`,
			"mv_map.yaml": `
type: metrics_view
model: m1
timeseries: time
skip_empty_dimensions: true
dimensions:
- column: service
- column: legacy_field
- map_column: attrs
measures:
- name: num
  expression: sum(num)
explore:
  skip: true
`,
			"mv_pattern.yaml": `
type: metrics_view
model: m1
timeseries: time
dimensions:
- map_column: attrs
  discover:
    pattern: '^http\.'
    limit: 10
measures:
- name: num
  expression: sum(num)
explore:
  skip: true
`,
		},
	})
	testruntime.RequireReconcileState(t, rt, id, 4, 0, 0)

	// mv_map: map keys expanded into dimensions, all-NULL legacy_field hidden.
	mvMap := testruntime.GetResource(t, rt, id, runtime.ResourceKindMetricsView, "mv_map")
	require.Empty(t, mvMap.Meta.ReconcileError)
	validSpec := mvMap.GetMetricsView().State.ValidSpec
	require.NotNil(t, validSpec)
	var names []string
	for _, d := range validSpec.Dimensions {
		names = append(names, d.Name)
	}
	// Discovered keys are ordered by frequency, then alphabetically.
	require.Equal(t, []string{"time", "service", "http_method", "k8s_pod", "level"}, names)
	require.Equal(t, "http.method", validSpec.Dimensions[2].DisplayName)
	require.Equal(t, `"attrs"['http.method']`, validSpec.Dimensions[2].Expression)
	// The declared spec still contains the raw map_column dimension.
	require.Len(t, mvMap.GetMetricsView().Spec.Dimensions, 4)
	require.Equal(t, "attrs", mvMap.GetMetricsView().Spec.Dimensions[3].MapColumn)
	// The all-NULL dimension is surfaced as a warning.
	require.Len(t, mvMap.Meta.ReconcileWarnings, 1)
	require.Contains(t, mvMap.Meta.ReconcileWarnings[0], "legacy_field")

	// mv_pattern: only keys matching the pattern are expanded.
	mvPattern := testruntime.GetResource(t, rt, id, runtime.ResourceKindMetricsView, "mv_pattern")
	require.Empty(t, mvPattern.Meta.ReconcileError)
	names = nil
	for _, d := range mvPattern.GetMetricsView().State.ValidSpec.Dimensions {
		names = append(names, d.Name)
	}
	require.Equal(t, []string{"time", "http_method"}, names)
}

func TestMetricsViewAllColumnsDimensions(t *testing.T) {
	rt, id := testruntime.NewInstanceWithOptions(t, testruntime.InstanceOptions{
		Files: map[string]string{
			// Schemaless-style data: the table schema is a union of all fields ever ingested
			// (as with e.g. read_json with union_by_name), so rows carry NULLs for fields they lack
			// and some columns (legacy) are all-NULL.
			"m1.sql": `
SELECT '2024-01-01T00:00:00Z'::TIMESTAMP AS "time", 'checkout' AS service, 'GET' AS http_method, 'alice' AS "user.name", NULL::VARCHAR AS legacy, 1 AS num
UNION ALL BY NAME
SELECT '2024-01-02T00:00:00Z'::TIMESTAMP AS "time", 'worker' AS service, 'debug' AS log_level, 'bob' AS "user.name", NULL::VARCHAR AS legacy, 2 AS num`,
			"mv_cols.yaml": `
type: metrics_view
model: m1
timeseries: time
skip_empty_dimensions: true
dimensions:
- columns: '*'
measures:
- name: num
  expression: sum(num)
explore:
  skip: true
`,
			"mv_cols_pattern.yaml": `
type: metrics_view
model: m1
timeseries: time
dimensions:
- columns: '*'
  discover:
    pattern: '^log_'
measures:
- name: num
  expression: sum(num)
explore:
  skip: true
`,
		},
	})
	testruntime.RequireReconcileState(t, rt, id, 4, 0, 0)

	// mv_cols: every groupable column becomes a dimension; measure columns and the time
	// dimension are not duplicated; the all-NULL column is hidden with a warning.
	mvCols := testruntime.GetResource(t, rt, id, runtime.ResourceKindMetricsView, "mv_cols")
	require.Empty(t, mvCols.Meta.ReconcileError)
	validSpec := mvCols.GetMetricsView().State.ValidSpec
	require.NotNil(t, validSpec)
	var names []string
	for _, d := range validSpec.Dimensions {
		names = append(names, d.Name)
	}
	require.Equal(t, []string{"time", "service", "http_method", "user_name", "log_level"}, names)
	require.Equal(t, "service", validSpec.Dimensions[1].Column)
	// Column names that are unsafe as dimension names (e.g. flattened nested fields from
	// schemaless ingestion) are sanitized, while the display name and column are preserved.
	require.Equal(t, "user.name", validSpec.Dimensions[3].DisplayName)
	require.Equal(t, "user.name", validSpec.Dimensions[3].Column)
	require.Len(t, mvCols.Meta.ReconcileWarnings, 1)
	require.Contains(t, mvCols.Meta.ReconcileWarnings[0], "legacy")

	// mv_cols_pattern: only columns matching the pattern are expanded.
	mvPattern := testruntime.GetResource(t, rt, id, runtime.ResourceKindMetricsView, "mv_cols_pattern")
	require.Empty(t, mvPattern.Meta.ReconcileError)
	names = nil
	for _, d := range mvPattern.GetMetricsView().State.ValidSpec.Dimensions {
		names = append(names, d.Name)
	}
	require.Equal(t, []string{"time", "log_level"}, names)
	testruntime.RequireReconcileState(t, rt, id, 5, 0, 0)

	// The primary metrics view is backed by the default table and carries the option mapping.
	primary := testruntime.GetResource(t, rt, id, runtime.ResourceKindMetricsView, "mv")
	require.Empty(t, primary.Meta.ReconcileError)
	primarySpec := primary.GetMetricsView().State.ValidSpec
	require.NotNil(t, primarySpec)
	require.Equal(t, "events_v2", primarySpec.Table)
	var names []string
	for _, d := range primarySpec.Dimensions {
		names = append(names, d.Name)
	}
	require.Equal(t, []string{"time", "service", "http_method", "region"}, names)
	require.Len(t, primarySpec.TableOptions, 2)
	require.Equal(t, "events_v2", primarySpec.TableOptions[0].Table)
	require.Equal(t, "mv", primarySpec.TableOptions[0].MetricsView)
	require.Equal(t, "events_v1", primarySpec.TableOptions[1].Table)
	require.Equal(t, "mv__events_v1", primarySpec.TableOptions[1].MetricsView)

	// The variant is backed by the old table; its missing columns are pruned by skip_invalid_dimensions.
	variant := testruntime.GetResource(t, rt, id, runtime.ResourceKindMetricsView, "mv__events_v1")
	require.Empty(t, variant.Meta.ReconcileError)
	variantSpec := variant.GetMetricsView().State.ValidSpec
	require.NotNil(t, variantSpec)
	require.Equal(t, "events_v1", variantSpec.Table)
	require.Empty(t, variantSpec.TableOptions)
	names = nil
	for _, d := range variantSpec.Dimensions {
		names = append(names, d.Name)
	}
	require.Equal(t, []string{"time", "service"}, names)
	require.Len(t, variant.Meta.ReconcileWarnings, 2)
}
